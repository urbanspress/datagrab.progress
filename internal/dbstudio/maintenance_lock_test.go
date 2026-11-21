package dbstudio

import (
	"context"
	"testing"

	"groxy-desk/internal/testfixture"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

func TestPinnedMaintenanceLock(t *testing.T) {
	endpoint := testfixture.PinnedSSHWithSFTP(t, 0, nil)
	verifyHost, err := knownhosts.New(endpoint.KnownHosts)
	if err != nil {
		t.Fatal(err)
	}
	dial := func(user string) *managedTunnel {
		t.Helper()
		client, err := ssh.Dial("tcp", endpoint.Address, &ssh.ClientConfig{User: user, HostKeyCallback: verifyHost})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = client.Close() })
		return &managedTunnel{sshClient: client}
	}
	first := networkScope{engine: "postgres", database: "fixture", schema: "public", tunnel: dial("first")}
	second := networkScope{engine: "postgres", database: "fixture", schema: "other", tunnel: dial("second")}
	ctx := context.Background()
	lock, err := acquireMaintenanceLock(ctx, first)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.close(false)
	if overlapping, err := acquireMaintenanceLock(ctx, second); err == nil {
		_ = overlapping.close(false)
		t.Fatal("two SSH sessions acquired a maintenance lock on the same database")
	}
	if err := lock.close(true); err != nil {
		t.Fatal(err)
	}
	lock, err = acquireMaintenanceLock(ctx, second)
	if err != nil {
		t.Fatalf("second session could not acquire the released lock: %v", err)
	}
	if err := lock.close(false); err != nil {
		t.Fatal(err)
	}
	if overlapping, err := acquireMaintenanceLock(ctx, first); err == nil {
		_ = overlapping.close(false)
		t.Fatal("closing the SSH session removed the retained lock")
	}
}
