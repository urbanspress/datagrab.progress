//go:build !windows

package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"groxy-desk/internal/server"
)

func launchWebViewWindow(sess server.SessionCredentials, _, _ string, _ bool) error {
	fmt.Printf("Groxy Desk loopback host active at %s (native WebView2 window requires Windows; headless daemon running)\n", sess.URL)
	sigCh := make(chan os.Signal, 2)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	<-sigCh
	return nil
}
