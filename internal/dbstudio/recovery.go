package dbstudio

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"groxy-desk/internal/groxybridge"
	"groxy-desk/internal/security"

	"modernc.org/sqlite"
)

const maxSQLiteBackup = 32 << 20

type dbSnapshot struct {
	Image []byte `json:"sqlite_image"`
}

type RestoreRequest struct {
	Target        TargetSpec `json:"target"`
	SnapshotID    string     `json:"snapshot_id"`
	ApprovalToken string     `json:"approval_token,omitempty"`
	Force         bool       `json:"force,omitempty"`
	BaselineSHA   string     `json:"baseline_sha256,omitempty"`
}

func sqliteURI(path, mode string) string {
	path = filepath.ToSlash(path)
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	u := url.URL{Scheme: "file", Path: path}
	return u.String() + "?mode=" + mode + "&_pragma=busy_timeout(5000)&_pragma=trusted_schema(OFF)"
}

func (m *Manager) sqliteTarget(target TargetSpec) (string, string, error) {
	if normalizeEngine(target.Engine) != "sqlite" {
		return "", "", security.NewError(security.CodeDeskE406, 403, "network database writes require a verified recoverable backup and are disabled", nil)
	}
	if target.DirectHost != "" {
		return "", "", security.NewError(security.CodeDeskE501, 422, "SQLite targets must be confined workspace files", nil)
	}
	abs, _, err := security.ConfineWorkspacePath(m.workspaceRoot, target.SQLitePath)
	if err != nil {
		return "", "", err
	}
	st, err := os.Stat(abs)
	if err != nil || !st.Mode().IsRegular() {
		return "", "", security.NewError(security.CodeDeskE101, 404, "SQLite workspace file is unavailable", err)
	}
	keyPath := abs
	if filepath.Separator == '\\' {
		keyPath = strings.ToLower(abs)
	}
	return abs, "db:sqlite:" + security.SHA256Hex([]byte(keyPath)), nil
}

func sqliteImage(ctx context.Context, conn *sql.Conn) ([]byte, error) {
	var pages, pageSize int64
	if err := conn.QueryRowContext(ctx, "PRAGMA main.page_count").Scan(&pages); err != nil {
		return nil, err
	}
	if err := conn.QueryRowContext(ctx, "PRAGMA main.page_size").Scan(&pageSize); err != nil {
		return nil, err
	}
	if pages == 0 || pageSize <= 0 || pages > maxSQLiteBackup/pageSize {
		return nil, fmt.Errorf("database exceeds the SQLite recovery limit or has no schema pages")
	}
	var image []byte
	err := conn.Raw(func(driver any) error {
		var err error
		image, err = driver.(interface{ Serialize() ([]byte, error) }).Serialize()
		return err
	})
	if err != nil {
		return nil, err
	}
	if len(image) < 100 || len(image) > maxSQLiteBackup || !bytes.HasPrefix(image, []byte("SQLite format 3\x00")) {
		return nil, fmt.Errorf("invalid SQLite backup image")
	}
	// Serialization includes pages read from the active WAL. Mark the standalone
	// copy as a rollback-journal image so its restore never depends on a WAL file.
	image[18], image[19] = 1, 1
	return image, nil
}

func sqliteImageDigest(image []byte) string {
	// Ignore the transaction change counter and version-valid-for copy. Those
	// header fields can change when SQLite commits or checkpoints identical pages.
	normalized := bytes.Clone(image)
	clear(normalized[24:28])
	clear(normalized[92:100])
	return security.SHA256Hex(normalized)
}

func (m *Manager) PlanMutation(ctx context.Context, req MutatePlanRequest) (map[string]any, error) {
	resolvedTarget, err := m.resolveTargetSecrets(req.Target)
	if err != nil {
		return nil, err
	}
	req.Target = resolvedTarget
	if normalizeEngine(req.Target.Engine) == "postgres" && req.Edit == nil && req.Target.MaintenanceUser != "" {
		statements, err := postgresWriteStatements(req.SQL)
		if err != nil {
			return nil, err
		}
		return m.planPostgresSQL(ctx, req.Target, "sql:"+req.SQL, statements)
	}
	if normalizeEngine(req.Target.Engine) == "mariadb" && req.Edit == nil {
		statements, err := mariadbWriteStatements(req.SQL)
		if err != nil {
			return nil, err
		}
		return m.planMariaDBSQL(ctx, req.Target, "sql:"+req.SQL, statements)
	}
	stmts, operation, err := mutationInput(req.SQL, req.Edit)
	if err != nil {
		return nil, err
	}
	if normalizeEngine(req.Target.Engine) != "sqlite" {
		return m.planNetworkCell(ctx, req, operation)
	}
	path, key, err := m.sqliteTarget(req.Target)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	db, err := sql.Open("sqlite", sqliteURI(path, "ro"))
	if err != nil {
		return nil, err
	}
	defer db.Close()
	conn, err := db.Conn(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, "BEGIN"); err != nil {
		return nil, err
	}
	if req.Edit != nil {
		if _, _, err := prepareCell(ctx, conn, *req.Edit); err != nil {
			return nil, err
		}
	}
	image, err := sqliteImage(ctx, conn)
	if err != nil {
		return nil, recoveryError(err)
	}
	if _, err := conn.ExecContext(ctx, "ROLLBACK"); err != nil {
		return nil, recoveryError(err)
	}

	backup, err := saveSQLiteScratch(ctx, image)
	if err != nil {
		return nil, recoveryError(err)
	}
	defer os.Remove(backup)
	if err := verifySQLiteRestore(ctx, backup); err != nil {
		return nil, recoveryError(err)
	}
	payload, err := json.Marshal(dbSnapshot{Image: image})
	if err != nil {
		return nil, err
	}
	snapshot, err := m.approvals.CreatePrefixedVirtualSnapshot("snap_db_", key, string(payload))
	if err != nil {
		return nil, err
	}
	if _, err := m.readDBSnapshot(snapshot.SnapshotID, key); err != nil {
		return nil, err
	}
	approval, err := m.approvals.IssueSnapshotBoundToken(groxybridge.ActionDBMutate, mutationBinding(key, operation), snapshot.SnapshotID)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"ok": true, "engine": "sqlite", "target_label": req.Target.SQLitePath, "target_key": key,
		"statement_count": strconv.Itoa(len(stmts)), "snapshot": snapshot, "approval_token": approval.Token,
		"plain_summary": fmt.Sprintf("Verified recovery of SQLite data and schema before %d statement(s) on %s.", len(stmts), req.Target.SQLitePath),
	}, nil
}

func mutationBinding(key, sql string) string {
	return key + ":" + security.SHA256Hex([]byte(sql))
}

func mutationInput(raw string, edit *CellEdit) ([]string, string, error) {
	if edit == nil {
		stmts, err := validatedMutation(raw)
		return stmts, "sql:" + raw, err
	}
	if raw != "" {
		return nil, "", invalidCell("submit SQL or a cell edit separately")
	}
	if _, err := editValue(edit.Value, edit.ValueKind); err != nil {
		return nil, "", err
	}
	if _, err := editValue(edit.Before, ""); err != nil {
		return nil, "", err
	}
	data, err := json.Marshal(edit)
	if err != nil {
		return nil, "", invalidCell("invalid inline cell edit")
	}
	return []string{""}, "cell:" + string(data), nil
}

func validatedMutation(raw string) ([]string, error) {
	stmts, err := splitSQLStatements(raw)
	if err != nil || len(stmts) == 0 {
		return nil, security.NewError(security.CodeDeskE101, 400, "valid SQL statement(s) required for mutation", err)
	}
	for _, st := range stmts {
		switch strings.ToUpper(strings.Fields(st)[0]) {
		case "INSERT", "UPDATE", "DELETE", "REPLACE", "CREATE", "ALTER", "DROP", "WITH":
		default:
			return nil, security.NewError(security.CodeDeskE406, 403, "statement is outside the recoverable SQLite write scope", nil)
		}
	}
	return stmts, nil
}

func (m *Manager) readDBSnapshot(id, target string) ([]byte, error) {
	raw, err := m.approvals.ReadDBSnapshot(id, target)
	if err != nil {
		return nil, err
	}
	var backup dbSnapshot
	if err := json.Unmarshal(raw, &backup); err != nil || len(backup.Image) < 100 || len(backup.Image) > maxSQLiteBackup {
		return nil, recoveryError(err)
	}
	return backup.Image, nil
}

func recoveryError(err error) error {
	return security.NewError(security.CodeDeskE406, 422, "SQLite recoverability verification failed", err)
}

func saveSQLiteScratch(ctx context.Context, image []byte) (string, error) {
	f, err := os.CreateTemp("", "groxy-sqlite-recovery-*.db")
	if err != nil {
		return "", err
	}
	path := f.Name()
	success := false
	defer func() {
		_ = f.Close()
		if !success {
			_ = os.Remove(path)
		}
	}()
	if _, err := f.Write(image); err != nil {
		return "", err
	}
	if err := f.Close(); err != nil {
		return "", err
	}
	db, err := sql.Open("sqlite", sqliteURI(path, "ro"))
	if err != nil {
		return "", err
	}
	defer db.Close()
	conn, err := db.Conn(ctx)
	if err != nil {
		return "", err
	}
	defer conn.Close()
	if err := sqliteIntegrity(ctx, conn, "main"); err != nil {
		return "", err
	}
	restored, err := sqliteImage(ctx, conn)
	if err != nil {
		return "", err
	}
	if sqliteImageDigest(restored) != sqliteImageDigest(image) {
		return "", fmt.Errorf("restored SQLite image differs from verified backup")
	}
	if _, err := sqliteObjects(ctx, conn, "main"); err != nil {
		return "", err
	}
	success = true
	return path, nil
}

func sqliteIntegrity(ctx context.Context, conn *sql.Conn, schema string) error {
	var check string
	if err := conn.QueryRowContext(ctx, "PRAGMA "+schema+".integrity_check").Scan(&check); err != nil {
		return err
	}
	if check != "ok" {
		return fmt.Errorf("SQLite integrity check failed")
	}
	return nil
}

func withSQLiteWrite(ctx context.Context, path string, foreignKeys bool, run func(context.Context, *sql.Conn) error) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	db, err := sql.Open("sqlite", sqliteURI(path, "rw"))
	if err != nil {
		return err
	}
	defer db.Close()
	conn, err := db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	if foreignKeys {
		if _, err := conn.ExecContext(ctx, "PRAGMA foreign_keys=ON"); err != nil {
			return err
		}
	}
	if _, err := conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		return security.NewError(security.CodeDeskE409, 409, "could not lock the approved SQLite target", err)
	}
	defer conn.ExecContext(context.Background(), "ROLLBACK")
	if err := run(ctx, conn); err != nil {
		return err
	}
	if _, err := conn.ExecContext(ctx, "COMMIT"); err != nil {
		return security.NewError(security.CodeDeskE502, 502, "SQLite commit failed; inspect the operation record before retrying", err)
	}
	return nil
}

func sqliteUserSQL(conn *sql.Conn, authorize bool) error {
	var policy sqlite.AuthorizerFn
	if authorize {
		policy = func(action sqlite.AuthorizerActionCode, arg1, arg2, schema, _ string) sqlite.AuthorizerReturnCode {
			switch action {
			case sqlite.AuthAttach, sqlite.AuthDetach, sqlite.AuthPragma, sqlite.AuthTransaction, sqlite.AuthSavepoint,
				sqlite.AuthCreateVTable, sqlite.AuthDropVTable, sqlite.AuthCreateTempTable, sqlite.AuthCreateTempIndex,
				sqlite.AuthCreateTempTrigger, sqlite.AuthCreateTempView:
				return sqlite.AuthorizerDeny
			case sqlite.AuthFunction:
				if strings.EqualFold(arg2, "load_extension") || strings.EqualFold(arg2, "writefile") {
					return sqlite.AuthorizerDeny
				}
			}
			return sqlite.AuthorizerOK
		}
	}
	return conn.Raw(func(driver any) error {
		return driver.(sqlite.AuthorizerRegisterer).RegisterAuthorizer(policy)
	})
}

func (m *Manager) ApplyMutation(ctx context.Context, req MutateApplyRequest) (map[string]any, error) {
	resolvedTarget, err := m.resolveTargetSecrets(req.Target)
	if err != nil {
		return nil, err
	}
	req.Target = resolvedTarget
	if normalizeEngine(req.Target.Engine) == "postgres" && req.Edit == nil && req.Target.MaintenanceUser != "" {
		statements, err := postgresWriteStatements(req.SQL)
		if err != nil {
			return nil, err
		}
		return m.applyPostgresSQL(ctx, req.Target, "sql:"+req.SQL, req.ApprovalToken, statements)
	}
	if normalizeEngine(req.Target.Engine) == "mariadb" && req.Edit == nil {
		statements, err := mariadbWriteStatements(req.SQL)
		if err != nil {
			return nil, err
		}
		return m.applyMariaDBSQL(ctx, req.Target, "sql:"+req.SQL, req.ApprovalToken, statements)
	}
	stmts, operation, err := mutationInput(req.SQL, req.Edit)
	if err != nil {
		return nil, err
	}
	if normalizeEngine(req.Target.Engine) != "sqlite" {
		return m.applyNetworkCell(ctx, req, operation)
	}
	path, key, err := m.sqliteTarget(req.Target)
	if err != nil {
		return nil, err
	}
	approval, err := m.approvals.ConsumeToken(req.ApprovalToken, groxybridge.ActionDBMutate, mutationBinding(key, operation))
	if err != nil {
		return nil, err
	}
	image, err := m.readDBSnapshot(approval.SnapshotID, key)
	if err != nil {
		return nil, err
	}
	var affected int64
	err = withSQLiteWrite(ctx, path, true, func(ctx context.Context, conn *sql.Conn) error {
		before, err := sqliteImage(ctx, conn)
		if err != nil {
			return recoveryError(err)
		}
		if sqliteImageDigest(before) != sqliteImageDigest(image) {
			return security.NewError(security.CodeDeskE409, 409, "SQLite baseline changed after approval; review a fresh plan", nil)
		}
		var args []any
		if req.Edit != nil {
			query, values, err := prepareCell(ctx, conn, *req.Edit)
			if err != nil {
				return err
			}
			stmts[0], args = query, values
		}
		if err := sqliteUserSQL(conn, true); err != nil {
			return err
		}
		for _, st := range stmts {
			res, err := conn.ExecContext(ctx, st, args...)
			if err != nil {
				_ = sqliteUserSQL(conn, false)
				return security.NewError(security.CodeDeskE101, 422, "SQLite transaction rejected the approved SQL", err)
			}
			if n, err := res.RowsAffected(); err == nil {
				affected += n
			}
		}
		if err := sqliteUserSQL(conn, false); err != nil {
			return err
		}
		if req.Edit != nil && affected != 1 {
			return security.NewError(security.CodeDeskE409, 409, "inline edit did not resolve to one row", nil)
		}
		after, err := sqliteImage(ctx, conn)
		if err != nil {
			return recoveryError(err)
		}
		if _, err := sqliteObjects(ctx, conn, "main"); err != nil {
			return recoveryError(err)
		}
		return m.approvals.WriteDBOutcome(approval.SnapshotID, key, sqliteImageDigest(after))
	})
	if err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "engine": "sqlite", "snapshot_id": approval.SnapshotID,
		"statements_executed": strconv.Itoa(len(stmts)), "affected_rows": strconv.FormatInt(affected, 10)}, nil
}

func restoreBinding(key, id, baseline string) string { return key + ":" + id + ":" + baseline }

func (m *Manager) PlanRestore(ctx context.Context, req RestoreRequest) (map[string]any, error) {
	resolvedTarget, err := m.resolveTargetSecrets(req.Target)
	if err != nil {
		return nil, err
	}
	req.Target = resolvedTarget
	if strings.HasPrefix(req.SnapshotID, postgresFullPrefix) {
		return m.restorePostgresSQL(ctx, req, false)
	}
	if strings.HasPrefix(req.SnapshotID, mariadbFullPrefix) {
		return m.restoreMariaDBSQL(ctx, req, false)
	}
	if normalizeEngine(req.Target.Engine) != "sqlite" {
		return m.networkRestore(ctx, req, false)
	}
	if req.Force || req.BaselineSHA != "" {
		return nil, security.NewError(security.CodeDeskE406, 403, "forced restore is unavailable for this snapshot", nil)
	}
	_, key, err := m.sqliteTarget(req.Target)
	if err != nil {
		return nil, err
	}
	if _, err := m.readDBSnapshot(req.SnapshotID, key); err != nil {
		return nil, err
	}
	baseline, err := m.approvals.ReadDBOutcome(req.SnapshotID, key)
	if err != nil {
		return nil, err
	}
	approval, err := m.approvals.IssueSnapshotBoundToken(groxybridge.ActionDBRestore, restoreBinding(key, req.SnapshotID, baseline), req.SnapshotID)
	if err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "snapshot_id": req.SnapshotID, "target_label": req.Target.SQLitePath,
		"approval_token": approval.Token, "plain_summary": "Restore the saved SQLite data and schema if the database still matches the recorded operation."}, nil
}

func (m *Manager) RestoreSnapshot(ctx context.Context, req RestoreRequest) (map[string]any, error) {
	resolvedTarget, err := m.resolveTargetSecrets(req.Target)
	if err != nil {
		return nil, err
	}
	req.Target = resolvedTarget
	if strings.HasPrefix(req.SnapshotID, postgresFullPrefix) {
		return m.restorePostgresSQL(ctx, req, true)
	}
	if strings.HasPrefix(req.SnapshotID, mariadbFullPrefix) {
		return m.restoreMariaDBSQL(ctx, req, true)
	}
	if normalizeEngine(req.Target.Engine) != "sqlite" {
		return m.networkRestore(ctx, req, true)
	}
	if req.Force || req.BaselineSHA != "" {
		return nil, security.NewError(security.CodeDeskE406, 403, "forced restore is unavailable for this snapshot", nil)
	}
	path, key, err := m.sqliteTarget(req.Target)
	if err != nil {
		return nil, err
	}
	image, err := m.readDBSnapshot(req.SnapshotID, key)
	if err != nil {
		return nil, err
	}
	baseline, err := m.approvals.ReadDBOutcome(req.SnapshotID, key)
	if err != nil {
		return nil, err
	}
	if _, err := m.approvals.ConsumeToken(req.ApprovalToken, groxybridge.ActionDBRestore, restoreBinding(key, req.SnapshotID, baseline)); err != nil {
		return nil, err
	}
	backup, err := saveSQLiteScratch(ctx, image)
	if err != nil {
		return nil, recoveryError(err)
	}
	defer os.Remove(backup)
	err = withSQLiteWrite(ctx, path, false, func(ctx context.Context, conn *sql.Conn) error {
		current, err := sqliteImage(ctx, conn)
		if err != nil {
			return recoveryError(err)
		}
		if sqliteImageDigest(current) != baseline {
			return security.NewError(security.CodeDeskE409, 409, "SQLite has newer changes; restore was blocked", nil)
		}
		if err := restoreSQLiteContents(ctx, conn, backup); err != nil {
			return recoveryError(err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "status": "restored", "engine": "sqlite", "snapshot_id": req.SnapshotID,
		"restored_sha256": security.SHA256Hex(image)}, nil
}
