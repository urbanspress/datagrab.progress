package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	groxydesk "groxy-desk"
	"groxy-desk/internal/security"
	"groxy-desk/internal/server"
)

func main() {
	addr := flag.String("addr", server.DefaultListenAddr, "Loopback-only HTTP listen address (e.g. 127.0.0.1:9490)")
	workspaceDir := flag.String("workspace", ".", "Workspace root directory to confine file and Grisp operations")
	stateDir := flag.String("state-dir", "", "Directory for snapshots and theme preferences outside workspace")
	grabBin := flag.String("grab-bin", "", "Optional explicit path to GRAB v1.1.0 binary")
	grabmanBin := flag.String("grabman-bin", "", "Optional explicit path to grabman v1.0.0 binary")
	ollamaURL := flag.String("ollama-url", "", "Optional loopback Ollama HTTP URL (fails closed DESK-E503 if unreachable)")
	emitSession := flag.Bool("emit-session", false, "Emit single-line JSON session credentials to stdout for parent IPC")
	sessionFile := flag.String("session-file", "", "Optional 0600 file path to write JSON session credentials")
	smokeTest := flag.Bool("smoke-test", false, "Start loopback host daemon, verify /_health and /_ready, and exit cleanly")
	flag.Parse()

	if err := runDaemon(daemonOpts{
		addr:         *addr,
		workspaceDir: *workspaceDir,
		stateDir:     *stateDir,
		grabBin:      *grabBin,
		grabmanBin:   *grabmanBin,
		ollamaURL:    *ollamaURL,
		emitSession:  *emitSession,
		sessionFile:  *sessionFile,
		smokeTest:    *smokeTest,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "groxy-deskd error: %v\n", err)
		os.Exit(1)
	}
}

type daemonOpts struct {
	addr         string
	workspaceDir string
	stateDir     string
	grabBin      string
	grabmanBin   string
	ollamaURL    string
	emitSession  bool
	sessionFile  string
	smokeTest    bool
}

func runDaemon(opts daemonOpts) error {
	viewsFS, err := groxydesk.ViewsFS()
	if err != nil {
		return err
	}
	publicFS, err := groxydesk.PublicFS()
	if err != nil {
		return err
	}

	stateDir := strings.TrimSpace(opts.stateDir)
	if stateDir == "" {
		cacheRoot, cacheErr := os.UserCacheDir()
		if cacheErr != nil || strings.TrimSpace(cacheRoot) == "" {
			tmpDir, tmpErr := os.MkdirTemp("", "groxy-desk-state-*")
			if tmpErr != nil {
				return tmpErr
			}
			stateDir = tmpDir
		} else {
			stateDir = filepath.Join(cacheRoot, "groxy-desk", "host-state")
		}
	}

	srv, err := server.New(server.Config{
		ListenAddr:    opts.addr,
		WorkspaceRoot: opts.workspaceDir,
		StateDir:      stateDir,
		GrabBin:       opts.grabBin,
		GrabmanBin:    opts.grabmanBin,
		OllamaURL:     opts.ollamaURL,
		BearerToken:   strings.TrimSpace(os.Getenv("GROXY_DESK_BEARER_TOKEN")),
		CSRFToken:     strings.TrimSpace(os.Getenv("GROXY_DESK_CSRF_TOKEN")),
		ViewsFS:       viewsFS,
		PublicFS:      publicFS,
	})
	if err != nil {
		return err
	}

	if err := srv.Start(); err != nil {
		return err
	}

	shutdownSrv := func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return srv.Shutdown(ctx)
	}

	sess := srv.Session()
	if opts.sessionFile != "" {
		rawSess, _ := json.Marshal(sess)
		if writeErr := os.WriteFile(opts.sessionFile, append(rawSess, '\n'), 0o600); writeErr != nil {
			_ = shutdownSrv()
			return security.NewError(security.CodeDeskE101, 500, "failed to write session file", writeErr)
		}
	}
	if opts.emitSession {
		rawSess, _ := json.Marshal(sess)
		fmt.Println(string(rawSess))
	} else {
		fmt.Fprintf(os.Stderr, "groxy-deskd listening strictly on %s\n", sess.URL)
	}

	if opts.smokeTest {
		if err := verifyLoopbackEndpoints(sess.URL); err != nil {
			_ = shutdownSrv()
			return err
		}
		return shutdownSrv()
	}

	sigCh := make(chan os.Signal, 2)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	<-sigCh
	signal.Stop(sigCh)
	return shutdownSrv()
}

func verifyLoopbackEndpoints(baseURL string) error {
	client := &http.Client{Timeout: 5 * time.Second}
	for _, ep := range []string{"/_health", "/_ready", "/"} {
		resp, err := client.Get(baseURL + ep)
		if err != nil {
			return fmt.Errorf("smoke probe %s failed: %w", ep, err)
		}
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("smoke probe %s returned status %d", ep, resp.StatusCode)
		}
	}
	return nil
}
