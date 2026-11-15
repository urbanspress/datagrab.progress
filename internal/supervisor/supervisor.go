package supervisor

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"groxy-desk/internal/security"
)

const (
	DefaultMinPort  = 8765
	DefaultMaxPort  = 8772
	MaxWorkers      = 8
	DefaultRingCap  = 256
	DefaultDrainSec = 30
)

// StartRequest defines parameters to launch a local Grisp (grab.exe) worker.
type StartRequest struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Subdir       string `json:"subdir"`
	GrabFile     string `json:"grab_file"`
	TemplateFile string `json:"template_file"`
	Port         string `json:"port"`
	AllowScripts bool   `json:"allow_scripts"`
}

// WorkerSnapshot represents the JSON-serializable state of a managed Grisp worker (ADR-025c base-10 strings).
type WorkerSnapshot struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Subdir        string `json:"subdir"`
	GrabFile      string `json:"grab_file"`
	TemplateFile  string `json:"template_file,omitempty"`
	Host          string `json:"host"`
	Port          string `json:"port"`
	URL           string `json:"url"`
	PID           string `json:"pid"`
	Status        string `json:"status"`
	HealthOK      bool   `json:"health_ok"`
	ReadyOK       bool   `json:"ready_ok"`
	HealthStatus  string `json:"health_status"`
	ReadyStatus   string `json:"ready_status"`
	ReloadCount   string `json:"reload_count"`
	LogCount      string `json:"log_count"`
	StartedAt     string `json:"started_at"`
	LastCheckedAt string `json:"last_checked_at"`
	LastError     string `json:"last_error,omitempty"`
}

// LogEntry holds a single secret-redacted log line from a Grisp worker.
type LogEntry struct {
	Seq       string `json:"seq"`
	Timestamp string `json:"timestamp"`
	Stream    string `json:"stream"`
	Line      string `json:"line"`
}

type ringBuffer struct {
	mu      sync.Mutex
	cap     int
	nextSeq uint64
	items   []LogEntry
}

func newRingBuffer(capacity int) *ringBuffer {
	if capacity <= 0 {
		capacity = DefaultRingCap
	}
	return &ringBuffer{cap: capacity}
}

func (rb *ringBuffer) add(stream, line string, redactor *security.SecretRedactor) {
	clean := line
	if redactor != nil {
		clean = redactor.Redact(clean)
	}
	rb.mu.Lock()
	defer rb.mu.Unlock()
	rb.nextSeq++
	entry := LogEntry{
		Seq:       strconv.FormatUint(rb.nextSeq, 10),
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Stream:    stream,
		Line:      clean,
	}
	if len(rb.items) >= rb.cap {
		copy(rb.items, rb.items[1:])
		rb.items[len(rb.items)-1] = entry
		return
	}
	rb.items = append(rb.items, entry)
}

func (rb *ringBuffer) list() []LogEntry {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	out := make([]LogEntry, len(rb.items))
	copy(out, rb.items)
	return out
}

func (rb *ringBuffer) count() int {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	return len(rb.items)
}

type managedWorker struct {
	mu            sync.Mutex
	id            string
	name          string
	subdir        string
	workDirAbs    string
	grabFile      string
	templateFile  string
	allowScripts  bool
	port          int
	pid           int
	status        string
	healthOK      bool
	readyOK       bool
	healthStatus  int
	readyStatus   int
	reloadCount   int
	startedAt     time.Time
	lastCheckedAt time.Time
	lastError     string

	cmd    *exec.Cmd
	cancel context.CancelFunc
	done   chan struct{}
	logs   *ringBuffer
}

// Manager supervises up to 8 local Grisp (grab.exe) worker processes on 127.0.0.1:8765-8772.
type Manager struct {
	mu            sync.Mutex
	grabBin       string
	workspaceRoot string
	redactor      *security.SecretRedactor
	minPort       int
	maxPort       int
	httpClient    *http.Client
	workers       map[string]*managedWorker
	pollStop      chan struct{}
	pollOnce      sync.Once
}

// NewManager creates a Grisp supervisor defaulting to loopback ports 127.0.0.1:8765..8772.
func NewManager(grabBin, workspaceRoot string, redactor *security.SecretRedactor) *Manager {
	return NewManagerWithPortRange(grabBin, workspaceRoot, redactor, DefaultMinPort, DefaultMaxPort)
}

// NewManagerWithPortRange creates a Grisp supervisor over an explicit loopback port range.
func NewManagerWithPortRange(grabBin, workspaceRoot string, redactor *security.SecretRedactor, minPort, maxPort int) *Manager {
	if minPort <= 0 || maxPort < minPort || (maxPort-minPort+1) > MaxWorkers {
		minPort = DefaultMinPort
		maxPort = DefaultMaxPort
	}
	m := &Manager{
		grabBin:       strings.TrimSpace(grabBin),
		workspaceRoot: workspaceRoot,
		redactor:      redactor,
		minPort:       minPort,
		maxPort:       maxPort,
		httpClient: &http.Client{
			Timeout: 1200 * time.Millisecond,
		},
		workers:  make(map[string]*managedWorker),
		pollStop: make(chan struct{}),
	}
	go m.pollLoop()
	return m
}

// SetGrabBinary updates the resolved GRAB executable path safely.
func (m *Manager) SetGrabBinary(path string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.grabBin = strings.TrimSpace(path)
}

// GrabBinary returns the configured GRAB executable path.
func (m *Manager) GrabBinary() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.grabBin
}

func (m *Manager) pollLoop() {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-m.pollStop:
			return
		case <-ticker.C:
			m.RefreshAllHealth(context.Background())
		}
	}
}

// RefreshAllHealth polls /_health and /_ready across all active workers.
func (m *Manager) RefreshAllHealth(ctx context.Context) {
	m.mu.Lock()
	list := make([]*managedWorker, 0, len(m.workers))
	for _, w := range m.workers {
		list = append(list, w)
	}
	m.mu.Unlock()

	for _, w := range list {
		m.probeWorker(ctx, w)
	}
}

func (m *Manager) probeWorker(ctx context.Context, w *managedWorker) {
	w.mu.Lock()
	status := w.status
	port := w.port
	w.mu.Unlock()

	if status == "stopped" || port == 0 {
		return
	}

	hCode, hOK := m.httpProbe(ctx, port, "/_health")
	rCode, rOK := m.httpProbe(ctx, port, "/_ready")

	w.mu.Lock()
	defer w.mu.Unlock()
	if w.status == "stopped" {
		return
	}
	w.healthStatus = hCode
	w.readyStatus = rCode
	w.healthOK = hOK
	w.readyOK = rOK
	w.lastCheckedAt = time.Now().UTC()
	if hOK && rOK {
		w.status = "ready"
		w.lastError = ""
	} else if w.status != "starting" && w.status != "reloading" {
		w.status = "degraded"
	}
}

func (m *Manager) httpProbe(ctx context.Context, port int, path string) (int, bool) {
	probeCtx, cancel := context.WithTimeout(ctx, 1200*time.Millisecond)
	defer cancel()
	url := fmt.Sprintf("http://127.0.0.1:%d%s", port, path)
	req, err := http.NewRequestWithContext(probeCtx, http.MethodGet, url, nil)
	if err != nil {
		return 0, false
	}
	resp, err := m.httpClient.Do(req)
	if err != nil {
		return 0, false
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	return resp.StatusCode, resp.StatusCode == http.StatusOK
}

// StartWorker spawns a local Grisp (grab serve) worker on 127.0.0.1:<port> and waits for /_health & /_ready == 200.
func (m *Manager) StartWorker(ctx context.Context, req StartRequest) (WorkerSnapshot, error) {
	m.mu.Lock()
	grabBin := m.grabBin
	root := m.workspaceRoot
	m.mu.Unlock()

	if grabBin == "" {
		return WorkerSnapshot{}, security.NewError(security.CodeDeskE301, 422, "GRAB v1.1.0 binary (grab.exe) not found", nil)
	}
	if err := security.ValidateSafeCommandBinary(grabBin, nil); err != nil {
		return WorkerSnapshot{}, err
	}

	subdir := strings.TrimSpace(req.Subdir)
	if subdir == "" {
		subdir = "."
	}
	workDirAbs, relSubdir, err := security.ConfineWorkspaceDir(root, subdir)
	if err != nil {
		return WorkerSnapshot{}, err
	}

	grabRel := strings.TrimSpace(req.GrabFile)
	if grabRel == "" {
		grabRel = "src/app.grab"
	}
	grabAbs, grabClean, err := security.ConfineWorkspacePath(workDirAbs, grabRel)
	if err != nil {
		return WorkerSnapshot{}, err
	}
	if !strings.HasSuffix(strings.ToLower(grabAbs), ".grab") {
		return WorkerSnapshot{}, security.NewError(security.CodeDeskE101, 400, "grab_file must end with .grab", nil)
	}
	if st, err := os.Stat(grabAbs); err != nil || !st.Mode().IsRegular() {
		return WorkerSnapshot{}, security.NewError(security.CodeDeskE101, 400, fmt.Sprintf("target GRAB program %q not found", grabClean), err)
	}

	tmplClean := strings.TrimSpace(req.TemplateFile)
	if tmplClean != "" {
		tmplAbs, cleaned, err := security.ConfineWorkspacePath(workDirAbs, tmplClean)
		if err != nil {
			return WorkerSnapshot{}, err
		}
		if !strings.HasSuffix(strings.ToLower(tmplAbs), ".grab.html") {
			return WorkerSnapshot{}, security.NewError(security.CodeDeskE101, 400, "template_file must end with .grab.html", nil)
		}
		if st, err := os.Stat(tmplAbs); err != nil || !st.Mode().IsRegular() {
			return WorkerSnapshot{}, security.NewError(security.CodeDeskE101, 400, fmt.Sprintf("template file %q not found", cleaned), err)
		}
		tmplClean = cleaned
	}

	reqPort := 0
	if strings.TrimSpace(req.Port) != "" {
		pStr := strings.TrimSpace(req.Port)
		if !security.IsExactIntString(pStr) {
			return WorkerSnapshot{}, security.NewError(security.CodeDeskE101, 400, "port must be an exact base-10 integer string", nil)
		}
		pVal, err := strconv.Atoi(pStr)
		if err != nil || pVal < m.minPort || pVal > m.maxPort {
			return WorkerSnapshot{}, security.NewError(
				security.CodeDeskE101,
				400,
				fmt.Sprintf("worker port must be in the local loopback pool %d..%d", m.minPort, m.maxPort),
				err,
			)
		}
		reqPort = pVal
	}

	workerID := strings.TrimSpace(req.ID)
	if workerID == "" {
		tok, _ := security.RandomTokenHex(4)
		workerID = "grisp-" + tok
	}
	workerName := strings.TrimSpace(req.Name)
	if workerName == "" {
		workerName = filepath.Base(workDirAbs)
	}

	m.mu.Lock()
	if _, exists := m.workers[workerID]; exists {
		m.mu.Unlock()
		return WorkerSnapshot{}, security.NewError(security.CodeDeskE409, 409, fmt.Sprintf("worker id %q already exists", workerID), nil)
	}
	activeCount := 0
	usedPorts := make(map[int]bool)
	for _, existing := range m.workers {
		existing.mu.Lock()
		if existing.status != "stopped" {
			activeCount++
			usedPorts[existing.port] = true
		}
		existing.mu.Unlock()
	}
	if activeCount >= MaxWorkers {
		m.mu.Unlock()
		return WorkerSnapshot{}, security.NewError(security.CodeDeskE409, 409, fmt.Sprintf("maximum of %d local Grisp workers already active", MaxWorkers), nil)
	}
	chosenPort := reqPort
	if chosenPort != 0 {
		if usedPorts[chosenPort] || !isLoopbackPortAvailable(chosenPort) {
			m.mu.Unlock()
			return WorkerSnapshot{}, security.NewError(security.CodeDeskE409, 409, fmt.Sprintf("port 127.0.0.1:%d is already in use", chosenPort), nil)
		}
	} else {
		for p := m.minPort; p <= m.maxPort; p++ {
			if !usedPorts[p] && isLoopbackPortAvailable(p) {
				chosenPort = p
				break
			}
		}
	}
	if chosenPort == 0 {
		m.mu.Unlock()
		return WorkerSnapshot{}, security.NewError(security.CodeDeskE409, 409, fmt.Sprintf("no free loopback port available in %d..%d", m.minPort, m.maxPort), nil)
	}

	w := &managedWorker{
		id:           workerID,
		name:         workerName,
		subdir:       relSubdir,
		workDirAbs:   workDirAbs,
		grabFile:     grabClean,
		templateFile: tmplClean,
		allowScripts: req.AllowScripts,
		port:         chosenPort,
		status:       "starting",
		startedAt:    time.Now().UTC(),
		logs:         newRingBuffer(DefaultRingCap),
	}
	m.workers[workerID] = w
	m.mu.Unlock()

	if err := m.launchProcess(ctx, w); err != nil {
		m.mu.Lock()
		delete(m.workers, workerID)
		m.mu.Unlock()
		return WorkerSnapshot{}, err
	}
	return w.snapshot(), nil
}

func (m *Manager) launchProcess(ctx context.Context, w *managedWorker) error {
	args, err := buildServeArgs(w.workDirAbs, w.port, w.grabFile, w.templateFile, w.allowScripts)
	if err != nil {
		return err
	}
	if err := security.ValidateSafeCommandBinary(m.grabBin, args); err != nil {
		return err
	}

	procCtx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(procCtx, m.grabBin, args...)
	cmd.Dir = w.workDirAbs
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		if runtime.GOOS != "windows" {
			_ = cmd.Process.Signal(os.Interrupt)
		}
		return cmd.Process.Kill()
	}
	cmd.WaitDelay = 2 * time.Second

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return security.NewError(security.CodeDeskE502, 502, "failed to attach worker stdout pipe", err)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		return security.NewError(security.CodeDeskE502, 502, "failed to attach worker stderr pipe", err)
	}

	if err := cmd.Start(); err != nil {
		cancel()
		return security.NewError(security.CodeDeskE502, 502, fmt.Sprintf("failed to start Grisp worker %q", w.name), err)
	}

	done := make(chan struct{})
	var pipesWG sync.WaitGroup
	pipesWG.Add(2)
	go func() {
		defer pipesWG.Done()
		streamLogLines(stdoutPipe, "stdout", w.logs, m.redactor)
	}()
	go func() {
		defer pipesWG.Done()
		streamLogLines(stderrPipe, "stderr", w.logs, m.redactor)
	}()

	go func() {
		pipesWG.Wait()
		err := cmd.Wait()
		w.mu.Lock()
		if w.status != "stopped" && w.status != "reloading" {
			w.status = "stopped"
			w.healthOK = false
			w.readyOK = false
			if err != nil && procCtx.Err() == nil {
				w.lastError = m.redact(err.Error())
			}
		}
		w.mu.Unlock()
		close(done)
	}()

	w.mu.Lock()
	w.cmd = cmd
	w.cancel = cancel
	w.done = done
	w.pid = cmd.Process.Pid
	w.status = "starting"
	w.healthOK = false
	w.readyOK = false
	w.mu.Unlock()

	if err := m.waitForReady(ctx, w, 12*time.Second); err != nil {
		_ = m.terminateWorkerProc(w, 5*time.Second)
		return err
	}
	return nil
}

func (m *Manager) waitForReady(ctx context.Context, w *managedWorker, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		w.mu.Lock()
		done := w.done
		port := w.port
		w.mu.Unlock()

		select {
		case <-done:
			logs := w.logs.list()
			tail := ""
			if len(logs) > 0 {
				tail = logs[len(logs)-1].Line
			}
			return security.NewError(
				security.CodeDeskE502,
				502,
				fmt.Sprintf("Grisp worker %q exited before /_ready became 200 (%s)", w.name, tail),
				nil,
			)
		default:
		}

		hCode, hOK := m.httpProbe(ctx, port, "/_health")
		rCode, rOK := m.httpProbe(ctx, port, "/_ready")
		w.mu.Lock()
		w.healthStatus = hCode
		w.readyStatus = rCode
		w.healthOK = hOK
		w.readyOK = rOK
		w.lastCheckedAt = time.Now().UTC()
		if hOK && rOK {
			w.status = "ready"
			w.lastError = ""
			w.mu.Unlock()
			return nil
		}
		w.mu.Unlock()

		if time.Now().After(deadline) || ctx.Err() != nil {
			return security.NewError(
				security.CodeDeskE502,
				502,
				fmt.Sprintf("timed out waiting for Grisp worker %q /_health and /_ready on 127.0.0.1:%d", w.name, port),
				ctx.Err(),
			)
		}
		time.Sleep(120 * time.Millisecond)
	}
}

// ReloadWorker runs `grab check` preflight first (preserving last-known-good on error),
// then atomically restarts/reloads the worker and verifies /_health and /_ready == 200.
func (m *Manager) ReloadWorker(ctx context.Context, id string) (WorkerSnapshot, error) {
	w, err := m.getWorker(id)
	if err != nil {
		return WorkerSnapshot{}, err
	}

	w.mu.Lock()
	if w.status == "stopped" {
		w.mu.Unlock()
		return WorkerSnapshot{}, security.NewError(security.CodeDeskE409, 409, fmt.Sprintf("worker %q is stopped", id), nil)
	}
	workDirAbs := w.workDirAbs
	grabFile := w.grabFile
	tmplFile := w.templateFile
	w.mu.Unlock()

	if err := m.runGrabCheckPreflight(ctx, workDirAbs, grabFile, tmplFile); err != nil {
		w.logs.add("stderr", fmt.Sprintf("reload rejected by grab check; continuing last-known-good (%v)", err), m.redactor)
		return WorkerSnapshot{}, err
	}

	w.mu.Lock()
	w.status = "reloading"
	w.mu.Unlock()

	if err := m.terminateWorkerProc(w, time.Duration(DefaultDrainSec)*time.Second); err != nil {
		return WorkerSnapshot{}, security.NewError(security.CodeDeskE502, 502, "failed draining previous worker generation", err)
	}

	if err := m.launchProcess(ctx, w); err != nil {
		return WorkerSnapshot{}, err
	}

	w.mu.Lock()
	w.reloadCount++
	w.logs.add("stdout", fmt.Sprintf("atomic reload completed cleanly (generation %d)", w.reloadCount), m.redactor)
	w.mu.Unlock()

	return w.snapshot(), nil
}

func (m *Manager) runGrabCheckPreflight(ctx context.Context, workDirAbs, grabFile, tmplFile string) error {
	args := []string{"check"}
	if _, err := os.Stat(filepath.Join(workDirAbs, "grab.platform.json")); err == nil {
		args = append(args, "--platform-config", "grab.platform.json")
	}
	if _, err := os.Stat(filepath.Join(workDirAbs, "grab.data.json")); err == nil {
		args = append(args, "--db-config", "grab.data.json")
	}
	args = append(args, grabFile)
	if tmplFile != "" {
		args = append(args, tmplFile)
	}
	if err := security.ValidateSafeCommandBinary(m.grabBin, args); err != nil {
		return err
	}

	checkCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(checkCtx, m.grabBin, args...)
	cmd.Dir = workDirAbs
	out, err := cmd.CombinedOutput()
	if err != nil {
		return security.NewError(
			security.CodeDeskE301,
			422,
			fmt.Sprintf("grab check failed during reload preflight: %s", strings.TrimSpace(m.redact(string(out)))),
			err,
		)
	}
	return nil
}

// StopWorker gracefully drains and stops the target worker.
func (m *Manager) StopWorker(ctx context.Context, id string) (WorkerSnapshot, error) {
	w, err := m.getWorker(id)
	if err != nil {
		return WorkerSnapshot{}, err
	}
	if err := m.terminateWorkerProc(w, time.Duration(DefaultDrainSec)*time.Second); err != nil {
		return WorkerSnapshot{}, err
	}
	w.mu.Lock()
	w.status = "stopped"
	w.healthOK = false
	w.readyOK = false
	w.healthStatus = 0
	w.readyStatus = 0
	w.mu.Unlock()
	return w.snapshot(), nil
}

// ListWorkers returns deterministic snapshots of all workers sorted by port and ID.
func (m *Manager) ListWorkers() []WorkerSnapshot {
	m.mu.Lock()
	list := make([]*managedWorker, 0, len(m.workers))
	for _, w := range m.workers {
		list = append(list, w)
	}
	m.mu.Unlock()

	out := make([]WorkerSnapshot, 0, len(list))
	for _, w := range list {
		out = append(out, w.snapshot())
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Port != out[j].Port {
			return out[i].Port < out[j].Port
		}
		return out[i].ID < out[j].ID
	})
	return out
}

// GetLogs returns bounded secret-redacted logs for a worker.
func (m *Manager) GetLogs(id string) ([]LogEntry, error) {
	w, err := m.getWorker(id)
	if err != nil {
		return nil, err
	}
	return w.logs.list(), nil
}

// ShutdownAll gracefully stops all running workers and the health poller.
func (m *Manager) ShutdownAll(ctx context.Context) error {
	m.pollOnce.Do(func() {
		close(m.pollStop)
	})

	m.mu.Lock()
	list := make([]*managedWorker, 0, len(m.workers))
	for _, w := range m.workers {
		list = append(list, w)
	}
	m.mu.Unlock()

	var firstErr error
	for _, w := range list {
		if err := m.terminateWorkerProc(w, time.Duration(DefaultDrainSec)*time.Second); err != nil && firstErr == nil {
			firstErr = err
		}
		w.mu.Lock()
		w.status = "stopped"
		w.healthOK = false
		w.readyOK = false
		w.mu.Unlock()
	}
	return firstErr
}

func (m *Manager) getWorker(id string) (*managedWorker, error) {
	cleanID := strings.TrimSpace(id)
	if cleanID == "" {
		return nil, security.NewError(security.CodeDeskE101, 400, "worker id is required", nil)
	}
	m.mu.Lock()
	w, ok := m.workers[cleanID]
	m.mu.Unlock()
	if !ok {
		return nil, security.NewError(security.CodeDeskE101, 404, fmt.Sprintf("worker %q not found", cleanID), nil)
	}
	return w, nil
}

func (m *Manager) terminateWorkerProc(w *managedWorker, timeout time.Duration) error {
	w.mu.Lock()
	cmd := w.cmd
	cancel := w.cancel
	done := w.done
	w.mu.Unlock()

	if cmd == nil || done == nil {
		return nil
	}
	select {
	case <-done:
		return nil
	default:
	}

	if cmd.Process != nil && runtime.GOOS != "windows" {
		_ = cmd.Process.Signal(os.Interrupt)
		select {
		case <-done:
			if cancel != nil {
				cancel()
			}
			return nil
		case <-time.After(3 * time.Second):
		}
	}

	if cancel != nil {
		cancel()
	}
	select {
	case <-done:
		return nil
	case <-time.After(timeout):
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		select {
		case <-done:
			return nil
		case <-time.After(2 * time.Second):
			return errors.New("timed out waiting for worker process to exit")
		}
	}
}

func (m *Manager) redact(s string) string {
	if m.redactor != nil {
		return m.redactor.Redact(s)
	}
	return s
}

func (w *managedWorker) snapshot() WorkerSnapshot {
	w.mu.Lock()
	defer w.mu.Unlock()
	checked := ""
	if !w.lastCheckedAt.IsZero() {
		checked = w.lastCheckedAt.Format(time.RFC3339)
	}
	return WorkerSnapshot{
		ID:            w.id,
		Name:          w.name,
		Subdir:        w.subdir,
		GrabFile:      w.grabFile,
		TemplateFile:  w.templateFile,
		Host:          "127.0.0.1",
		Port:          strconv.Itoa(w.port),
		URL:           fmt.Sprintf("http://127.0.0.1:%d", w.port),
		PID:           strconv.Itoa(w.pid),
		Status:        w.status,
		HealthOK:      w.healthOK,
		ReadyOK:       w.readyOK,
		HealthStatus:  strconv.Itoa(w.healthStatus),
		ReadyStatus:   strconv.Itoa(w.readyStatus),
		ReloadCount:   strconv.Itoa(w.reloadCount),
		LogCount:      strconv.Itoa(w.logs.count()),
		StartedAt:     w.startedAt.Format(time.RFC3339),
		LastCheckedAt: checked,
		LastError:     w.lastError,
	}
}

func streamLogLines(r io.Reader, stream string, rb *ringBuffer, redactor *security.SecretRedactor) {
	scanner := bufio.NewScanner(r)
	buf := make([]byte, 0, 16*1024)
	scanner.Buffer(buf, 256*1024)
	for scanner.Scan() {
		rb.add(stream, scanner.Text(), redactor)
	}
}

func isLoopbackPortAvailable(port int) bool {
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return false
	}
	_ = ln.Close()
	return true
}

func buildServeArgs(workDirAbs string, port int, grabFile, templateFile string, allowScripts bool) ([]string, error) {
	args := []string{
		"serve",
		"--addr", fmt.Sprintf("127.0.0.1:%d", port),
		"--reload",
		"--allow-read", ".",
	}
	if allowScripts {
		args = append(args, "--allow-scripts")
	}

	platPath := filepath.Join(workDirAbs, "grab.platform.json")
	if raw, err := os.ReadFile(platPath); err == nil {
		args = append(args, "--platform-config", "grab.platform.json")
		var parsed struct {
			Caches   map[string]any `json:"caches"`
			Channels map[string]any `json:"channels"`
		}
		if json.Unmarshal(raw, &parsed) == nil {
			var cacheNames []string
			for k := range parsed.Caches {
				cacheNames = append(cacheNames, k)
			}
			sort.Strings(cacheNames)
			for _, name := range cacheNames {
				args = append(args, "--allow-cache", name+":read-write")
			}
			var chanNames []string
			for k := range parsed.Channels {
				chanNames = append(chanNames, k)
			}
			sort.Strings(chanNames)
			for _, name := range chanNames {
				args = append(args, "--allow-channel", name+":publish")
			}
		}
	}

	dataPath := filepath.Join(workDirAbs, "grab.data.json")
	if raw, err := os.ReadFile(dataPath); err == nil {
		_ = os.MkdirAll(filepath.Join(workDirAbs, "storage"), 0o755)
		args = append(args, "--db-config", "grab.data.json")
		var parsed struct {
			Connections map[string]any `json:"connections"`
		}
		if json.Unmarshal(raw, &parsed) == nil {
			var connNames []string
			for k := range parsed.Connections {
				connNames = append(connNames, k)
			}
			sort.Strings(connNames)
			for _, name := range connNames {
				args = append(args, "--allow-db", name+":read-write")
			}
		}
	}

	if st, err := os.Stat(filepath.Join(workDirAbs, "public", "assets")); err == nil && st.IsDir() {
		args = append(args, "--static", "/assets/=public/assets/")
	} else if st, err := os.Stat(filepath.Join(workDirAbs, "public")); err == nil && st.IsDir() {
		args = append(args, "--static", "/static/=public/")
	}

	args = append(args, grabFile)
	if templateFile != "" {
		args = append(args, templateFile)
	}
	return args, nil
}
