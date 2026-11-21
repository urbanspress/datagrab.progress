// Package testfixture provides disposable loopback services for integration tests.
package testfixture

import (
	"crypto/ed25519"
	"crypto/rand"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

type SSHEndpoint struct {
	Address, Port, KnownHosts, Fingerprint string
}

// PinnedSSH serves direct TCP channels restricted to a single loopback port.
func PinnedSSH(t testing.TB, destinationPort uint32, respond func(ssh.Channel)) (SSHEndpoint, func(string)) {
	t.Helper()
	return pinnedSSH(t, destinationPort, respond, nil)
}

// PinnedSSHWithSFTP also serves a filesystem shared by every SSH session.
func PinnedSSHWithSFTP(t testing.TB, destinationPort uint32, respond func(ssh.Channel)) SSHEndpoint {
	t.Helper()
	handlers := sftp.InMemHandler()
	if err := handlers.FileCmd.Filecmd(&sftp.Request{Method: "Mkdir", Filepath: "/tmp"}); err != nil {
		t.Fatal(err)
	}
	endpoint, _ := pinnedSSH(t, destinationPort, respond, &handlers)
	return endpoint
}

func pinnedSSH(t testing.TB, destinationPort uint32, respond func(ssh.Channel), files *sftp.Handlers) (SSHEndpoint, func(string)) {
	t.Helper()
	_, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.NewSignerFromKey(private)
	if err != nil {
		t.Fatal(err)
	}
	server := &ssh.ServerConfig{NoClientAuth: true}
	server.AddHostKey(signer)
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	address := listener.Addr().String()
	_, port, err := net.SplitHostPort(address)
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		for {
			raw, err := listener.Accept()
			if err != nil {
				return
			}
			go servePinnedSSH(raw, server, destinationPort, respond, files)
		}
	}()

	known := filepath.Join(t.TempDir(), "known_hosts")
	writeKnown := func(address string) {
		t.Helper()
		if err := os.WriteFile(known, []byte(knownhosts.Line([]string{address}, signer.PublicKey())+"\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	writeKnown(address)
	return SSHEndpoint{Address: address, Port: port, KnownHosts: known, Fingerprint: ssh.FingerprintSHA256(signer.PublicKey())}, writeKnown
}

func servePinnedSSH(raw net.Conn, server *ssh.ServerConfig, port uint32, respond func(ssh.Channel), files *sftp.Handlers) {
	defer raw.Close()
	_ = raw.SetDeadline(time.Now().Add(10 * time.Second))
	conn, channels, requests, err := ssh.NewServerConn(raw, server)
	if err != nil {
		return
	}
	defer conn.Close()
	_ = raw.SetDeadline(time.Time{})
	go ssh.DiscardRequests(requests)
	for channel := range channels {
		if channel.ChannelType() == "session" && files != nil {
			go serveSFTP(channel, *files)
			continue
		}
		var attrs struct {
			Host       string
			Port       uint32
			Origin     string
			OriginPort uint32
		}
		if channel.ChannelType() != "direct-tcpip" || ssh.Unmarshal(channel.ExtraData(), &attrs) != nil ||
			attrs.Host != "127.0.0.1" || attrs.Port != port {
			_ = channel.Reject(ssh.Prohibited, "unexpected channel or destination")
			continue
		}
		tunnel, requests, err := channel.Accept()
		if err != nil {
			return
		}
		go ssh.DiscardRequests(requests)
		go func() {
			defer tunnel.Close()
			respond(tunnel)
		}()
	}
}

func serveSFTP(channel ssh.NewChannel, files sftp.Handlers) {
	stream, requests, err := channel.Accept()
	if err != nil {
		return
	}
	defer stream.Close()
	for request := range requests {
		var subsystem struct{ Name string }
		if request.Type != "subsystem" || ssh.Unmarshal(request.Payload, &subsystem) != nil || subsystem.Name != "sftp" {
			_ = request.Reply(false, nil)
			continue
		}
		if err := request.Reply(true, nil); err != nil {
			return
		}
		go ssh.DiscardRequests(requests)
		server := sftp.NewRequestServer(stream, files)
		defer server.Close()
		_ = server.Serve()
		return
	}
}

func ForwardLoopback(port uint32) func(ssh.Channel) {
	return func(ch ssh.Channel) {
		destination := net.JoinHostPort("127.0.0.1", strconv.FormatUint(uint64(port), 10))
		conn, err := net.DialTimeout("tcp4", destination, 3*time.Second)
		if err != nil {
			return
		}
		defer conn.Close()
		copied := make(chan struct{})
		go func() {
			_, _ = io.Copy(conn, ch)
			_ = conn.(*net.TCPConn).CloseWrite()
			close(copied)
		}()
		_, _ = io.Copy(ch, conn)
		_ = ch.Close()
		<-copied
	}
}
