//go:build windows

package main

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"groxy-desk/internal/security"
)

// researchProxy enforces the selected HTTPS authority even when WebView2
// omits a WebResourceRequested event. TLS stays between the pane and the site.
type researchProxy struct {
	server *http.Server
	addr   string

	mu     sync.Mutex
	origin string
	ctx    context.Context
	cancel context.CancelFunc
}

func startResearchProxy() (*researchProxy, error) {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	p := &researchProxy{addr: listener.Addr().String()}
	p.server = &http.Server{Handler: http.HandlerFunc(p.connect), ReadHeaderTimeout: 5 * time.Second, MaxHeaderBytes: 8192}
	go func() { _ = p.server.Serve(listener) }()
	return p, nil
}

func (p *researchProxy) setOrigin(origin string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.origin == origin {
		return
	}
	if p.cancel != nil {
		p.cancel()
		p.ctx, p.cancel = nil, nil
	}
	p.origin = origin
	if origin != "" {
		p.ctx, p.cancel = context.WithCancel(context.Background())
	}
}

func (p *researchProxy) close() {
	p.setOrigin("")
	_ = p.server.Close()
}

func (p *researchProxy) connect(w http.ResponseWriter, r *http.Request) {
	// Accept only CONNECT in authority form. The actual TLS handshake and
	// certificate validation still run in WebView2.
	if r.Method != http.MethodConnect || r.URL.Host == "" || r.URL.Scheme != "" ||
		r.URL.Path != "" || r.URL.RawQuery != "" || r.URL.User != nil {
		http.Error(w, "GRAB-E402", http.StatusForbidden)
		return
	}
	origin, err := security.ExactOrigin(&url.URL{Scheme: "https", Host: r.URL.Host})
	p.mu.Lock()
	allowed := err == nil && origin == p.origin
	ctx := p.ctx
	p.mu.Unlock()
	if !allowed {
		http.Error(w, "GRAB-E402", http.StatusForbidden)
		return
	}
	dialer := net.Dialer{Timeout: 10 * time.Second}
	remote, err := dialer.DialContext(ctx, "tcp", strings.TrimPrefix(origin, "https://"))
	if err != nil {
		http.Error(w, "Research connection failed", http.StatusBadGateway)
		return
	}
	defer remote.Close()
	client, buffer, err := http.NewResponseController(w).Hijack()
	if err != nil {
		return
	}
	defer client.Close()
	stop := context.AfterFunc(ctx, func() {
		_ = client.Close()
		_ = remote.Close()
	})
	defer stop()
	_ = client.SetDeadline(time.Time{})
	if _, err := buffer.WriteString("HTTP/1.1 200 Connection Established\r\n\r\n"); err != nil || buffer.Flush() != nil {
		return
	}
	inbound := make(chan struct{})
	go func() {
		_, _ = io.Copy(remote, buffer)
		_ = remote.(*net.TCPConn).CloseWrite()
		close(inbound)
	}()
	_, _ = io.Copy(client, remote)
	_ = client.Close()
	_ = remote.Close()
	<-inbound
}
