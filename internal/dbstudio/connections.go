package dbstudio

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"groxy-desk/internal/security"
)

const connectionsFileName = "db-connections.json"

var reSafeConnID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,63}$`)

// SavedConnection persists non-secret metadata and AES-256-GCM (v1:...) encrypted DB/SSH credentials in stateDir.
type SavedConnection struct {
	ID                string `json:"id"`
	Name              string `json:"name"`
	Engine            string `json:"engine"`
	SQLitePath        string `json:"sqlite_path,omitempty"`
	DBHost            string `json:"db_host,omitempty"`
	DBPort            string `json:"db_port,omitempty"`
	DBUser            string `json:"db_user,omitempty"`
	DBName            string `json:"db_name,omitempty"`
	SchemaName        string `json:"schema_name,omitempty"`
	DBPasswordEnc     string `json:"db_password_enc,omitempty"`
	BridgeBearerEnc   string `json:"bridge_bearer_enc,omitempty"`
	SSHEnabled        bool   `json:"ssh_enabled,omitempty"`
	SSHHost           string `json:"ssh_host,omitempty"`
	SSHPort           string `json:"ssh_port,omitempty"`
	SSHUser           string `json:"ssh_user,omitempty"`
	SSHPrivateKeyPath string `json:"ssh_private_key_path,omitempty"`
	KnownHostsPath    string `json:"known_hosts_path,omitempty"`
	PinnedFingerprint string `json:"pinned_fingerprint,omitempty"`
	RemoteDBPort      string `json:"remote_db_port,omitempty"`
	SSHPasswordEnc    string `json:"ssh_password_enc,omitempty"`
	CreatedAt         string `json:"created_at"`
	UpdatedAt         string `json:"updated_at"`
}

// ConnectionUpsertRequest saves or updates a Database Studio connection profile.
type ConnectionUpsertRequest struct {
	ID                string `json:"id,omitempty"`
	Name              string `json:"name"`
	Engine            string `json:"engine"`
	SQLitePath        string `json:"sqlite_path,omitempty"`
	DBHost            string `json:"db_host,omitempty"`
	DBPort            string `json:"db_port,omitempty"`
	DBUser            string `json:"db_user,omitempty"`
	DBPassword        string `json:"db_password,omitempty"`
	ClearDBPassword   bool   `json:"clear_db_password,omitempty"`
	DBName            string `json:"db_name,omitempty"`
	SchemaName        string `json:"schema_name,omitempty"`
	BridgeBearer      string `json:"bridge_bearer,omitempty"`
	ClearBridgeBearer bool   `json:"clear_bridge_bearer,omitempty"`
	SSHEnabled        bool   `json:"ssh_enabled,omitempty"`
	SSHHost           string `json:"ssh_host,omitempty"`
	SSHPort           string `json:"ssh_port,omitempty"`
	SSHUser           string `json:"ssh_user,omitempty"`
	SSHPrivateKeyPath string `json:"ssh_private_key_path,omitempty"`
	KnownHostsPath    string `json:"known_hosts_path,omitempty"`
	PinnedFingerprint string `json:"pinned_fingerprint,omitempty"`
	RemoteDBPort      string `json:"remote_db_port,omitempty"`
	SSHPassword       string `json:"ssh_password,omitempty"`
	ClearSSHPassword  bool   `json:"clear_ssh_password,omitempty"`
}

// ConnectionView exposes safe metadata without ever returning plaintext passwords or bearers.
type ConnectionView struct {
	ID                string `json:"id"`
	Name              string `json:"name"`
	Engine            string `json:"engine"`
	SQLitePath        string `json:"sqlite_path,omitempty"`
	DBHost            string `json:"db_host,omitempty"`
	DBPort            string `json:"db_port,omitempty"`
	DBUser            string `json:"db_user,omitempty"`
	DBName            string `json:"db_name,omitempty"`
	SchemaName        string `json:"schema_name,omitempty"`
	HasDBPassword     bool   `json:"has_db_password"`
	DBPasswordMasked  string `json:"db_password_masked,omitempty"`
	HasBridgeBearer   bool   `json:"has_bridge_bearer"`
	SSHEnabled        bool   `json:"ssh_enabled"`
	SSHHost           string `json:"ssh_host,omitempty"`
	SSHPort           string `json:"ssh_port,omitempty"`
	SSHUser           string `json:"ssh_user,omitempty"`
	SSHPrivateKeyPath string `json:"ssh_private_key_path,omitempty"`
	KnownHostsPath    string `json:"known_hosts_path,omitempty"`
	PinnedFingerprint string `json:"pinned_fingerprint,omitempty"`
	RemoteDBPort      string `json:"remote_db_port,omitempty"`
	HasSSHPassword    bool   `json:"has_ssh_password"`
	CreatedAt         string `json:"created_at"`
	UpdatedAt         string `json:"updated_at"`
}

// ConnectionStore manages encrypted database connection profiles in stateDir.
type ConnectionStore struct {
	stateDir string
	redactor *security.SecretRedactor
	mu       sync.RWMutex
	items    map[string]SavedConnection
}

// NewConnectionStore initializes an encrypted connection profile store inside stateDir.
func NewConnectionStore(stateDir string, redactor *security.SecretRedactor) (*ConnectionStore, error) {
	cleanState := strings.TrimSpace(stateDir)
	if cleanState == "" {
		return nil, security.NewError(security.CodeDeskE101, 500, "connection store state directory is required", nil)
	}
	if err := os.MkdirAll(cleanState, 0o700); err != nil {
		return nil, security.NewError(security.CodeDeskE101, 500, "failed creating connection store directory", err)
	}
	if redactor == nil {
		redactor = security.NewSecretRedactor()
	}
	cs := &ConnectionStore{
		stateDir: cleanState,
		redactor: redactor,
		items:    make(map[string]SavedConnection),
	}
	if err := cs.load(); err != nil {
		return nil, err
	}
	return cs, nil
}

// List returns all saved connection profiles without secrets, sorted by name then ID.
func (cs *ConnectionStore) List() []ConnectionView {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	out := make([]ConnectionView, 0, len(cs.items))
	for _, item := range cs.items {
		out = append(out, toConnectionView(item))
	}
	sort.Slice(out, func(i, j int) bool {
		ni := strings.ToLower(out[i].Name)
		nj := strings.ToLower(out[j].Name)
		if ni != nj {
			return ni < nj
		}
		return out[i].ID < out[j].ID
	})
	return out
}

// Save creates or updates an encrypted connection profile in stateDir/db-connections.json.
func (cs *ConnectionStore) Save(req ConnectionUpsertRequest) (ConnectionView, error) {
	engine := normalizeEngine(req.Engine)
	switch engine {
	case "sqlite", "sqlite-bridge", "postgres", "mariadb":
	default:
		return ConnectionView{}, security.NewError(security.CodeDeskE101, 400, "engine must be sqlite, sqlite-bridge, postgres, or mariadb", nil)
	}

	id := strings.TrimSpace(req.ID)
	if id == "" {
		var rnd [6]byte
		if _, err := rand.Read(rnd[:]); err != nil {
			return ConnectionView{}, security.NewError(security.CodeDeskE101, 500, "failed generating connection ID", err)
		}
		id = "conn-" + hex.EncodeToString(rnd[:])
	}
	if !reSafeConnID.MatchString(id) {
		return ConnectionView{}, security.NewError(security.CodeDeskE101, 400, "connection id contains invalid characters", nil)
	}

	for _, portField := range []string{req.DBPort, req.SSHPort, req.RemoteDBPort} {
		trimmed := strings.TrimSpace(portField)
		if trimmed != "" && !security.IsExactIntString(trimmed) {
			return ConnectionView{}, security.NewError(security.CodeDeskE101, 400, "port values must be exact base-10 integers", nil)
		}
	}

	pin := strings.TrimSpace(req.PinnedFingerprint)
	if pin != "" {
		normPin, err := security.NormalizeSSHFingerprint(pin)
		if err != nil {
			return ConnectionView{}, err
		}
		pin = normPin
	}

	key, err := security.ResolveMasterKey(cs.stateDir)
	if err != nil {
		return ConnectionView{}, err
	}

	cs.mu.Lock()
	defer cs.mu.Unlock()

	existing, exists := cs.items[id]
	now := time.Now().UTC().Format(time.RFC3339)
	createdAt := now
	if exists && existing.CreatedAt != "" {
		createdAt = existing.CreatedAt
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		if engine == "sqlite" {
			name = firstNonEmpty(strings.TrimSpace(req.SQLitePath), "Workspace SQLite")
		} else {
			name = firstNonEmpty(strings.TrimSpace(req.DBName), strings.TrimSpace(req.SSHHost), engine)
		}
	}

	sshHost := strings.TrimSpace(req.SSHHost)
	sshEnabled := req.SSHEnabled || engine == "postgres" || engine == "mariadb" || engine == "sqlite-bridge" || sshHost != "" || pin != ""

	rec := SavedConnection{
		ID:                id,
		Name:              name,
		Engine:            engine,
		SQLitePath:        strings.TrimSpace(req.SQLitePath),
		DBHost:            firstNonEmpty(strings.TrimSpace(req.DBHost), "127.0.0.1"),
		DBPort:            strings.TrimSpace(req.DBPort),
		DBUser:            strings.TrimSpace(req.DBUser),
		DBName:            strings.TrimSpace(req.DBName),
		SchemaName:        strings.TrimSpace(req.SchemaName),
		SSHEnabled:        sshEnabled,
		SSHHost:           sshHost,
		SSHPort:           firstNonEmpty(strings.TrimSpace(req.SSHPort), "22"),
		SSHUser:           strings.TrimSpace(req.SSHUser),
		SSHPrivateKeyPath: strings.TrimSpace(req.SSHPrivateKeyPath),
		KnownHostsPath:    strings.TrimSpace(req.KnownHostsPath),
		PinnedFingerprint: pin,
		RemoteDBPort:      strings.TrimSpace(req.RemoteDBPort),
		CreatedAt:         createdAt,
		UpdatedAt:         now,
	}

	if req.DBPassword != "" {
		cs.redactor.Register(req.DBPassword)
		enc, encErr := security.EncryptAEAD(key, req.DBPassword)
		if encErr != nil {
			return ConnectionView{}, encErr
		}
		rec.DBPasswordEnc = enc
	} else if exists && !req.ClearDBPassword {
		rec.DBPasswordEnc = existing.DBPasswordEnc
	}

	if req.SSHPassword != "" {
		cs.redactor.Register(req.SSHPassword)
		enc, encErr := security.EncryptAEAD(key, req.SSHPassword)
		if encErr != nil {
			return ConnectionView{}, encErr
		}
		rec.SSHPasswordEnc = enc
	} else if exists && !req.ClearSSHPassword {
		rec.SSHPasswordEnc = existing.SSHPasswordEnc
	}

	if req.BridgeBearer != "" {
		cs.redactor.Register(req.BridgeBearer)
		enc, encErr := security.EncryptAEAD(key, req.BridgeBearer)
		if encErr != nil {
			return ConnectionView{}, encErr
		}
		rec.BridgeBearerEnc = enc
	} else if exists && !req.ClearBridgeBearer {
		rec.BridgeBearerEnc = existing.BridgeBearerEnc
	}

	prev := cs.items[id]
	cs.items[id] = rec
	if err := cs.saveLocked(); err != nil {
		if exists {
			cs.items[id] = prev
		} else {
			delete(cs.items, id)
		}
		return ConnectionView{}, err
	}
	return toConnectionView(rec), nil
}

// Delete removes a saved connection profile by ID.
func (cs *ConnectionStore) Delete(id string) (bool, error) {
	trimmed := strings.TrimSpace(id)
	if !reSafeConnID.MatchString(trimmed) {
		return false, security.NewError(security.CodeDeskE101, 400, "invalid connection id", nil)
	}
	cs.mu.Lock()
	defer cs.mu.Unlock()
	prev, exists := cs.items[trimmed]
	if !exists {
		return false, nil
	}
	delete(cs.items, trimmed)
	if err := cs.saveLocked(); err != nil {
		cs.items[trimmed] = prev
		return false, err
	}
	return true, nil
}

// ResolveSecrets decrypts stored secrets strictly in Go memory for tunnel/database connections.
func (cs *ConnectionStore) ResolveSecrets(id string) (dbPassword, sshPassword, bridgeBearer string, err error) {
	trimmed := strings.TrimSpace(id)
	if trimmed == "" {
		return "", "", "", nil
	}
	if !reSafeConnID.MatchString(trimmed) {
		return "", "", "", security.NewError(security.CodeDeskE101, 400, "invalid connection id", nil)
	}
	cs.mu.RLock()
	rec, exists := cs.items[trimmed]
	cs.mu.RUnlock()
	if !exists {
		return "", "", "", nil
	}
	key, err := security.ResolveMasterKey(cs.stateDir)
	if err != nil {
		return "", "", "", err
	}
	if rec.DBPasswordEnc != "" {
		dbPassword, err = security.DecryptAEAD(key, rec.DBPasswordEnc)
		if err != nil {
			return "", "", "", err
		}
		cs.redactor.Register(dbPassword)
	}
	if rec.SSHPasswordEnc != "" {
		sshPassword, err = security.DecryptAEAD(key, rec.SSHPasswordEnc)
		if err != nil {
			return "", "", "", err
		}
		cs.redactor.Register(sshPassword)
	}
	if rec.BridgeBearerEnc != "" {
		bridgeBearer, err = security.DecryptAEAD(key, rec.BridgeBearerEnc)
		if err != nil {
			return "", "", "", err
		}
		cs.redactor.Register(bridgeBearer)
	}
	return dbPassword, sshPassword, bridgeBearer, nil
}

func (cs *ConnectionStore) storePath() string {
	return filepath.Join(cs.stateDir, connectionsFileName)
}

func (cs *ConnectionStore) load() error {
	raw, err := os.ReadFile(cs.storePath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return security.NewError(security.CodeDeskE101, 500, "failed reading saved connections", err)
	}
	var list []SavedConnection
	if len(strings.TrimSpace(string(raw))) == 0 {
		return nil
	}
	if err := json.Unmarshal(raw, &list); err != nil {
		return security.NewError(security.CodeDeskE101, 500, "invalid saved connections store format", err)
	}
	for _, rec := range list {
		if reSafeConnID.MatchString(rec.ID) {
			cs.items[rec.ID] = rec
		}
	}
	return nil
}

func (cs *ConnectionStore) saveLocked() error {
	list := make([]SavedConnection, 0, len(cs.items))
	for _, rec := range cs.items {
		list = append(list, rec)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].ID < list[j].ID })
	raw, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return security.NewError(security.CodeDeskE101, 500, "failed encoding saved connections", err)
	}
	if err := os.WriteFile(cs.storePath(), raw, 0o600); err != nil {
		return security.NewError(security.CodeDeskE101, 500, "failed persisting saved connections", err)
	}
	return nil
}

func toConnectionView(rec SavedConnection) ConnectionView {
	masked := ""
	if rec.DBPasswordEnc != "" {
		masked = "********"
	}
	return ConnectionView{
		ID:                rec.ID,
		Name:              rec.Name,
		Engine:            rec.Engine,
		SQLitePath:        rec.SQLitePath,
		DBHost:            rec.DBHost,
		DBPort:            rec.DBPort,
		DBUser:            rec.DBUser,
		DBName:            rec.DBName,
		SchemaName:        rec.SchemaName,
		HasDBPassword:     rec.DBPasswordEnc != "",
		DBPasswordMasked:  masked,
		HasBridgeBearer:   rec.BridgeBearerEnc != "",
		SSHEnabled:        rec.SSHEnabled,
		SSHHost:           rec.SSHHost,
		SSHPort:           rec.SSHPort,
		SSHUser:           rec.SSHUser,
		SSHPrivateKeyPath: rec.SSHPrivateKeyPath,
		KnownHostsPath:    rec.KnownHostsPath,
		PinnedFingerprint: rec.PinnedFingerprint,
		RemoteDBPort:      rec.RemoteDBPort,
		HasSSHPassword:    rec.SSHPasswordEnc != "",
		CreatedAt:         rec.CreatedAt,
		UpdatedAt:         rec.UpdatedAt,
	}
}
