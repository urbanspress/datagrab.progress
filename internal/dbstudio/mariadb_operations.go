package dbstudio

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"groxy-desk/internal/groxybridge"
	"groxy-desk/internal/security"
)

const mariadbFullPrefix = "snap_db_mdfull_"

const mariaOperationTimeout = 2 * time.Minute

// mariadbFullScope identifies a MariaDB target whose whole database Desk can
// snapshot, replay and verify. With audit set it also proves, before any
// approval is issued, that Desk can exclude every other writer.
func (m *Manager) mariadbFullScope(ctx context.Context, target TargetSpec, audit bool) (networkScope, error) {
	scope, err := m.networkScopeFor(target, "database")
	if err != nil {
		return scope, err
	}
	if scope.engine != "mariadb" || target.DBUser == "" || target.MaintenanceUser != "" {
		return scope, writeFenceUnavailable("Full MariaDB recovery needs one database-scoped administrator login and no separate maintenance login.", nil)
	}
	scope.label = scope.database + " via " + scope.tunnel.view.SSHHost
	scope.key += ":full"
	if !audit {
		return scope, nil
	}
	err = m.withMariaDBReader(ctx, target, scope, func(conn *sql.Conn) error {
		_, err := mariadbAdmission(ctx, conn, scope)
		return err
	})
	return scope, err
}

func (m *Manager) withMariaDBReader(ctx context.Context, target TargetSpec, scope networkScope, run func(*sql.Conn) error) error {
	db, _, _, err := m.openTargetOnTunnel(target, scope.tunnel)
	if err != nil {
		return err
	}
	defer db.Close()
	conn, err := db.Conn(ctx)
	if err != nil {
		return writeFenceUnavailable("Could not reserve a MariaDB connection.", err)
	}
	defer conn.Close()
	if err := mariaSessionSettings(ctx, conn, mariaSessionMode); err != nil {
		return writeFenceUnavailable("Could not prepare the MariaDB session.", err)
	}
	return run(conn)
}

func (m *Manager) withMariaDBImage(ctx context.Context, target TargetSpec, scope networkScope, apply bool, run func(*sql.Conn) error) error {
	if !apply {
		return m.withMariaDBReader(ctx, target, scope, run)
	}
	return withPinnedMaintenanceLock(ctx, scope, func(remove *bool) error {
		return m.withMariaDBFence(ctx, target, scope, remove, func(ctx context.Context, conn *sql.Conn) error { return run(conn) })
	})
}

func (m *Manager) planMariaDBSQL(ctx context.Context, target TargetSpec, operation string, statements []string) (map[string]any, error) {
	scope, err := m.mariadbFullScope(ctx, target, false)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, mariaOperationTimeout)
	defer cancel()
	var image []byte
	err = m.withMariaDBReader(ctx, target, scope, func(conn *sql.Conn) error {
		if _, err := mariadbAdmission(ctx, conn, scope); err != nil {
			return err
		}
		var err error
		if image, err = mariaLogicalImage(ctx, conn, scope); err != nil {
			return err
		}
		_, err = parseMariaImage(image, scope)
		return err
	})
	if err != nil {
		return nil, networkRecoveryError(err)
	}
	record, err := m.approvals.CreatePrefixedVirtualSnapshot(mariadbFullPrefix, scope.key, string(image))
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

func mariaDML(statement string) bool {
	switch strings.ToUpper(strings.Fields(statement)[0]) {
	case "INSERT", "UPDATE", "DELETE", "REPLACE", "WITH":
		return true
	}
	return false
}

func (m *Manager) applyMariaDBSQL(ctx context.Context, target TargetSpec, operation, approval string, statements []string) (map[string]any, error) {
	scope, err := m.mariadbFullScope(ctx, target, true)
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
	if _, err := parseMariaImage(saved, scope); err != nil {
		return nil, err
	}
	savedDigest := security.SHA256Hex(saved)
	ctx, cancel := context.WithTimeout(ctx, mariaOperationTimeout)
	defer cancel()
	var affected int64
	executed := 0
	var operationErr error
	err = m.withMariaDBImage(ctx, target, scope, true, func(conn *sql.Conn) error {
		// Prove the restore path on the unchanged baseline before any
		// approved statement runs. Replay verifies the image byte for byte.
		if err := replayMariaImage(ctx, conn, scope, savedDigest, saved, true); err != nil {
			return err
		}
		if err := mariaSessionSettings(ctx, conn, mariaUserMode); err != nil {
			return mariaReplayFailure{err}
		}
		transactional := true
		for _, statement := range statements {
			transactional = transactional && mariaDML(statement)
		}
		var tx *sql.Tx
		if transactional {
			var txErr error
			if tx, txErr = conn.BeginTx(ctx, nil); txErr != nil {
				return mariaReplayFailure{txErr}
			}
		}
		for _, statement := range statements {
			var result sql.Result
			if tx != nil {
				result, operationErr = tx.ExecContext(ctx, statement)
			} else {
				result, operationErr = conn.ExecContext(ctx, statement)
			}
			if operationErr != nil {
				break
			}
			executed++
			if n, err := result.RowsAffected(); err == nil {
				affected += n
			}
		}
		if tx != nil {
			if operationErr != nil {
				_ = tx.Rollback()
			} else if operationErr = tx.Commit(); operationErr != nil {
				executed = 0
			}
		}
		// Settle the outcome even if the request was canceled mid-statement.
		settle, stop := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
		defer stop()
		if err := mariaSessionSettings(settle, conn, mariaSessionMode); err != nil {
			return mariaReplayFailure{err}
		}
		after, err := mariaLogicalImage(settle, conn, scope)
		if err != nil {
			// A state Desk cannot capture cannot be rolled back later. Restore
			// the verified baseline now, discarding whatever was created.
			if operationErr == nil {
				operationErr = errors.New("the approved SQL produced a MariaDB database that Desk cannot verify or recover")
			}
			if replayErr := replayMariaImage(settle, conn, scope, savedDigest, saved, false); replayErr != nil {
				return mariaReplayFailure{errors.Join(operationErr, err, replayErr)}
			}
			executed = 0
			return m.approvals.WriteDBOutcome(token.SnapshotID, scope.key, savedDigest)
		}
		return m.approvals.WriteDBOutcome(token.SnapshotID, scope.key, security.SHA256Hex(after))
	})
	if err != nil {
		return nil, networkRecoveryError(err)
	}
	if operationErr != nil {
		digest, readErr := m.approvals.ReadDBOutcome(token.SnapshotID, scope.key)
		if readErr == nil && digest != savedDigest {
			return nil, security.NewError(security.CodeDeskE406, 422,
				fmt.Sprintf("MariaDB applied %d of %d approved statements before the next one failed and DDL cannot be rolled back automatically. "+
					"The snapshot %s is retained; review a rollback plan to restore the exact previous state.", executed, len(statements), token.SnapshotID), operationErr)
		}
		return nil, security.NewError(security.CodeDeskE406, 422,
			"MariaDB rejected the approved SQL and the database still matches its verified snapshot", operationErr)
	}
	return map[string]any{"ok": true, "engine": scope.engine, "snapshot_id": token.SnapshotID,
		"affected_rows": strconv.FormatInt(affected, 10), "statements_executed": strconv.Itoa(executed)}, nil
}

func (m *Manager) restoreMariaDBSQL(ctx context.Context, r RestoreRequest, apply bool) (map[string]any, error) {
	scope, err := m.mariadbFullScope(ctx, r.Target, !apply)
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
	ctx, cancel := context.WithTimeout(ctx, mariaOperationTimeout)
	defer cancel()
	if apply {
		if r.Force {
			baseline = r.BaselineSHA
		}
		if _, err := m.approvals.ConsumeToken(r.ApprovalToken, groxybridge.ActionDBRestore, binding(baseline)); err != nil {
			return nil, err
		}
		err = m.withMariaDBImage(ctx, r.Target, scope, true, func(conn *sql.Conn) error {
			return replayMariaImage(ctx, conn, scope, baseline, saved, true)
		})
		if err != nil {
			return nil, networkRecoveryError(err)
		}
		return m.finishNetworkRestore(r, scope, security.SHA256Hex(saved))
	}
	err = m.withMariaDBImage(ctx, r.Target, scope, false, func(conn *sql.Conn) error {
		current, err := mariaLogicalImage(ctx, conn, scope)
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
