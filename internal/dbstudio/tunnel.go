package dbstudio

import (
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"groxy-desk/internal/security"

	"golang.org/x/crypto/ssh"
)

const (
	MaxConcurrentTunnels = 8
	FirstTunnelPort      = 15432
	LastTunnelPort       = 15439
	defaultTunnelTTL     = 15 * time.Minute
)

// OpenTunnelRequest configures a loopback-bound (127.0.0.1:15432-15439) SSH DB tunnel.
type OpenTunnelRequest struct {
	TunnelID             string `json:"tunnel_id,omitempty"`
	Engine               string `json:"engine"` // postgres | mariadb
	SSHHost              string `json:"ssh_host"`
	SSHPort              string `json:"ssh_port"`
	SSHUser              string `json:"ssh_user"`
	KnownHostsPath       string `json:"known_hosts_path,omitempty"`
	PinnedFingerprint    string `json:"pinned_fingerprint"`
	PresentedFingerprint string `json:"presented_fingerprint,omitempty"`
	SSHPrivateKeyPath    string `json:"ssh_private_key_path,omitempty"`
	SSHPassword          string `json:"ssh_password,omitempty"`
	RemoteDBHost         string `json:"remote_db_host,omitempty"` // default 127.0.0.1 inside remote VPS
	RemoteDBPort         string `json:"remote_db_port,omitempty"` // e.g. 5432 or 3306
	LocalBindHost        string `json:"local_bind_host,omitempty"` // must be empty or 127.0.0.1 (DESK-E501 otherwise)
	IdleTTLSeconds       string `json:"idle_ttl_seconds,omitempty"`
	Transport            string `json:"transport,omitempty"` // "ssh" (default when remote) | "loopback_test"
}

// TunnelView exposes safe non-secret metadata for UI and API responses.
type TunnelView struct {
	TunnelID          string `json:"tunnel_id"`
	Engine            string `json:"engine"`
	LocalBindAddr     string `json:"local_bind_addr"`
	LocalPort         string `json:"local_port"`
	RemoteTarget      string `json:"remote_target"`
	SSHHost           string `json:"ssh_host"`
	PinnedFingerprint string `json:"pinned_fingerprint"`
	Transport         string `json:"transport"`
	TTLSeconds        string `json:"ttl_seconds"`
	Status            string `json:"status"`
	CreatedAt         string `json:"created_at"`
	LastActivityAt    string `json:"last_activity_at"`
}

type managedTunnel struct {
	view       TunnelView
	listener   net.Listener
	sshClient  *ssh.Client
	remoteAddr string
	idleTTL    time.Duration
	lastActive time.Time
	closedCh   chan struct{}
	closeOnce  sync.Once
}

// TunnelManager manages up to 8 concurrent 127.0.0.1 SSH database port-forward tunnels.
type TunnelManager struct {
	redactor *security.SecretRedactor
	mu       sync.Mutex
	tunnels  map[string]*managedTunnel
	usedPort map[int]string
}

// NewTunnelManager constructs a TunnelManager enforcing DESK-E501, DESK-E405, and 8-tunnel limits.
func NewTunnelManager(redactor *security.SecretRedactor) *TunnelManager {
	if redactor == nil {
		redactor = security.NewSecretRedactor()
	}
	return &TunnelManager{
		redactor: redactor,
		tunnels:  make(map[string]*managedTunnel),
		usedPort: make(map[int]string),
	}
}

// OpenTunnel binds a new loopback port in 127.0.0.1:15432-15439 after pinned SSH verification.
func (tm *TunnelManager) OpenTunnel(req OpenTunnelRequest) (TunnelView, error) {
	if bindHost := strings.TrimSpace(req.LocalBindHost); bindHost != "" {
		if _, err := security.ValidateLoopbackBind(net.JoinHostPort(bindHost, "15432")); err != nil {
			return TunnelView{}, err
		}
	}

	engine := normalizeEngine(req.Engine)
	if engine != "postgres" && engine != "mariadb" {
		return TunnelView{}, security.NewError(security.CodeDeskE101, 400, "tunnel engine must be postgres or mariadb", nil)
	}

	pin, err := security.NormalizeSSHFingerprint(req.PinnedFingerprint)
	if err != nil {
		return TunnelView{}, err
	}
	if strings.TrimSpace(req.PresentedFingerprint) != "" {
		if err := security.VerifySSHFingerprintString(pin, req.PresentedFingerprint); err != nil {
			return TunnelView{}, err
		}
	}
	if strings.TrimSpace(req.KnownHostsPath) != "" {
		keys, khErr := security.ReadKnownHostKeys(req.KnownHostsPath)
		if khErr != nil {
			return TunnelView{}, khErr
		}
		matched := false
		for _, k := range keys {
			if security.VerifySSHHostKey(pin, k) == nil {
				matched = true
				break
			}
		}
		if !matched {
			return TunnelView{}, security.NewError(security.CodeDeskE405, 403, "tunnel pinned fingerprint absent from known_hosts (DESK-E405)", nil)
		}
	}

	remoteHost := strings.TrimSpace(req.RemoteDBHost)
	if remoteHost == "" {
		remoteHost = "127.0.0.1"
	}
	remotePort := strings.TrimSpace(req.RemoteDBPort)
	if remotePort == "" {
		if engine == "postgres" {
			remotePort = "5432"
		} else {
			remotePort = "3306"
		}
	}
	if !security.IsExactIntString(remotePort) {
		return TunnelView{}, security.NewError(security.CodeDeskE101, 400, "remote_db_port must be an exact integer string", nil)
	}
	remoteTarget := net.JoinHostPort(remoteHost, remotePort)

	ttl := defaultTunnelTTL
	ttlStr := "900"
	if rawTTL := strings.TrimSpace(req.IdleTTLSeconds); rawTTL != "" {
		if !security.IsExactIntString(rawTTL) {
			return TunnelView{}, security.NewError(security.CodeDeskE101, 400, "idle_ttl_seconds must be an exact base-10 integer string", nil)
		}
		secs, convErr := strconv.Atoi(rawTTL)
		if convErr != nil || secs <= 0 || secs > 86400 {
			return TunnelView{}, security.NewError(security.CodeDeskE101, 400, "idle_ttl_seconds must be between 1 and 86400", nil)
		}
		ttl = time.Duration(secs) * time.Second
		ttlStr = rawTTL
	}

	transport := strings.ToLower(strings.TrimSpace(req.Transport))
	if transport == "" {
		transport = "ssh"
	}

	var sshClient *ssh.Client
	if transport == "loopback_test" {
		if _, err := security.ValidateLoopbackBind(remoteTarget); err != nil {
			return TunnelView{}, security.NewError(security.CodeDeskE501, 422, "loopback_test transport only permits 127.0.0.1 targets (DESK-E501)", err)
		}
	} else {
		sshPort := strings.TrimSpace(req.SSHPort)
		if sshPort == "" {
			sshPort = "22"
		}
		if strings.TrimSpace(req.SSHHost) == "" {
			return TunnelView{}, security.NewError(security.CodeDeskE101, 400, "ssh_host is required for SSH tunnel", nil)
		}
		client, dialErr := tm.dialPinnedSSH(req.SSHHost, sshPort, req.SSHUser, pin, req.KnownHostsPath, req.SSHPassword)
		if dialErr != nil {
			return TunnelView{}, dialErr
		}
		sshClient = client
	}

	tm.mu.Lock()
	defer tm.mu.Unlock()

	tm.reapExpiredLocked()
	if len(tm.tunnels) >= MaxConcurrentTunnels {
		if sshClient != nil {
			_ = sshClient.Close()
		}
		return TunnelView{}, security.NewError(security.CodeDeskE409, 409, fmt.Sprintf("maximum of %d concurrent SSH database tunnels reached (DESK-E409)", MaxConcurrentTunnels), nil)
	}

	tunnelID := strings.TrimSpace(req.TunnelID)
	if tunnelID == "" {
		rnd, _ := security.RandomTokenHex(6)
		tunnelID = "tun-" + rnd
	}
	if _, exists := tm.tunnels[tunnelID]; exists {
		if sshClient != nil {
			_ = sshClient.Close()
		}
		return TunnelView{}, security.NewError(security.CodeDeskE409, 409, fmt.Sprintf("tunnel_id %q already active", tunnelID), nil)
	}

	var ln net.Listener
	var chosenPort int
	for p := FirstTunnelPort; p <= LastTunnelPort; p++ {
		if _, taken := tm.usedPort[p]; taken {
			continue
		}
		addr := net.JoinHostPort("127.0.0.1", strconv.Itoa(p))
		cand, lErr := net.Listen("tcp", addr)
		if lErr == nil {
			ln = cand
			chosenPort = p
			break
		}
	}
	if ln == nil {
		if sshClient != nil {
			_ = sshClient.Close()
		}
		return TunnelView{}, security.NewError(security.CodeDeskE409, 409, "no free ports remaining in 127.0.0.1:15432-15439 (DESK-E409)", nil)
	}

	now := time.Now().UTC()
	view := TunnelView{
		TunnelID:          tunnelID,
		Engine:            engine,
		LocalBindAddr:     ln.Addr().String(),
		LocalPort:         strconv.Itoa(chosenPort),
		RemoteTarget:      remoteTarget,
		SSHHost:           strings.TrimSpace(req.SSHHost),
		PinnedFingerprint: pin,
		Transport:         transport,
		TTLSeconds:        ttlStr,
		Status:            "open",
		CreatedAt:         now.Format(time.RFC3339),
		LastActivityAt:    now.Format(time.RFC3339),
	}

	mt := &managedTunnel{
		view:       view,
		listener:   ln,
		sshClient:  sshClient,
		remoteAddr: remoteTarget,
		idleTTL:    ttl,
		lastActive: now,
		closedCh:   make(chan struct{}),
	}
	tm.tunnels[tunnelID] = mt
	tm.usedPort[chosenPort] = tunnelID

	go tm.serveTunnel(mt)
	return view, nil
}

func (tm *TunnelManager) dialPinnedSSH(host, port, user, expectedPin, knownHostsPath, password string) (*ssh.Client, error) {
	cb, err := security.NewPinnedHostKeyCallback(expectedPin, knownHostsPath)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(password) != "" {
		tm.redactor.Register(password)
	}
	var auths []ssh.AuthMethod
	if strings.TrimSpace(password) != "" {
		auths = append(auths, ssh.Password(password))
	}
	cfg := &ssh.ClientConfig{
		User:            firstNonEmpty(user, "grab"),
		Auth:            auths,
		HostKeyCallback: cb,
		Timeout:         6 * time.Second,
	}
	client, err := ssh.Dial("tcp", net.JoinHostPort(host, port), cfg)
	if err != nil {
		var de *security.DeskError
		if errors.As(err, &de) {
			return nil, de
		}
		if strings.Contains(err.Error(), security.CodeDeskE405) {
			return nil, security.NewError(security.CodeDeskE405, 403, err.Error(), err)
		}
		return nil, security.NewError(security.CodeDeskE502, 502, fmt.Sprintf("pinned SSH dial to %s:%s failed", host, port), err)
	}
	return client, nil
}

func (tm *TunnelManager) serveTunnel(mt *managedTunnel) {
	for {
		conn, err := mt.listener.Accept()
		if err != nil {
			select {
			case <-mt.closedCh:
				return
			default:
				return
			}
		}
		tm.touchTunnel(mt.view.TunnelID)
		go tm.forwardConn(mt, conn)
	}
}

func (tm *TunnelManager) forwardConn(mt *managedTunnel, localConn net.Conn) {
	defer localConn.Close()
	var remoteConn net.Conn
	var err error
	if mt.sshClient != nil {
		remoteConn, err = mt.sshClient.Dial("tcp", mt.remoteAddr)
	} else {
		remoteConn, err = net.DialTimeout("tcp", mt.remoteAddr, 5*time.Second)
	}
	if err != nil {
		return
	}
	defer remoteConn.Close()

	done := make(chan struct{}, 2)
	cp := func(dst io.Writer, src io.Reader) {
		_, _ = io.Copy(dst, src)
		done <- struct{}{}
	}
	go cp(remoteConn, localConn)
	go cp(localConn, remoteConn)

	select {
	case <-done:
	case <-mt.closedCh:
	}
}

// ListTunnels returns all active loopback DB tunnels after reaping idle-expired ones.
func (tm *TunnelManager) ListTunnels() []TunnelView {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	tm.reapExpiredLocked()
	out := make([]TunnelView, 0, len(tm.tunnels))
	for _, mt := range tm.tunnels {
		out = append(out, mt.view)
	}
	return out
}

// GetTunnel retrieves an active tunnel by ID and refreshes its activity timestamp.
func (tm *TunnelManager) GetTunnel(tunnelID string) (TunnelView, bool) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	tm.reapExpiredLocked()
	mt, ok := tm.tunnels[strings.TrimSpace(tunnelID)]
	if !ok {
		return TunnelView{}, false
	}
	now := time.Now().UTC()
	mt.lastActive = now
	mt.view.LastActivityAt = now.Format(time.RFC3339)
	return mt.view, true
}

// CloseTunnel tears down a single SSH database tunnel cleanly.
func (tm *TunnelManager) CloseTunnel(tunnelID string) bool {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	mt, ok := tm.tunnels[strings.TrimSpace(tunnelID)]
	if !ok {
		return false
	}
	tm.closeTunnelLocked(mt)
	return true
}

// CloseAll tears down all active SSH tunnels during server Shutdown.
func (tm *TunnelManager) CloseAll() {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	for _, mt := range tm.tunnels {
		tm.closeTunnelLocked(mt)
	}
}

func (tm *TunnelManager) touchTunnel(tunnelID string) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	if mt, ok := tm.tunnels[tunnelID]; ok {
		now := time.Now().UTC()
		mt.lastActive = now
		mt.view.LastActivityAt = now.Format(time.RFC3339)
	}
}

func (tm *TunnelManager) reapExpiredLocked() {
	now := time.Now().UTC()
	for _, mt := range tm.tunnels {
		if now.Sub(mt.lastActive) > mt.idleTTL {
			tm.closeTunnelLocked(mt)
		}
	}
}

func (tm *TunnelManager) closeTunnelLocked(mt *managedTunnel) {
	mt.closeOnce.Do(func() {
		close(mt.closedCh)
		if mt.listener != nil {
			_ = mt.listener.Close()
		}
		if mt.sshClient != nil {
			_ = mt.sshClient.Close()
		}
		if portNum, err := strconv.Atoi(mt.view.LocalPort); err == nil {
			delete(tm.usedPort, portNum)
		}
		delete(tm.tunnels, mt.view.TunnelID)
	})
}

func normalizeEngine(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "sqlite", "sqlite3":
		return "sqlite"
	case "postgres", "postgresql":
		return "postgres"
	case "mariadb", "mysql":
		return "mariadb"
	default:
		return strings.ToLower(strings.TrimSpace(raw))
	}
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
