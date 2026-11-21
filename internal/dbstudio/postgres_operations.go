package dbstudio

import (
	"context"
	"database/sql"
	"errors"
	"os/exec"
	"strconv"
	"time"

	"groxy-desk/internal/groxybridge"
	"groxy-desk/internal/security"

	"github.com/jackc/pgx/v5/stdlib"
)

const postgresFullPrefix = "snap_db_pgfull_"

func (m *Manager) postgresFullScope(target TargetSpec) (networkScope, error) {
	scope, err := m.networkScopeFor(target, "database")
	if err != nil {
		return scope, err
	}
	if !postgresReplayCandidate {
		return scope, writeFenceUnavailable("Full database replay is pending Section 9 acceptance.", nil)
	}
	if scope.engine != "postgres" || scope.database == "postgres" || target.MaintenanceUser == "" ||
		target.DBUser == "" || target.DBUser == target.MaintenanceUser {
		return scope, writeFenceUnavailable("Full logical recovery needs a separate PostgreSQL maintenance login and a database-scoped writer.", nil)
	}
	if _, err := exec.LookPath("pg_dump"); err != nil {
		return scope, writeFenceUnavailable("Install the PostgreSQL 18 pg_dump client for full logical recovery.", err)
	}
	scope.label = scope.database + " via " + scope.tunnel.view.SSHHost
	scope.key += ":full:" + security.SHA256Hex([]byte(target.MaintenanceUser))
	return scope, nil
}

// The privileged login only controls admission from the maintenance database.
// User statements and native dump replay must run on a separate backend
// authenticated as a principal that cannot reopen the target or change roles.
func checkPostgresWriter(ctx context.Context, c *sql.Conn) error {
	var eligible bool
	err := c.QueryRowContext(ctx, `SELECT NOT (r.rolsuper OR r.rolcreaterole OR r.rolcreatedb OR r.rolreplication OR r.rolbypassrls)
		AND r.oid <> d.datdba AND NOT EXISTS (SELECT FROM pg_catalog.pg_auth_members WHERE member = r.oid)
		AND current_setting('server_version_num')::integer BETWEEN 180000 AND 189999
		AND current_setting('server_encoding') = 'UTF8'
		AND NOT EXISTS (SELECT FROM pg_catalog.pg_extension WHERE extname <> 'plpgsql')
		AND NOT EXISTS (SELECT FROM pg_catalog.pg_foreign_server)
		AND NOT EXISTS (SELECT FROM pg_catalog.pg_event_trigger)
		AND NOT EXISTS (SELECT FROM pg_catalog.pg_publication)
		AND NOT EXISTS (SELECT FROM pg_catalog.pg_proc p JOIN pg_catalog.pg_namespace n ON n.oid=p.pronamespace
			WHERE p.prosecdef AND p.proowner <> r.oid AND n.nspname NOT IN ('pg_catalog','information_schema'))
		FROM pg_catalog.pg_roles r, pg_catalog.pg_database d
		WHERE r.rolname = session_user AND d.datname = current_database()`).Scan(&eligible)
	if err != nil || !eligible {
		return writeFenceUnavailable("Desk could not verify a database-scoped PostgreSQL writer and self-contained logical snapshot.", err)
	}
	return nil
}

func postgresTransaction(ctx context.Context, conn *sql.Conn, readOnly bool, run func() error) error {
	tx, err := conn.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: readOnly})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, "SET LOCAL search_path = pg_catalog"); err != nil {
		return err
	}
	if err := checkPostgresWriter(ctx, conn); err != nil {
		return err
	}
	if err := run(); err != nil {
		return err
	}
	return tx.Commit()
}

func (m *Manager) withPostgresImage(ctx context.Context, target TargetSpec, scope networkScope, apply bool, run func(*sql.Conn) error) error {
	if apply {
		return withPinnedMaintenanceLock(ctx, scope, func(remove *bool) error {
			return m.withPostgresRestoreFence(ctx, target, scope, remove, func(ctx context.Context, conn *sql.Conn) error {
				return postgresTransaction(ctx, conn, false, func() error { return run(conn) })
			})
		})
	}
	db, _, _, err := m.openTargetOnTunnel(target, scope.tunnel)
	if err != nil {
		return err
	}
	defer db.Close()
	conn, err := db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	return postgresTransaction(ctx, conn, true, func() error { return run(conn) })
}

func (m *Manager) planPostgresSQL(ctx context.Context, target TargetSpec, operation string, statements []string) (map[string]any, error) {
	scope, err := m.postgresFullScope(target)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	var image []byte
	err = m.withPostgresImage(ctx, target, scope, false, func(c *sql.Conn) error {
		var err error
		image, err = postgresLogicalImage(ctx, c, scope.database, false)
		return err
	})
	if err != nil {
		return nil, networkRecoveryError(err)
	}
	record, err := m.approvals.CreatePrefixedVirtualSnapshot(postgresFullPrefix, scope.key, string(image))
	if err != nil {
		return nil, err
	}
	if _, err := m.approvals.ReadDBSnapshot(record.SnapshotID, scope.key); err != nil {
		return nil, err
	}
	token, err := m.approvals.IssueSnapshotBoundToken(groxybridge.ActionDBMutate, mutationBinding(scope.key, operation), record.SnapshotID)
	if err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "engine": scope.engine, "target_key": scope.key, "target_label": scope.label,
		"snapshot": record, "approval_token": token.Token, "statement_count": strconv.Itoa(len(statements)),
		"plain_summary": scope.restoreSummary("Apply the reviewed SQL after verifying full logical recovery on " + scope.label + ".")}, nil
}

func (m *Manager) applyPostgresSQL(ctx context.Context, target TargetSpec, operation, approval string, statements []string) (map[string]any, error) {
	scope, err := m.postgresFullScope(target)
	if err != nil {
		return nil, err
	}
	token, err := m.approvals.ConsumeToken(approval, groxybridge.ActionDBMutate, mutationBinding(scope.key, operation))
	if err != nil {
		return nil, err
	}
	saved, err := m.approvals.ReadDBSnapshot(token.SnapshotID, scope.key)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	var affected int64
	var operationErr error
	err = m.withPostgresImage(ctx, target, scope, true, func(c *sql.Conn) error {
		// Rehearse against the actual database, then roll back the rehearsal
		// so existing object identities survive a successful user operation.
		if _, err := c.ExecContext(ctx, "SAVEPOINT desk_rehearsal"); err != nil {
			return err
		}
		if err := replayPostgresImage(ctx, c, scope.database, security.SHA256Hex(saved), saved); err != nil {
			return err
		}
		if _, err := c.ExecContext(ctx, "ROLLBACK TO SAVEPOINT desk_rehearsal"); err != nil {
			return err
		}
		if _, err := c.ExecContext(ctx, "SELECT pg_catalog.set_config('search_path',pg_catalog.quote_ident($1),true)", scope.schema); err != nil {
			return err
		}
		for _, statement := range statements {
			err := c.Raw(func(driver any) error {
				// Extended protocol enforces one statement per Parse at the
				// server, independently of Desk's review-time SQL splitter.
				tag, err := driver.(*stdlib.Conn).Conn().PgConn().ExecParams(ctx, statement, nil, nil, nil, nil).Close()
				affected += tag.RowsAffected()
				return err
			})
			if err != nil {
				operationErr = err
				break
			}
		}
		// Deferred triggers can change data at commit. Fire them before
		// recording the digest or deciding whether the operation succeeded.
		if operationErr == nil {
			_, operationErr = c.ExecContext(ctx, "SET CONSTRAINTS ALL IMMEDIATE")
		}
		if operationErr != nil {
			if _, err := c.ExecContext(ctx, "ROLLBACK TO SAVEPOINT desk_rehearsal"); err != nil {
				return errors.Join(operationErr, err)
			}
			// nextval/setval on an existing sequence survive SQL rollback.
			// Recover those changes before releasing database admission.
			current, err := postgresLogicalImage(ctx, c, scope.database, false)
			if err != nil {
				return errors.Join(operationErr, err)
			}
			if digest := security.SHA256Hex(current); digest != security.SHA256Hex(saved) {
				if err := replayPostgresImage(ctx, c, scope.database, digest, saved); err != nil {
					return errors.Join(operationErr, err)
				}
			}
			return m.approvals.WriteDBOutcome(token.SnapshotID, scope.key, security.SHA256Hex(saved))
		}
		if _, err := c.ExecContext(ctx, "SET LOCAL search_path = pg_catalog"); err != nil {
			return err
		}
		if err := checkPostgresWriter(ctx, c); err != nil {
			return err
		}
		after, err := postgresLogicalImage(ctx, c, scope.database, false)
		if err != nil {
			return err
		}
		return m.approvals.WriteDBOutcome(token.SnapshotID, scope.key, security.SHA256Hex(after))
	})
	if err != nil {
		return nil, networkRecoveryError(err)
	}
	if operationErr != nil {
		return nil, networkRecoveryError(operationErr)
	}
	return map[string]any{"ok": true, "engine": scope.engine, "snapshot_id": token.SnapshotID,
		"affected_rows": strconv.FormatInt(affected, 10), "statements_executed": strconv.Itoa(len(statements))}, nil
}

func (m *Manager) restorePostgresSQL(ctx context.Context, r RestoreRequest, apply bool) (map[string]any, error) {
	scope, err := m.postgresFullScope(r.Target)
	if err != nil {
		return nil, err
	}
	saved, err := m.approvals.ReadDBSnapshot(r.SnapshotID, scope.key)
	if err != nil {
		return nil, err
	}
	baseline, err := m.approvals.ReadDBOutcome(r.SnapshotID, scope.key)
	if err != nil {
		return nil, err
	}
	if !r.Force && r.BaselineSHA != "" {
		return nil, invalidCell("a reviewed baseline is only used for forced recovery")
	}
	binding := func(digest string) string {
		if r.Force {
			digest = "force:" + digest
		}
		return restoreBinding(scope.key, r.SnapshotID, digest)
	}
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	if apply {
		if r.Force {
			baseline = r.BaselineSHA
		}
		if _, err := m.approvals.ConsumeToken(r.ApprovalToken, groxybridge.ActionDBRestore, binding(baseline)); err != nil {
			return nil, err
		}
		err = m.withPostgresImage(ctx, r.Target, scope, true, func(c *sql.Conn) error {
			return replayPostgresImage(ctx, c, scope.database, baseline, saved)
		})
		if err != nil {
			return nil, networkRecoveryError(err)
		}
		return m.finishNetworkRestore(r, scope, security.SHA256Hex(saved))
	}
	err = m.withPostgresImage(ctx, r.Target, scope, false, func(c *sql.Conn) error {
		current, err := postgresLogicalImage(ctx, c, scope.database, false)
		if err != nil {
			return err
		}
		if r.Force {
			baseline = security.SHA256Hex(current)
		} else if security.SHA256Hex(current) != baseline {
			return changedNetworkTarget()
		}
		return nil
	})
	if err != nil {
		return nil, networkRecoveryError(err)
	}
	token, err := m.approvals.IssueSnapshotBoundToken(groxybridge.ActionDBRestore, binding(baseline), r.SnapshotID)
	if err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "target_label": scope.label, "snapshot_id": r.SnapshotID, "approval_token": token.Token,
		"baseline_sha256": baseline, "force": r.Force, "plain_summary": scope.restoreSummary("Replace the database contents with the reviewed full logical snapshot.")}, nil
}
