package dbstudio

import (
	"crypto/ed25519"
	"crypto/rand"
	"io"
	"net"
	"strconv"
	"testing"
	"time"

	"groxy-desk/internal/security"
	"groxy-desk/internal/testfixture"

	"golang.org/x/crypto/ssh"
)

func pinnedSSHFixture(t *testing.T, destinationPort uint32, respond func(ssh.Channel)) (OpenTunnelRequest, func(string)) {
	t.Helper()
	endpoint, writeKnown := testfixture.PinnedSSH(t, destinationPort, respond)
	request := OpenTunnelRequest{
		Engine: "postgres", SSHHost: "127.0.0.1", SSHPort: endpoint.Port, KnownHostsPath: endpoint.KnownHosts,
		PinnedFingerprint: endpoint.Fingerprint, RemoteDBPort: strconv.FormatUint(uint64(destinationPort), 10),
	}
	return request, writeKnown
}

func TestDatabaseTunnelChecksKnownHostAndPinInHandshake(t *testing.T) {
	request, writeKnown := pinnedSSHFixture(t, 5432, func(tunnel ssh.Channel) {
		_, _ = tunnel.Write([]byte("ready"))
	})
	writeKnown(net.JoinHostPort("127.0.0.1", "22"))
	tm := NewTunnelManager(nil)
	defer tm.CloseAll()
	if _, err := tm.OpenTunnel(request); security.ErrorCode(err, "") != security.CodeDeskE405 {
		t.Fatalf("SSH port mismatch returned %v", err)
	}
	writeKnown(net.JoinHostPort(request.SSHHost, request.SSHPort))
	wrong := request
	_, other, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	otherKey, err := ssh.NewPublicKey(other.Public())
	if err != nil {
		t.Fatal(err)
	}
	wrong.PinnedFingerprint = ssh.FingerprintSHA256(otherKey)
	if _, err := tm.OpenTunnel(wrong); security.ErrorCode(err, "") != security.CodeDeskE405 {
		t.Fatalf("SSH fingerprint mismatch returned %v", err)
	}
	view, err := tm.OpenTunnel(request)
	if err != nil {
		t.Fatal(err)
	}
	conn, err := net.DialTimeout("tcp4", view.LocalBindAddr, 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
	var received [5]byte
	n, err := io.ReadFull(conn, received[:])
	if err != nil || string(received[:n]) != "ready" {
		t.Fatalf("SSH forward read %q, %v", received[:n], err)
	}
}
