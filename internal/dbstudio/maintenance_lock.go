package dbstudio

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"strings"
	"time"

	"groxy-desk/internal/security"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

const maintenanceIOTimeout = 5 * time.Second

type maintenanceLock struct {
	session *ssh.Session
	client  *sftp.Client
	path    string
}

func writeFenceUnavailable(message string, err error) error {
	return security.NewError(security.CodeWriteFenceUnavailable, 409, message, err)
}

func acquireMaintenanceLock(ctx context.Context, scope networkScope) (*maintenanceLock, error) {
	session, err := scope.tunnel.sshClient.NewSession()
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, maintenanceIOTimeout)
	defer cancel()
	stop := context.AfterFunc(ctx, func() { _ = session.Close() })
	defer stop()
	lock, err := openMaintenanceLock(session, scope.engine, scope.database)
	if err != nil {
		_ = session.Close()
		return nil, err
	}
	return lock, nil
}

func openMaintenanceLock(session *ssh.Session, engine, database string) (*maintenanceLock, error) {
	stdin, err := session.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := session.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := session.RequestSubsystem("sftp"); err != nil {
		return nil, err
	}
	client, err := sftp.NewClientPipe(stdout, stdin)
	if err != nil {
		return nil, err
	}
	// All SSH sessions on the host must see the same /tmp. Exclude the SSH
	// user, local port and schema so aliases cannot create separate locks.
	if engine == "mariadb" {
		database = strings.ToLower(database)
	}
	path := "/tmp/groxy-desk-db-" + security.SHA256Hex([]byte(engine+":"+database)) + ".lock"
	file, err := client.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL)
	if err != nil {
		return nil, err
	}
	if err := file.Close(); err != nil {
		return nil, err
	}
	return &maintenanceLock{session: session, client: client, path: path}, nil
}

func (lock *maintenanceLock) close(remove bool) error {
	defer lock.session.Close()
	if !remove {
		return nil
	}
	// Cleanup is independent of the HTTP request. If the SSH session is lost,
	// the file remains on the host and recovery stays closed to other sessions.
	timeout := time.AfterFunc(maintenanceIOTimeout, func() { _ = lock.session.Close() })
	defer timeout.Stop()
	return lock.client.Remove(lock.path)
}

func (m *Manager) withNetworkRestoreScope(ctx context.Context, target TargetSpec, scope networkScope, apply bool, run func(context.Context, *sql.Tx) error) (err error) {
	if !apply {
		return m.withNetworkScope(ctx, target, scope, run)
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return withPinnedMaintenanceLock(ctx, scope, func(removeLock *bool) error {
		if scope.engine == "postgres" {
			return m.withPostgresRestoreFence(ctx, target, scope, removeLock, func(ctx context.Context, conn *sql.Conn) error {
				return withNetworkTableTransaction(ctx, conn, scope, run)
			})
		}
		return m.withNetworkScope(ctx, target, scope, run)
	})
}

func withPinnedMaintenanceLock(ctx context.Context, scope networkScope, run func(removeLock *bool) error) (err error) {
	lock, err := acquireMaintenanceLock(ctx, scope)
	if err != nil {
		return writeFenceUnavailable("Desk could not acquire the exclusive maintenance lock over pinned SSH.", err)
	}
	removeLock := true
	defer func() {
		if cleanupErr := lock.close(removeLock); cleanupErr != nil {
			err = security.NewError(security.CodeDeskE502, 502, "Desk could not remove the recovery lock. Inspect the operation before retrying.", errors.Join(err, cleanupErr))
		}
	}()
	return run(&removeLock)
}
