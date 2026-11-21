package dbstudio

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"groxy-desk/internal/security"

	"github.com/jackc/pgx/v5/pgconn"
)

func (m *Manager) withPostgresRestoreFence(ctx context.Context, target TargetSpec, scope networkScope, removeLock *bool, run func(context.Context, *sql.Conn) error) (err error) {
	if scope.database == "postgres" {
		return writeFenceUnavailable("Choose a target separate from the PostgreSQL maintenance database.", nil)
	}
	maintenance := target
	maintenance.DBName = "postgres"
	if target.MaintenanceUser != "" {
		maintenance.DBUser, maintenance.DBPassword = target.MaintenanceUser, target.MaintenancePassword
	}
	admin, _, _, err := m.openTargetOnTunnel(maintenance, scope.tunnel)
	if err != nil {
		return writeFenceUnavailable("Could not open the PostgreSQL maintenance database.", err)
	}
	defer admin.Close()

	var oid int64
	var eligible bool
	err = admin.QueryRowContext(ctx, `SELECT d.oid::bigint,
		d.datallowconn AND NOT d.datistemplate AND NOT pg_catalog.pg_is_in_recovery()
		AND (SELECT rolsuper FROM pg_catalog.pg_roles WHERE rolname = current_user)
		AND pg_catalog.current_setting('shared_preload_libraries') = ''
		AND NOT EXISTS (SELECT FROM pg_catalog.pg_subscription WHERE subdbid = d.oid AND subenabled)
		AND NOT EXISTS (SELECT FROM pg_catalog.pg_prepared_xacts WHERE database = d.datname)
		FROM pg_catalog.pg_database d WHERE d.datname::text = $1`, scope.database).Scan(&oid, &eligible)
	if err != nil || !eligible {
		return writeFenceUnavailable("PostgreSQL requires a maintenance role able to evict all writers and a target without prepared transactions or background writers.", err)
	}

	db, _, _, err := m.openTargetOnTunnel(target, scope.tunnel)
	if err != nil {
		return writeFenceUnavailable("Could not open the PostgreSQL recovery connection.", err)
	}
	defer db.Close()
	conn, err := db.Conn(ctx)
	if err != nil {
		return writeFenceUnavailable("Could not reserve the PostgreSQL recovery connection.", err)
	}
	defer conn.Close()
	var pid int
	var connectedOID int64
	err = conn.QueryRowContext(ctx, `SELECT pg_catalog.pg_backend_pid(), oid::bigint
		FROM pg_catalog.pg_database WHERE datname = pg_catalog.current_database()`).Scan(&pid, &connectedOID)
	if err != nil || connectedOID != oid {
		return writeFenceUnavailable("Could not confirm the PostgreSQL recovery database identity.", err)
	}

	alter := "ALTER DATABASE " + quoteIdent("postgres", scope.database) + " WITH ALLOW_CONNECTIONS "
	// Cleanup is registered before submitting ALTER. A broken connection can
	// lose the reply to an ALTER that PostgreSQL has already committed.
	var pgErr *pgconn.PgError
	mayBeClosed := true
	defer func() {
		if !mayBeClosed {
			return
		}
		_ = conn.Close()
		_ = db.Close()
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), maintenanceIOTimeout)
		defer cancel()
		if reopenErr := reopenPostgresDatabase(cleanup, admin, alter, oid, pid); reopenErr != nil {
			*removeLock = false
			err = security.NewError(security.CodeDeskE502, 502, "Desk could not reopen PostgreSQL connections and retained the maintenance lock. Inspect the target before retrying.", errors.Join(err, reopenErr))
		}
	}()
	if _, err := admin.ExecContext(ctx, alter+"false"); err != nil {
		if errors.As(err, &pgErr) && pgErr.SeverityUnlocalized == "ERROR" {
			mayBeClosed = false
		}
		return writeFenceUnavailable("PostgreSQL could not exclude new connections to the recovery database.", err)
	}
	if err := drainPostgresBackends(ctx, admin, oid, pid, scope.database); err != nil {
		return writeFenceUnavailable("PostgreSQL could not exclude all other writers from the recovery database.", err)
	}
	err = run(ctx, conn)
	if errors.As(err, &pgErr) && pgErr.Code == "42501" {
		return writeFenceUnavailable("The PostgreSQL recovery role lacks required permissions.", err)
	}
	return err
}

func reopenPostgresDatabase(ctx context.Context, admin *sql.DB, alter string, oid int64, deskPID int) error {
	// Finish tearing down the recovery backend before reopening the database,
	// including when a canceled request lost the transaction's final reply.
	var disconnected bool
	err := admin.QueryRowContext(ctx, `SELECT COALESCE((SELECT pg_catalog.pg_terminate_backend(pid, 2000)
		FROM pg_catalog.pg_stat_activity WHERE datid = $1::bigint::oid AND pid = $2), true)`, oid, deskPID).Scan(&disconnected)
	if err != nil {
		return err
	}
	if !disconnected {
		var stillConnected bool
		if err := admin.QueryRowContext(ctx, `SELECT EXISTS (SELECT FROM pg_catalog.pg_stat_activity
			WHERE datid = $1::bigint::oid AND pid = $2)`, oid, deskPID).Scan(&stillConnected); err != nil {
			return err
		}
		if stillConnected {
			return errors.New("PostgreSQL recovery backend is still connected")
		}
	}
	_, err = admin.ExecContext(ctx, alter+"true")
	return err
}

func drainPostgresBackends(ctx context.Context, admin *sql.DB, oid int64, deskPID int, database string) error {
	// PostgreSQL holds this database object lock during connection startup.
	// Wait for any startup that passed the admission check before ALTER. It
	// must publish its backend status before releasing the startup lock.
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	for {
		var starting bool
		err := admin.QueryRowContext(ctx, `SELECT EXISTS (SELECT FROM pg_catalog.pg_locks
			WHERE locktype = 'object' AND classid = 'pg_catalog.pg_database'::pg_catalog.regclass
			AND objid = $1::bigint::oid AND mode = 'RowExclusiveLock' AND granted)`, oid).Scan(&starting)
		if err != nil {
			return err
		}
		if !starting {
			break
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
	// Termination with a positive timeout waits for the backend to exit and
	// roll back its transaction. Recheck so a timed-out signal cannot pass.
	if _, err := admin.ExecContext(ctx, `SELECT pg_catalog.pg_terminate_backend(pid, 2000)
		FROM pg_catalog.pg_stat_activity WHERE datid = $1::bigint::oid AND pid <> $2`, oid, deskPID); err != nil {
		return err
	}
	var exclusive bool
	err := admin.QueryRowContext(ctx, `SELECT EXISTS (SELECT FROM pg_catalog.pg_database
		WHERE oid = $1::bigint::oid AND datname::text = $3 AND NOT datallowconn)
		AND NOT EXISTS (SELECT FROM pg_catalog.pg_stat_activity WHERE datid = $1::bigint::oid AND pid <> $2)
		AND NOT EXISTS (SELECT FROM pg_catalog.pg_subscription WHERE subdbid = $1::bigint::oid AND subenabled)
		AND NOT EXISTS (SELECT FROM pg_catalog.pg_prepared_xacts WHERE database::text = $3)`, oid, deskPID, database).Scan(&exclusive)
	if err != nil {
		return err
	}
	if !exclusive {
		return errors.New("database admission, backend or prepared transaction check failed")
	}
	return nil
}
