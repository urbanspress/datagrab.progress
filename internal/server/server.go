package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"mime"
	"net"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"groxy-desk/internal/author"
	"groxy-desk/internal/dbstudio"
	"groxy-desk/internal/dns"
	"groxy-desk/internal/fleet"
	"groxy-desk/internal/groxybridge"
	"groxy-desk/internal/mcpbridge"
	"groxy-desk/internal/radar"
	"groxy-desk/internal/security"
	"groxy-desk/internal/supervisor"
	"groxy-desk/internal/workspace"
)

const (
	// DefaultListenAddr is the fixed loopback-only desktop host daemon endpoint.
	DefaultListenAddr = "127.0.0.1:9490"

	maxJSONBodyBytes = 2 << 20 // 2 MiB
)

// Config defines all dependencies and runtime boundaries for the Groxy Desk HTTP host daemon.
type Config struct {
	ListenAddr    string
	WorkspaceRoot string
	StateDir      string
	GrabBin       string
	GrabmanBin    string
	OllamaURL     string
	BearerToken   string
	CSRFToken     string
	ViewsFS       fs.FS
	PublicFS      fs.FS
}

// SessionCredentials contains ephemeral loopback bearer and CSRF tokens delivered in memory to WebView2 or test harnesses.
type SessionCredentials struct {
	URL         string `json:"url"`
	BearerToken string `json:"bearer_token"`
	CSRFToken   string `json:"csrf_token"`
}

// Server implements the authenticated loopback-only Groxy Desk host API and static IDE host.
type Server struct {
	cfg           Config
	listenAddr    string
	bearerToken   string
	csrfToken     string
	stateDir      string
	prefsPath     string
	prefsMu       sync.Mutex
	redactor      *security.SecretRedactor
	workspaceMgr  *workspace.Manager
	supervisorMgr *supervisor.Manager
	approvals     *groxybridge.ApprovalManager
	groxy         *groxybridge.Bridge
	mcp           *mcpbridge.Bridge
	fleetMgr      *fleet.Manager
	dbStudio      *dbstudio.Manager
	radar         *radar.Manager
	dnsMgr        *dns.Manager
	authorMgr     *author.Manager
	viewsFS       fs.FS
	publicFS      fs.FS
	httpServer    *http.Server
	listener      net.Listener
	mu            sync.Mutex
}

// New constructs and validates a loopback-only Groxy Desk Server.
func New(cfg Config) (*Server, error) {
	addr := strings.TrimSpace(cfg.ListenAddr)
	if addr == "" {
		addr = DefaultListenAddr
	}
	validatedAddr, err := security.ValidateLoopbackBind(addr)
	if err != nil {
		return nil, err
	}

	wsRoot := strings.TrimSpace(cfg.WorkspaceRoot)
	if wsRoot == "" {
		wsRoot, err = os.Getwd()
		if err != nil {
			return nil, security.NewError(security.CodeDeskE101, 500, "failed to resolve current workspace root", err)
		}
	}

	stateDir := strings.TrimSpace(cfg.StateDir)
	if stateDir == "" {
		tmpState, err := os.MkdirTemp("", "groxy-desk-state-*")
		if err != nil {
			return nil, security.NewError(security.CodeDeskE101, 500, "failed to create state directory", err)
		}
		stateDir = tmpState
	}
	absState, err := filepath.Abs(stateDir)
	if err != nil {
		return nil, security.NewError(security.CodeDeskE101, 500, "invalid state directory", err)
	}
	if err := os.MkdirAll(absState, 0o700); err != nil {
		return nil, security.NewError(security.CodeDeskE101, 500, "failed to initialize state directory", err)
	}

	bearer := strings.TrimSpace(cfg.BearerToken)
	if bearer == "" {
		bearer, err = security.RandomTokenHex(32)
		if err != nil {
			return nil, security.NewError(security.CodeDeskE101, 500, "failed to generate bearer token", err)
		}
	}
	csrf := strings.TrimSpace(cfg.CSRFToken)
	if csrf == "" {
		csrf, err = security.RandomTokenHex(32)
		if err != nil {
			return nil, security.NewError(security.CodeDeskE101, 500, "failed to generate CSRF token", err)
		}
	}

	redactor := security.NewSecretRedactor(bearer, csrf)
	grabBin := workspace.ResolveGrabBinary(cfg.GrabBin)
	grabmanBin := workspace.ResolveGrabmanBinary(cfg.GrabmanBin)

	wsMgr, err := workspace.NewManager(wsRoot, grabBin, grabmanBin, redactor)
	if err != nil {
		return nil, err
	}
	supMgr := supervisor.NewManager(grabBin, wsMgr.Root(), redactor)
	apprMgr, err := groxybridge.NewApprovalManager(absState, redactor)
	if err != nil {
		return nil, err
	}
	bridge, err := groxybridge.NewBridge(wsMgr, supMgr, apprMgr, redactor, cfg.OllamaURL)
	if err != nil {
		return nil, err
	}
	mcpBridge, err := mcpbridge.New(wsMgr.Root(), grabBin, absState, apprMgr, redactor)
	if err != nil {
		return nil, err
	}
	fleetMgr, err := fleet.NewManager(absState, wsMgr.Root(), wsMgr, apprMgr, redactor)
	if err != nil {
		return nil, err
	}
	dbStudio := dbstudio.NewManager(wsMgr.Root(), apprMgr, redactor)
	dnsMgr, err := dns.NewManager(absState, apprMgr, redactor)
	if err != nil {
		dbStudio.CloseAll()
		return nil, err
	}
	authorMgr, err := author.NewManager(wsMgr.Root(), cfg.OllamaURL, apprMgr, redactor)
	if err != nil {
		dbStudio.CloseAll()
		return nil, err
	}

	if cfg.ViewsFS == nil {
		cfg.ViewsFS = os.DirFS(filepath.Join(wsMgr.Root(), "views"))
	}
	if cfg.PublicFS == nil {
		cfg.PublicFS = os.DirFS(filepath.Join(wsMgr.Root(), "public"))
	}

	s := &Server{
		cfg:           cfg,
		listenAddr:    validatedAddr,
		bearerToken:   bearer,
		csrfToken:     csrf,
		stateDir:      absState,
		prefsPath:     filepath.Join(absState, "prefs.json"),
		redactor:      redactor,
		workspaceMgr:  wsMgr,
		supervisorMgr: supMgr,
		approvals:     apprMgr,
		groxy:         bridge,
		mcp:           mcpBridge,
		fleetMgr:      fleetMgr,
		dbStudio:      dbStudio,
		radar:         radar.NewManager(apprMgr, absState),
		dnsMgr:        dnsMgr,
		authorMgr:     authorMgr,
		viewsFS:       cfg.ViewsFS,
		publicFS:      cfg.PublicFS,
	}

	mux := http.NewServeMux()
	s.registerRoutes(mux)

	s.httpServer = &http.Server{
		Handler:           s.securityHeadersMiddleware(mux),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      90 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	return s, nil
}

// Start binds the strictly validated loopback TCP socket and begins serving requests.
func (s *Server) Start() error {
	validated, err := security.ValidateLoopbackBind(s.listenAddr)
	if err != nil {
		return err
	}
	ln, err := net.Listen("tcp", validated)
	if err != nil {
		return security.NewError(security.CodeDeskE501, 500, "failed to bind loopback host socket", err)
	}

	s.mu.Lock()
	s.listener = ln
	s.listenAddr = ln.Addr().String()
	s.mu.Unlock()

	go func() {
		if serveErr := s.httpServer.Serve(ln); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			// Listener closed cleanly on shutdown; unexpected errors end Serve.
		}
	}()
	return nil
}

// Addr returns the live bound loopback host:port string.
func (s *Server) Addr() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.listener != nil {
		return s.listener.Addr().String()
	}
	return s.listenAddr
}

// BaseURL returns the base http://127.0.0.1:<port> URL of the running daemon.
func (s *Server) BaseURL() string {
	return "http://" + s.Addr()
}

// Session returns the in-memory loopback URL, bearer token, and CSRF token.
func (s *Server) Session() SessionCredentials {
	return SessionCredentials{
		URL:         s.BaseURL(),
		BearerToken: s.bearerToken,
		CSRFToken:   s.csrfToken,
	}
}

// Shutdown drains managed Grisp workers, closes active SSH DB tunnels, stops spawned Author runtimes, and stops the HTTP server.
func (s *Server) Shutdown(ctx context.Context) error {
	if s.dbStudio != nil {
		s.dbStudio.CloseAll()
	}
	if s.authorMgr != nil {
		s.authorMgr.Shutdown()
	}
	supErr := s.supervisorMgr.ShutdownAll(ctx)
	srvErr := s.httpServer.Shutdown(ctx)
	if supErr != nil {
		return supErr
	}
	return srvErr
}

func (s *Server) registerRoutes(mux *http.ServeMux) {
	// Unauthenticated liveness, UI document shell, and static UI bundles.
	mux.HandleFunc("GET /_health", s.handleHealth)
	mux.HandleFunc("GET /_ready", s.handleReady)
	mux.HandleFunc("GET /public/{rest...}", s.handleStaticPublic)
	mux.HandleFunc("GET /", s.handleIndex)

	// Strictly authenticated + CSRF protected local host API endpoints.
	mux.Handle("GET /api/preferences/theme", s.authCSRF(http.HandlerFunc(s.handleGetTheme)))
	mux.Handle("POST /api/preferences/theme", s.authCSRF(http.HandlerFunc(s.handleSetTheme)))

	mux.Handle("GET /api/workspace/tree", s.authCSRF(http.HandlerFunc(s.handleWorkspaceTree)))
	mux.Handle("GET /api/workspace/read", s.authCSRF(http.HandlerFunc(s.handleWorkspaceRead)))
	mux.Handle("POST /api/workspace/check", s.authCSRF(http.HandlerFunc(s.handleWorkspaceCheck)))

	mux.Handle("POST /api/grabman/init/plan", s.authCSRF(http.HandlerFunc(s.handleGrabmanInitPlan)))
	mux.Handle("POST /api/grabman/init", s.authCSRF(http.HandlerFunc(s.handleGrabmanInit)))
	mux.Handle("POST /api/grabman/doctor", s.authCSRF(http.HandlerFunc(s.handleGrabmanDoctor)))
	mux.Handle("POST /api/grabman/solve-verify", s.authCSRF(http.HandlerFunc(s.handleGrabmanSolveVerify)))

	mux.Handle("GET /api/grisp/workers", s.authCSRF(http.HandlerFunc(s.handleListWorkers)))
	mux.Handle("POST /api/grisp/workers/start", s.authCSRF(http.HandlerFunc(s.handleStartWorker)))
	mux.Handle("POST /api/grisp/workers/reload", s.authCSRF(http.HandlerFunc(s.handleReloadWorker)))
	mux.Handle("POST /api/grisp/workers/stop", s.authCSRF(http.HandlerFunc(s.handleStopWorker)))
	mux.Handle("GET /api/grisp/workers/logs", s.authCSRF(http.HandlerFunc(s.handleWorkerLogs)))

	mux.Handle("POST /api/groxy/ask", s.authCSRF(http.HandlerFunc(s.handleGroxyAsk)))
	mux.Handle("POST /api/groxy/approvals/apply", s.authCSRF(http.HandlerFunc(s.handleGroxyApply)))
	mux.Handle("GET /api/groxy/snapshots", s.authCSRF(http.HandlerFunc(s.handleListSnapshots)))
	mux.Handle("POST /api/groxy/snapshots/restore", s.authCSRF(http.HandlerFunc(s.handleRestoreSnapshot)))

	// v0.2.0 Unified MCP Studio, Split-Preview, 22-Rule Auditor & "Let Groxy Boost This" endpoints.
	mux.Handle("GET /api/mcp/tools", s.authCSRF(http.HandlerFunc(s.handleMCPTools)))
	mux.Handle("POST /api/mcp/call", s.authCSRF(http.HandlerFunc(s.handleMCPCall)))
	mux.Handle("GET /api/studio/design-system", s.authCSRF(http.HandlerFunc(s.handleStudioDesignSystem)))
	mux.Handle("POST /api/studio/synthesize", s.authCSRF(http.HandlerFunc(s.handleStudioSynthesize)))
	mux.Handle("POST /api/studio/layout/apply", s.authCSRF(http.HandlerFunc(s.handleStudioLayoutApply)))
	mux.Handle("POST /api/groxy/audit", s.authCSRF(http.HandlerFunc(s.handleGroxyAuditV2)))
	mux.Handle("POST /api/groxy/boost", s.authCSRF(http.HandlerFunc(s.handleGroxyBoost)))
	mux.Handle("POST /api/groxy/boost/approve", s.authCSRF(http.HandlerFunc(s.handleGroxyBoostApprove)))

	// v0.3.0 Step 5 Fleet, Pinned SSH DB Tunnels + Explorer, Namecheap DNS/HTTPS & Author Mode endpoints.
	s.registerV030Routes(mux)
	s.registerRadarRoutes(mux)
}

func (s *Server) securityHeadersMiddleware(next http.Handler) http.Handler {
	const csp = "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; connect-src 'self' http://127.0.0.1:*; object-src 'none'; frame-ancestors 'none'; base-uri 'self'"
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", csp)
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		if strings.HasPrefix(r.URL.Path, "/api/") || r.URL.Path == "/_health" || r.URL.Path == "/_ready" {
			h.Set("Cache-Control", "no-store")
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) authCSRF(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
		const bearerPrefix = "Bearer "
		if !strings.HasPrefix(authHeader, bearerPrefix) {
			s.writeError(w, security.NewError(security.CodeDeskE403, 403, "missing or invalid Authorization bearer header", nil))
			return
		}
		gotBearer := strings.TrimSpace(strings.TrimPrefix(authHeader, bearerPrefix))
		if !security.ConstantTimeEqual(gotBearer, s.bearerToken) {
			s.writeError(w, security.NewError(security.CodeDeskE403, 403, "invalid Authorization bearer token", nil))
			return
		}

		gotCSRF := strings.TrimSpace(r.Header.Get("X-CSRF-Token"))
		if !security.ConstantTimeEqual(gotCSRF, s.csrfToken) {
			s.writeError(w, security.NewError(security.CodeDeskE403, 403, "missing or invalid X-CSRF-Token header", nil))
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	workers := s.supervisorMgr.ListWorkers()
	s.writeJSON(w, http.StatusOK, map[string]any{
		"status":       "ok",
		"service":      "groxy-desk",
		"version":      "1.3.0",
		"bind":         s.Addr(),
		"worker_count": strconv.Itoa(len(workers)),
	})
}

func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	grabReady := s.workspaceMgr.GrabBinary() != ""
	grabmanReady := s.workspaceMgr.GrabmanBinary() != ""
	s.writeJSON(w, http.StatusOK, map[string]any{
		"ready":          true,
		"service":        "groxy-desk",
		"workspace_root": s.workspaceMgr.Root(),
		"grab_ready":     grabReady,
		"grabman_ready":  grabmanReady,
	})
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		s.writeError(w, security.NewError(security.CodeDeskE101, 404, "not found", nil))
		return
	}
	raw, err := fs.ReadFile(s.viewsFS, "workbench.grab.html")
	if err != nil {
		s.writeError(w, security.NewError(security.CodeDeskE101, 500, "workbench view template missing", err))
		return
	}
	theme := s.readThemePreference()
	rendered := strings.ReplaceAll(string(raw), "{initial_theme}", theme)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, rendered)
}

func (s *Server) handleStaticPublic(w http.ResponseWriter, r *http.Request) {
	rel := path.Clean("/" + r.PathValue("rest"))
	rel = strings.TrimPrefix(rel, "/")
	if rel == "" || rel == "." || strings.Contains(rel, "..") {
		s.writeError(w, security.NewError(security.CodeGrabE401, 403, "invalid public asset path", nil))
		return
	}
	info, err := fs.Stat(s.publicFS, rel)
	if err != nil || info.IsDir() {
		s.writeError(w, security.NewError(security.CodeDeskE101, 404, "static asset not found", nil))
		return
	}
	data, err := fs.ReadFile(s.publicFS, rel)
	if err != nil {
		s.writeError(w, security.NewError(security.CodeDeskE101, 500, "failed to read static asset", err))
		return
	}
	ext := strings.ToLower(path.Ext(rel))
	ctype := mime.TypeByExtension(ext)
	switch ext {
	case ".js":
		ctype = "application/javascript; charset=utf-8"
	case ".json":
		ctype = "application/json; charset=utf-8"
	case ".css":
		ctype = "text/css; charset=utf-8"
	case ".svg":
		ctype = "image/svg+xml"
	case ".woff2":
		ctype = "font/woff2"
	case ".woff":
		ctype = "font/woff"
	default:
		if ctype == "" {
			ctype = "application/octet-stream"
		}
	}
	w.Header().Set("Content-Type", ctype)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

type themePayload struct {
	Theme string `json:"theme"`
}

func (s *Server) readThemePreference() string {
	s.prefsMu.Lock()
	defer s.prefsMu.Unlock()

	raw, err := os.ReadFile(s.prefsPath)
	if err != nil {
		return "light"
	}
	var payload themePayload
	if json.Unmarshal(raw, &payload) == nil && (payload.Theme == "dark" || payload.Theme == "light") {
		return payload.Theme
	}
	return "light"
}

func (s *Server) handleGetTheme(w http.ResponseWriter, r *http.Request) {
	s.writeJSON(w, http.StatusOK, themePayload{Theme: s.readThemePreference()})
}

func (s *Server) handleSetTheme(w http.ResponseWriter, r *http.Request) {
	var payload themePayload
	if err := decodeJSONBody(r, &payload); err != nil {
		s.writeError(w, err)
		return
	}
	theme := strings.ToLower(strings.TrimSpace(payload.Theme))
	if theme != "light" && theme != "dark" {
		s.writeError(w, security.NewError(security.CodeDeskE101, 400, "theme must be light or dark", nil))
		return
	}

	s.prefsMu.Lock()
	raw, _ := json.MarshalIndent(themePayload{Theme: theme}, "", "  ")
	writeErr := os.WriteFile(s.prefsPath, raw, 0o600)
	s.prefsMu.Unlock()
	if writeErr != nil {
		s.writeError(w, security.NewError(security.CodeDeskE101, 500, "failed to persist theme preference", writeErr))
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]any{"ok": true, "theme": theme})
}

func (s *Server) handleWorkspaceTree(w http.ResponseWriter, r *http.Request) {
	dir := strings.TrimSpace(r.URL.Query().Get("dir"))
	if dir == "" {
		dir = "."
	}
	entries, err := s.workspaceMgr.Tree(dir)
	if err != nil {
		s.writeError(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]any{
		"workspace_root": s.workspaceMgr.Root(),
		"dir":            dir,
		"entries":        entries,
	})
}

func (s *Server) handleWorkspaceRead(w http.ResponseWriter, r *http.Request) {
	relPath := strings.TrimSpace(r.URL.Query().Get("path"))
	if relPath == "" {
		s.writeError(w, security.NewError(security.CodeDeskE101, 400, "path query parameter is required", nil))
		return
	}
	fc, err := s.workspaceMgr.ReadFile(relPath)
	if err != nil {
		s.writeError(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, fc)
}

type workspaceCheckReq struct {
	GrabFile     string `json:"grab_file"`
	TemplateFile string `json:"template_file"`
}

func (s *Server) handleWorkspaceCheck(w http.ResponseWriter, r *http.Request) {
	var req workspaceCheckReq
	if err := decodeJSONBody(r, &req); err != nil {
		s.writeError(w, err)
		return
	}
	res, err := s.workspaceMgr.CheckFiles(r.Context(), req.GrabFile, req.TemplateFile)
	if err != nil {
		s.writeError(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, res)
}

type grabmanInitReq struct {
	ApprovalToken string `json:"approval_token,omitempty"`
	TargetDir     string `json:"target_dir"`
	Profile       string `json:"profile"`
	ProjectName   string `json:"project_name"`
	EdgeProxy     string `json:"edge_proxy"`
	Force         bool   `json:"force"`
}

func (s *Server) handleGrabmanInitPlan(w http.ResponseWriter, r *http.Request) {
	var req grabmanInitReq
	if err := decodeJSONBody(r, &req); err != nil {
		s.writeError(w, err)
		return
	}
	target := strings.TrimSpace(req.TargetDir)
	if target == "" {
		target = "."
	}
	_, relSlash, err := security.ConfineWorkspaceDir(s.workspaceMgr.Root(), target)
	if err != nil {
		s.writeError(w, err)
		return
	}
	profile := strings.TrimSpace(req.Profile)
	if profile == "" {
		profile = "app"
	}
	edgeProxy := strings.TrimSpace(req.EdgeProxy)
	if edgeProxy == "" {
		edgeProxy = "caddy"
	}
	appr, err := s.approvals.IssueActionToken(groxybridge.ActionGrabmanInit, relSlash)
	if err != nil {
		s.writeError(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]any{
		"approval_required":   true,
		"approval_token":      appr.Token,
		"approval_expires_at": appr.ExpiresAt.Format(time.RFC3339),
		"target_dir":          relSlash,
		"profile":             profile,
		"edge_proxy":          edgeProxy,
		"project_name":        req.ProjectName,
	})
}

func (s *Server) handleGrabmanInit(w http.ResponseWriter, r *http.Request) {
	var req grabmanInitReq
	if err := decodeJSONBody(r, &req); err != nil {
		s.writeError(w, err)
		return
	}
	target := strings.TrimSpace(req.TargetDir)
	if target == "" {
		target = "."
	}
	_, relSlash, err := security.ConfineWorkspaceDir(s.workspaceMgr.Root(), target)
	if err != nil {
		s.writeError(w, err)
		return
	}
	if _, err := s.approvals.ConsumeToken(req.ApprovalToken, groxybridge.ActionGrabmanInit, relSlash); err != nil {
		s.writeError(w, err)
		return
	}
	res, err := s.workspaceMgr.GrabmanInit(r.Context(), relSlash, req.Profile, req.ProjectName, req.EdgeProxy, req.Force)
	if err != nil {
		s.writeError(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, res)
}

type grabmanDoctorReq struct {
	TargetDir string `json:"target_dir"`
	Strict    bool   `json:"strict"`
}

func (s *Server) handleGrabmanDoctor(w http.ResponseWriter, r *http.Request) {
	var req grabmanDoctorReq
	if err := decodeJSONBody(r, &req); err != nil {
		s.writeError(w, err)
		return
	}
	res, err := s.workspaceMgr.GrabmanDoctor(r.Context(), req.TargetDir, req.Strict)
	if err != nil {
		s.writeError(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, res)
}

type grabmanSolveReq struct {
	TargetDir string `json:"target_dir"`
}

func (s *Server) handleGrabmanSolveVerify(w http.ResponseWriter, r *http.Request) {
	var req grabmanSolveReq
	if err := decodeJSONBody(r, &req); err != nil {
		s.writeError(w, err)
		return
	}
	res, err := s.workspaceMgr.GrabmanSolveVerify(r.Context(), req.TargetDir)
	if err != nil {
		s.writeError(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, res)
}

func (s *Server) handleListWorkers(w http.ResponseWriter, r *http.Request) {
	s.supervisorMgr.RefreshAllHealth(r.Context())
	workers := s.supervisorMgr.ListWorkers()
	s.writeJSON(w, http.StatusOK, map[string]any{
		"workers":     workers,
		"max_workers": strconv.Itoa(supervisor.MaxWorkers),
	})
}

func (s *Server) handleStartWorker(w http.ResponseWriter, r *http.Request) {
	var req supervisor.StartRequest
	if err := decodeJSONBody(r, &req); err != nil {
		s.writeError(w, err)
		return
	}
	snap, err := s.supervisorMgr.StartWorker(r.Context(), req)
	if err != nil {
		s.writeError(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, snap)
}

type workerIDReq struct {
	ID string `json:"id"`
}

func (s *Server) handleReloadWorker(w http.ResponseWriter, r *http.Request) {
	var req workerIDReq
	if err := decodeJSONBody(r, &req); err != nil {
		s.writeError(w, err)
		return
	}
	snap, err := s.supervisorMgr.ReloadWorker(r.Context(), req.ID)
	if err != nil {
		s.writeError(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, snap)
}

func (s *Server) handleStopWorker(w http.ResponseWriter, r *http.Request) {
	var req workerIDReq
	if err := decodeJSONBody(r, &req); err != nil {
		s.writeError(w, err)
		return
	}
	snap, err := s.supervisorMgr.StopWorker(r.Context(), req.ID)
	if err != nil {
		s.writeError(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, snap)
}

func (s *Server) handleWorkerLogs(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.URL.Query().Get("id"))
	if id == "" {
		s.writeError(w, security.NewError(security.CodeDeskE101, 400, "id query parameter is required", nil))
		return
	}
	logs, err := s.supervisorMgr.GetLogs(id)
	if err != nil {
		s.writeError(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]any{
		"id":   id,
		"logs": logs,
	})
}

func (s *Server) handleGroxyAsk(w http.ResponseWriter, r *http.Request) {
	var req groxybridge.AskRequest
	if err := decodeJSONBody(r, &req); err != nil {
		s.writeError(w, err)
		return
	}
	resp, err := s.groxy.Ask(r.Context(), req)
	if err != nil {
		s.writeError(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleGroxyApply(w http.ResponseWriter, r *http.Request) {
	var req groxybridge.ApplyRequest
	if err := decodeJSONBody(r, &req); err != nil {
		s.writeError(w, err)
		return
	}
	res, err := s.approvals.ApplyMutation(s.workspaceMgr.Root(), req)
	if err != nil {
		s.writeError(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, res)
}

func (s *Server) handleListSnapshots(w http.ResponseWriter, r *http.Request) {
	snaps := s.approvals.ListSnapshots()
	s.writeJSON(w, http.StatusOK, map[string]any{
		"snapshots": snaps,
	})
}

type restoreSnapReq struct {
	SnapshotID string `json:"snapshot_id"`
}

func (s *Server) handleRestoreSnapshot(w http.ResponseWriter, r *http.Request) {
	var req restoreSnapReq
	if err := decodeJSONBody(r, &req); err != nil {
		s.writeError(w, err)
		return
	}
	res, err := s.approvals.RestoreSnapshot(s.workspaceMgr.Root(), req.SnapshotID)
	if err != nil {
		s.writeError(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, res)
}

func decodeJSONBody(r *http.Request, dst any) error {
	limited := io.LimitReader(r.Body, maxJSONBodyBytes)
	dec := json.NewDecoder(limited)
	if err := dec.Decode(dst); err != nil {
		return security.NewError(security.CodeDeskE101, 400, "invalid JSON request payload", err)
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return security.NewError(security.CodeDeskE101, 400, "unexpected trailing JSON data", nil)
	}
	return nil
}

func (s *Server) writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(true)
	_ = enc.Encode(payload)
}

func (s *Server) writeError(w http.ResponseWriter, err error) {
	status := security.ErrorStatus(err, http.StatusInternalServerError)
	code := security.ErrorCode(err, security.CodeDeskE101)
	msg := s.redactor.Redact(err.Error())
	s.writeJSON(w, status, map[string]any{
		"ok": false,
		"error": map[string]string{
			"code":    code,
			"message": msg,
		},
	})
}
