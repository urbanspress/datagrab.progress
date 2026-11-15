package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	groxydesk "groxy-desk"
	"groxy-desk/internal/server"
)

func main() {
	addr := flag.String("addr", server.DefaultListenAddr, "Loopback-only HTTP listen address (e.g. 127.0.0.1:9490)")
	workspaceDir := flag.String("workspace", ".", "Workspace root directory to confine file and Grisp operations")
	stateDir := flag.String("state-dir", "", "Directory for snapshots, WebView2 profile, and theme preferences outside workspace")
	grabBin := flag.String("grab-bin", "", "Optional explicit path to GRAB v1.1.0 binary")
	grabmanBin := flag.String("grabman-bin", "", "Optional explicit path to grabman v1.0.0 binary")
	ollamaURL := flag.String("ollama-url", "", "Optional loopback Ollama HTTP URL (fails closed DESK-E503 if unreachable)")
	smokeTest := flag.Bool("smoke-test", false, "Verify desktop launcher startup, embedded assets, and clean teardown without opening a GUI window")
	debug := flag.Bool("debug", false, "Enable WebView2 developer tools for local diagnostics")
	flag.Parse()

	if err := runDesktop(*addr, *workspaceDir, *stateDir, *grabBin, *grabmanBin, *ollamaURL, *smokeTest, *debug); err != nil {
		fmt.Fprintf(os.Stderr, "groxy-desk error: %v\n", err)
		os.Exit(1)
	}
}

func runDesktop(addr, workspaceDir, stateDir, grabBin, grabmanBin, ollamaURL string, smokeTest, debug bool) error {
	viewsFS, err := groxydesk.ViewsFS()
	if err != nil {
		return err
	}
	publicFS, err := groxydesk.PublicFS()
	if err != nil {
		return err
	}

	cleanState := strings.TrimSpace(stateDir)
	if cleanState == "" {
		cacheRoot, cacheErr := os.UserCacheDir()
		if cacheErr != nil || strings.TrimSpace(cacheRoot) == "" {
			tmpDir, tmpErr := os.MkdirTemp("", "groxy-desk-desktop-*")
			if tmpErr != nil {
				return tmpErr
			}
			cleanState = tmpDir
		} else {
			cleanState = filepath.Join(cacheRoot, "groxy-desk", "desktop-state")
		}
	}

	srv, err := server.New(server.Config{
		ListenAddr:    addr,
		WorkspaceRoot: workspaceDir,
		StateDir:      cleanState,
		GrabBin:       grabBin,
		GrabmanBin:    grabmanBin,
		OllamaURL:     ollamaURL,
		ViewsFS:       viewsFS,
		PublicFS:      publicFS,
	})
	if err != nil {
		return err
	}
	if err := srv.Start(); err != nil {
		return err
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	}()

	sess := srv.Session()
	if smokeTest {
		client := &http.Client{Timeout: 5 * time.Second}
		for _, ep := range []string{"/_health", "/_ready", "/", "/public/grex/grex.js", "/public/desk/workbench.js"} {
			resp, reqErr := client.Get(sess.URL + ep)
			if reqErr != nil {
				return fmt.Errorf("smoke check %s failed: %w", ep, reqErr)
			}
			_ = resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				return fmt.Errorf("smoke check %s returned HTTP %d", ep, resp.StatusCode)
			}
		}
		fmt.Printf("groxy-desk smoke-test ok bind=%s\n", srv.Addr())
		return nil
	}

	return launchWebViewWindow(sess, cleanState, debug)
}
