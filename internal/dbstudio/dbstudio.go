package dbstudio

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"groxy-desk/internal/groxybridge"
	"groxy-desk/internal/security"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"
	_ "modernc.org/sqlite"
)

const (
	MaxRowsPerPage = 100
	queryTimeout   = 10 * time.Second
)

var (
	reSafeIdent   = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]{0,63}$`)
	reUnsafeWrite = regexp.MustCompile(`(?i)\b(INSERT|UPDATE|DELETE|DROP|ALTER|CREATE|REPLACE|TRUNCATE|GRANT|REVOKE|ATTACH|DETACH|VACUUM|REINDEX|PRAGMA\s+[A-Za-z_]+\s*=|INTO\s+OUTFILE|INTO\s+DUMPFILE|LOAD_FILE)\b`)
	reAllowedPragma = regexp.MustCompile(`(?i)^PRAGMA\s+(table_info|index_list|index_info|foreign_key_list)\s*\(`)
)

// TargetSpec identifies an inspectable SQLite (GRAB-E401) or active 127.0.0.1 tunneled database.
type TargetSpec struct {
	Engine     string `json:"engine"` // sqlite | postgres | mariadb
	SQLitePath string `json:"sqlite_path,omitempty"`
	TunnelID   string `json:"tunnel_id,omitempty"`
	DirectHost string `json:"host,omitempty"` // rejected if non-empty to prevent public DB bypass
	DBUser     string `json:"db_user,omitempty"`
	DBPassword string `json:"db_password,omitempty"`
	DBName     string `json:"db_name,omitempty"`
	SchemaName string `json:"schema_name,omitempty"`
}

// ColumnMeta describes table/query column metadata.
type ColumnMeta struct {
	Name       string `json:"name"`
	Type       string `json:"type"`
	Nullable   bool   `json:"nullable"`
	PrimaryKey bool   `json:"primary_key"`
	DefaultVal string `json:"default_val,omitempty"`
}

// ForeignKeyMeta describes a table foreign key relationship.
type ForeignKeyMeta struct {
	Column    string `json:"column"`
	RefTable  string `json:"ref_table"`
	RefColumn string `json:"ref_column"`
}

// TableSchema holds schema metadata and exact ADR-025c base-10 row count.
type TableSchema struct {
	Name        string           `json:"name"`
	Schema      string           `json:"schema"`
	RowCount    string           `json:"row_count"`
	Columns     []ColumnMeta     `json:"columns"`
	Indexes     []string         `json:"indexes"`
	ForeignKeys []ForeignKeyMeta `json:"foreign_keys"`
}

// RowsRequest configures bounded, parameterized pagination, sorting, and filtering.
type RowsRequest struct {
	Target    TargetSpec `json:"target"`
	Table     string     `json:"table"`
	Page      string     `json:"page"`
	Limit     string     `json:"limit"`
	SortCol   string     `json:"sort_col,omitempty"`
	SortDir   string     `json:"sort_dir,omitempty"`
	FilterCol string     `json:"filter_col,omitempty"`
	FilterOp  string     `json:"filter_op,omitempty"`
	FilterVal string     `json:"filter_val,omitempty"`
}

// MutatePlanRequest prepares a GROXY-SAFE-001 DB mutation + Pre-Mutation SHA-256 Snapshot.
type MutatePlanRequest struct {
	Target TargetSpec `json:"target"`
	SQL    string     `json:"sql"`
}

// MutateApplyRequest executes an approved mutation gated by single-use approval_token.
type MutateApplyRequest struct {
	Target        TargetSpec `json:"target"`
	SQL           string     `json:"sql"`
	ApprovalToken string     `json:"approval_token"`
}

// Manager provides loopback-tunneled and SQLite Visual DB Explorer operations.
type Manager struct {
	workspaceRoot string
	approvals     *groxybridge.ApprovalManager
	redactor      *security.SecretRedactor
	tunnels       *TunnelManager
}

// NewManager creates a Visual DB Explorer manager bound cleanly to workspaceRoot.
func NewManager(workspaceRoot string, approvals *groxybridge.ApprovalManager, redactor *security.SecretRedactor) *Manager {
	if redactor == nil {
		redactor = security.NewSecretRedactor()
	}
	return &Manager{
		workspaceRoot: workspaceRoot,
		approvals:     approvals,
		redactor:      redactor,
		tunnels:       NewTunnelManager(redactor),
	}
}

// Tunnels returns the underlying loopback SSH DB TunnelManager.
func (m *Manager) Tunnels() *TunnelManager {
	return m.tunnels
}

// CloseAll closes all active SSH database tunnels on daemon/app teardown.
func (m *Manager) CloseAll() {
	if m.tunnels != nil {
		m.tunnels.CloseAll()
	}
}

// GetSchema discovers tables, columns, indexes, foreign keys, and exact row counts.
func (m *Manager) GetSchema(ctx context.Context, target TargetSpec) (map[string]any, error) {
	db, engine, label, err := m.openTarget(target)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	qCtx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	var tables []TableSchema
	switch engine {
	case "sqlite":
		tables, err = inspectSQLiteSchema(qCtx, db)
	case "postgres":
		schemaName := firstNonEmpty(target.SchemaName, "public")
		tables, err = inspectPostgresSchema(qCtx, db, schemaName)
	case "mariadb":
		tables, err = inspectMariaDBSchema(qCtx, db, target.DBName)
	}
	if err != nil {
		return nil, security.NewError(security.CodeDeskE101, 500, "database schema inspection failed", err)
	}

	return map[string]any{
		"ok":           true,
		"engine":       engine,
		"target_label": label,
		"tables":       tables,
	}, nil
}

// BrowseRows returns <= 100 bounded rows with exact ADR-025c base-10 wire strings.
func (m *Manager) BrowseRows(ctx context.Context, req RowsRequest) (map[string]any, error) {
	if err := validateIdentifier(req.Table); err != nil {
		return nil, err
	}
	page, err := parsePositiveInt(req.Page, "page", 1, 1000000)
	if err != nil {
		return nil, err
	}
	limit, err := parsePositiveInt(req.Limit, "limit", 50, MaxRowsPerPage)
	if err != nil {
		return nil, err
	}

	db, engine, _, err := m.openTarget(req.Target)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	tableIdent := quoteIdent(engine, req.Table)
	whereClause := ""
	var args []any
	if strings.TrimSpace(req.FilterCol) != "" {
		if err := validateIdentifier(req.FilterCol); err != nil {
			return nil, err
		}
		op, opErr := normalizeFilterOp(req.FilterOp)
		if opErr != nil {
			return nil, opErr
		}
		placeholder := "?"
		if engine == "postgres" {
			placeholder = "$1"
		}
		whereClause = fmt.Sprintf(" WHERE %s %s %s", quoteIdent(engine, req.FilterCol), op, placeholder)
		args = append(args, req.FilterVal)
	}

	orderClause := ""
	if strings.TrimSpace(req.SortCol) != "" {
		if err := validateIdentifier(req.SortCol); err != nil {
			return nil, err
		}
		dir := "ASC"
		if strings.EqualFold(strings.TrimSpace(req.SortDir), "DESC") {
			dir = "DESC"
		}
		orderClause = fmt.Sprintf(" ORDER BY %s %s", quoteIdent(engine, req.SortCol), dir)
	}

	qCtx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	var totalCount int64
	countSQL := fmt.Sprintf("SELECT COUNT(*) FROM %s%s", tableIdent, whereClause)
	if err := db.QueryRowContext(qCtx, countSQL, args...).Scan(&totalCount); err != nil {
		return nil, security.NewError(security.CodeDeskE101, 400, "failed counting table rows", err)
	}

	offset := (page - 1) * limit
	var limitSQL string
	if engine == "postgres" {
		p1 := len(args) + 1
		p2 := len(args) + 2
		limitSQL = fmt.Sprintf("SELECT * FROM %s%s%s LIMIT $%d OFFSET $%d", tableIdent, whereClause, orderClause, p1, p2)
	} else {
		limitSQL = fmt.Sprintf("SELECT * FROM %s%s%s LIMIT ? OFFSET ?", tableIdent, whereClause, orderClause)
	}
	queryArgs := append(append([]any(nil), args...), limit, offset)
	rows, err := db.QueryContext(qCtx, limitSQL, queryArgs...)
	if err != nil {
		return nil, security.NewError(security.CodeDeskE101, 400, "failed querying table rows", err)
	}
	defer rows.Close()

	cols, rowMaps, err := scanExactRows(rows, MaxRowsPerPage)
	if err != nil {
		return nil, err
	}

	totalPages := int64(1)
	if totalCount > 0 {
		totalPages = (totalCount + int64(limit) - 1) / int64(limit)
	}

	return map[string]any{
		"ok":          true,
		"engine":      engine,
		"table":       req.Table,
		"page":        strconv.Itoa(page),
		"limit":       strconv.Itoa(limit),
		"total_rows":  strconv.FormatInt(totalCount, 10),
		"total_pages": strconv.FormatInt(totalPages, 10),
		"columns":     cols,
		"rows":        rowMaps,
	}, nil
}

// RunReadOnlyQuery validates strict read-only SQL (403 DESK-E406 on writes/DDL/multi-statements) and returns <= 100 rows.
func (m *Manager) RunReadOnlyQuery(ctx context.Context, target TargetSpec, rawSQL string) (map[string]any, error) {
	verb, err := ValidateReadOnlySQL(rawSQL)
	if err != nil {
		return nil, err
	}
	db, engine, _, err := m.openTarget(target)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	qCtx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	rows, err := db.QueryContext(qCtx, strings.TrimSpace(rawSQL))
	if err != nil {
		return nil, security.NewError(security.CodeDeskE101, 400, "read-only SQL query execution failed", err)
	}
	defer rows.Close()

	cols, rowMaps, err := scanExactRows(rows, MaxRowsPerPage)
	if err != nil {
		return nil, err
	}
	truncated := len(rowMaps) == MaxRowsPerPage

	return map[string]any{
		"ok":        true,
		"engine":    engine,
		"verb":      verb,
		"read_only": true,
		"truncated": truncated,
		"row_count": strconv.Itoa(len(rowMaps)),
		"columns":   cols,
		"rows":      rowMaps,
	}, nil
}

// PlanMutation inspects a write/DDL/import SQL batch, captures a SHA-256 Pre-Mutation DB Snapshot (snap_db_*),
// and mints a single-use target-bound GROXY-SAFE-001 approval_token.
func (m *Manager) PlanMutation(ctx context.Context, req MutatePlanRequest) (map[string]any, error) {
	stmts, err := splitSQLStatements(req.SQL)
	if err != nil || len(stmts) == 0 {
		return nil, security.NewError(security.CodeDeskE101, 400, "valid SQL statement(s) required for mutation plan", err)
	}
	for _, s := range stmts {
		if strings.Contains(strings.ToUpper(s), "ATTACH") {
			return nil, security.NewError(security.CodeDeskE406, 403, "ATTACH DATABASE is blocked in Visual DB Explorer (DESK-E406)", nil)
		}
	}

	db, engine, label, err := m.openTarget(req.Target)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	snapPayload, err := m.capturePreMutationSnapshotPayload(ctx, db, engine, req.Target, stmts)
	if err != nil {
		return nil, err
	}

	targetKey := buildTargetKey(req.Target, engine)
	snap, err := m.approvals.CreatePrefixedVirtualSnapshot("snap_db_", targetKey, snapPayload)
	if err != nil {
		return nil, err
	}
	appr, err := m.approvals.IssueSnapshotBoundToken(groxybridge.ActionDBMutate, targetKey, snap.SnapshotID)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"ok":              true,
		"engine":          engine,
		"target_label":    label,
		"target_key":      targetKey,
		"statement_count": strconv.Itoa(len(stmts)),
		"snapshot":        snap,
		"approval_token":  appr.Token,
		"plain_summary":   fmt.Sprintf("Captured Pre-Mutation DB Snapshot %s before applying %d SQL statement(s) to %s.", snap.SnapshotID, len(stmts), label),
	}, nil
}

// ApplyMutation consumes a single-use approval_token and executes the SQL batch atomically inside a transaction.
func (m *Manager) ApplyMutation(ctx context.Context, req MutateApplyRequest) (map[string]any, error) {
	engine := normalizeEngine(req.Target.Engine)
	targetKey := buildTargetKey(req.Target, engine)

	appr, err := m.approvals.ConsumeToken(req.ApprovalToken, groxybridge.ActionDBMutate, targetKey)
	if err != nil {
		return nil, err
	}

	stmts, err := splitSQLStatements(req.SQL)
	if err != nil || len(stmts) == 0 {
		return nil, security.NewError(security.CodeDeskE101, 400, "SQL statements must not be empty", err)
	}

	db, openEngine, _, err := m.openTarget(req.Target)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	qCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	tx, err := db.BeginTx(qCtx, nil)
	if err != nil {
		return nil, security.NewError(security.CodeDeskE101, 500, "failed starting DB mutation transaction", err)
	}

	var affected int64
	for _, st := range stmts {
		res, execErr := tx.ExecContext(qCtx, st)
		if execErr != nil {
			_ = tx.Rollback()
			return nil, security.NewError(security.CodeDeskE101, 422, "DB mutation rolled back cleanly on SQL error", execErr)
		}
		if rows, rErr := res.RowsAffected(); rErr == nil {
			affected += rows
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, security.NewError(security.CodeDeskE101, 500, "failed committing DB mutation transaction", err)
	}

	return map[string]any{
		"ok":                  true,
		"engine":              openEngine,
		"snapshot_id":         appr.SnapshotID,
		"statements_executed": strconv.Itoa(len(stmts)),
		"affected_rows":       strconv.FormatInt(affected, 10),
	}, nil
}

// RestoreSnapshot verifies SHA-256 snapshot integrity and restores the pre-mutation SQLite file or SQL dump.
func (m *Manager) RestoreSnapshot(ctx context.Context, target TargetSpec, snapshotID string) (map[string]any, error) {
	restoreRes, err := m.approvals.RestoreSnapshot(m.workspaceRoot, snapshotID)
	if err != nil {
		return nil, err
	}
	var payload struct {
		Engine        string   `json:"engine"`
		SQLiteRelPath string   `json:"sqlite_rel_path,omitempty"`
		SQLiteBytesB64 string  `json:"sqlite_bytes_b64,omitempty"`
		RestoreSQL    []string `json:"restore_sql,omitempty"`
	}
	if err := json.Unmarshal([]byte(restoreRes.RestoredContent), &payload); err != nil {
		return nil, security.NewError(security.CodeDeskE101, 500, "corrupt DB snapshot payload format", err)
	}

	if payload.Engine == "sqlite" && payload.SQLiteBytesB64 != "" {
		rel := firstNonEmpty(payload.SQLiteRelPath, target.SQLitePath)
		absDB, _, confErr := security.ConfineWorkspacePath(m.workspaceRoot, rel)
		if confErr != nil {
			return nil, confErr
		}
		origBytes, decErr := base64.StdEncoding.DecodeString(payload.SQLiteBytesB64)
		if decErr != nil {
			return nil, security.NewError(security.CodeDeskE101, 500, "failed decoding SQLite snapshot bytes", decErr)
		}
		_ = os.Remove(absDB + "-wal")
		_ = os.Remove(absDB + "-shm")
		tmp := absDB + ".groxy-restore-tmp"
		if wErr := os.WriteFile(tmp, origBytes, 0o600); wErr != nil {
			return nil, security.NewError(security.CodeDeskE101, 500, "failed writing SQLite snapshot restore file", wErr)
		}
		if rErr := os.Rename(tmp, absDB); rErr != nil {
			_ = os.Remove(tmp)
			return nil, security.NewError(security.CodeDeskE101, 500, "failed swapping restored SQLite database", rErr)
		}
	} else if len(payload.RestoreSQL) > 0 {
		db, _, _, err := m.openTarget(target)
		if err != nil {
			return nil, err
		}
		defer db.Close()
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return nil, err
		}
		for _, stmt := range payload.RestoreSQL {
			if _, execErr := tx.ExecContext(ctx, stmt); execErr != nil {
				_ = tx.Rollback()
				return nil, security.NewError(security.CodeDeskE101, 500, "failed executing DB restore SQL", execErr)
			}
		}
		if err := tx.Commit(); err != nil {
			return nil, err
		}
	}

	return map[string]any{
		"ok":              true,
		"snapshot_id":     restoreRes.SnapshotID,
		"restored_sha256": restoreRes.RestoredSHA256,
		"engine":          payload.Engine,
		"status":          "restored",
	}, nil
}

func (m *Manager) capturePreMutationSnapshotPayload(ctx context.Context, db *sql.DB, engine string, target TargetSpec, stmts []string) (string, error) {
	if engine == "sqlite" {
		absDB, relDB, err := security.ConfineWorkspacePath(m.workspaceRoot, target.SQLitePath)
		if err != nil {
			return "", err
		}
		_, _ = db.ExecContext(ctx, "PRAGMA wal_checkpoint(TRUNCATE);")
		rawBytes, rErr := os.ReadFile(absDB)
		if rErr != nil {
			return "", security.NewError(security.CodeDeskE101, 500, "failed reading SQLite database bytes for snapshot", rErr)
		}
		out, _ := json.Marshal(map[string]any{
			"engine":           "sqlite",
			"sqlite_rel_path":  relDB,
			"sqlite_bytes_b64": base64.StdEncoding.EncodeToString(rawBytes),
			"statements":       stmts,
		})
		return string(out), nil
	}

	// Remote tunneled Postgres / MariaDB: snapshot affected table rows or schema list
	out, _ := json.Marshal(map[string]any{
		"engine":     engine,
		"tunnel_id":  target.TunnelID,
		"db_name":    target.DBName,
		"statements": stmts,
	})
	return string(out), nil
}

func (m *Manager) openTarget(target TargetSpec) (*sql.DB, string, string, error) {
	if strings.TrimSpace(target.DirectHost) != "" {
		return nil, "", "", security.NewError(security.CodeDeskE501, 422, "direct public database hosts are forbidden; use an active 127.0.0.1 SSH tunnel or confined SQLite file (DESK-E501)", nil)
	}
	engine := normalizeEngine(target.Engine)
	switch engine {
	case "sqlite":
		absPath, relPath, err := security.ConfineWorkspacePath(m.workspaceRoot, target.SQLitePath)
		if err != nil {
			return nil, "", "", err
		}
		if _, stErr := os.Stat(absPath); stErr != nil {
			return nil, "", "", security.NewError(security.CodeDeskE101, 404, fmt.Sprintf("SQLite database %q not found", relPath), stErr)
		}
		db, err := sql.Open("sqlite", absPath)
		if err != nil {
			return nil, "", "", security.NewError(security.CodeDeskE101, 500, "failed opening SQLite database", err)
		}
		_, _ = db.Exec("PRAGMA foreign_keys = ON;")
		_, _ = db.Exec("PRAGMA journal_mode = DELETE;")
		_, _ = db.Exec("PRAGMA busy_timeout = 5000;")
		return db, "sqlite", relPath, nil

	case "postgres":
		tun, ok := m.tunnels.GetTunnel(target.TunnelID)
		if !ok || tun.Engine != "postgres" {
			return nil, "", "", security.NewError(security.CodeDeskE405, 403, "active pinned PostgreSQL SSH tunnel is required (DESK-E405)", nil)
		}
		if strings.TrimSpace(target.DBPassword) != "" {
			m.redactor.Register(target.DBPassword)
		}
		u := url.URL{
			Scheme: "postgres",
			User:   url.UserPassword(firstNonEmpty(target.DBUser, "postgres"), target.DBPassword),
			Host:   tun.LocalBindAddr,
			Path:   "/" + firstNonEmpty(target.DBName, "postgres"),
		}
		q := u.Query()
		q.Set("sslmode", "disable")
		u.RawQuery = q.Encode()
		dsn := u.String()
		m.redactor.Register(dsn)
		db, err := sql.Open("pgx", dsn)
		if err != nil {
			return nil, "", "", security.NewError(security.CodeDeskE101, 500, "failed initializing PostgreSQL tunnel client", err)
		}
		return db, "postgres", fmt.Sprintf("%s@%s/%s", firstNonEmpty(target.DBUser, "postgres"), tun.LocalBindAddr, firstNonEmpty(target.DBName, "postgres")), nil

	case "mariadb":
		tun, ok := m.tunnels.GetTunnel(target.TunnelID)
		if !ok || tun.Engine != "mariadb" {
			return nil, "", "", security.NewError(security.CodeDeskE405, 403, "active pinned MariaDB SSH tunnel is required (DESK-E405)", nil)
		}
		if strings.TrimSpace(target.DBPassword) != "" {
			m.redactor.Register(target.DBPassword)
		}
		dsn := fmt.Sprintf("%s:%s@tcp(%s)/%s?parseTime=false",
			firstNonEmpty(target.DBUser, "root"),
			target.DBPassword,
			tun.LocalBindAddr,
			target.DBName,
		)
		m.redactor.Register(dsn)
		db, err := sql.Open("mysql", dsn)
		if err != nil {
			return nil, "", "", security.NewError(security.CodeDeskE101, 500, "failed initializing MariaDB tunnel client", err)
		}
		return db, "mariadb", fmt.Sprintf("%s@%s/%s", firstNonEmpty(target.DBUser, "root"), tun.LocalBindAddr, target.DBName), nil

	default:
		return nil, "", "", security.NewError(security.CodeDeskE101, 400, fmt.Sprintf("unsupported DB engine %q (expected sqlite, postgres, or mariadb)", target.Engine), nil)
	}
}

// ValidateReadOnlySQL rejects DML/DDL writes and multi-statement stacked SQL with 403 DESK-E406.
func ValidateReadOnlySQL(rawSQL string) (string, error) {
	stripped := stripSQLComments(rawSQL)
	trimmed := strings.TrimSpace(stripped)
	if trimmed == "" {
		return "", security.NewError(security.CodeDeskE101, 400, "SQL query must not be empty", nil)
	}
	// Strip at most one trailing semicolon
	trimmed = strings.TrimSpace(strings.TrimSuffix(trimmed, ";"))
	if hasUnquotedSemicolon(trimmed) {
		return "", security.NewError(security.CodeDeskE406, 403, "multi-statement SQL is forbidden in read-only mode (DESK-E406)", nil)
	}
	upper := strings.ToUpper(trimmed)
	fields := strings.Fields(upper)
	if len(fields) == 0 {
		return "", security.NewError(security.CodeDeskE101, 400, "SQL query must not be empty", nil)
	}
	verb := fields[0]
	switch verb {
	case "SELECT", "WITH", "EXPLAIN":
		if reUnsafeWrite.MatchString(stripped) {
			return "", security.NewError(security.CodeDeskE406, 403, "write/DDL keywords are blocked in read-only DB query mode (DESK-E406)", nil)
		}
		return verb, nil
	case "PRAGMA":
		if !reAllowedPragma.MatchString(trimmed) || reUnsafeWrite.MatchString(stripped) {
			return "", security.NewError(security.CodeDeskE406, 403, "only read-only PRAGMA introspection is permitted (DESK-E406)", nil)
		}
		return "PRAGMA", nil
	default:
		return "", security.NewError(security.CodeDeskE406, 403, fmt.Sprintf("statement %q blocked in read-only DB mode (DESK-E406); use Guarded Mutation Plan", verb), nil)
	}
}

func inspectSQLiteSchema(ctx context.Context, db *sql.DB) ([]TableSchema, error) {
	tRows, err := db.QueryContext(ctx, "SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name")
	if err != nil {
		return nil, err
	}
	var names []string
	for tRows.Next() {
		var name string
		if sErr := tRows.Scan(&name); sErr == nil && validateIdentifier(name) == nil {
			names = append(names, name)
		}
	}
	_ = tRows.Close()

	out := make([]TableSchema, 0, len(names))
	for _, tName := range names {
		quoted := quoteIdent("sqlite", tName)
		var count int64
		_ = db.QueryRowContext(ctx, fmt.Sprintf("SELECT COUNT(*) FROM %s", quoted)).Scan(&count)

		cols := make([]ColumnMeta, 0)
		cRows, cErr := db.QueryContext(ctx, fmt.Sprintf("PRAGMA table_info(%s)", quoted))
		if cErr == nil {
			for cRows.Next() {
				var cid, notNull, pk int
				var cName, cType string
				var dflt sql.NullString
				if cRows.Scan(&cid, &cName, &cType, &notNull, &dflt, &pk) == nil {
					cols = append(cols, ColumnMeta{
						Name:       cName,
						Type:       cType,
						Nullable:   notNull == 0,
						PrimaryKey: pk > 0,
						DefaultVal: dflt.String,
					})
				}
			}
			_ = cRows.Close()
		}

		idxs := make([]string, 0)
		iRows, iErr := db.QueryContext(ctx, fmt.Sprintf("PRAGMA index_list(%s)", quoted))
		if iErr == nil {
			for iRows.Next() {
				var seq, unique, partial int
				var iName, origin string
				if iRows.Scan(&seq, &iName, &unique, &origin, &partial) == nil {
					idxs = append(idxs, iName)
				}
			}
			_ = iRows.Close()
		}

		fks := make([]ForeignKeyMeta, 0)
		fRows, fErr := db.QueryContext(ctx, fmt.Sprintf("PRAGMA foreign_key_list(%s)", quoted))
		if fErr == nil {
			for fRows.Next() {
				var id, seq int
				var refTable, fromCol, toCol, onUpd, onDel, match string
				if fRows.Scan(&id, &seq, &refTable, &fromCol, &toCol, &onUpd, &onDel, &match) == nil {
					fks = append(fks, ForeignKeyMeta{
						Column:    fromCol,
						RefTable:  refTable,
						RefColumn: toCol,
					})
				}
			}
			_ = fRows.Close()
		}

		out = append(out, TableSchema{
			Name:        tName,
			Schema:      "main",
			RowCount:    strconv.FormatInt(count, 10),
			Columns:     cols,
			Indexes:     idxs,
			ForeignKeys: fks,
		})
	}
	return out, nil
}

func inspectPostgresSchema(ctx context.Context, db *sql.DB, schemaName string) ([]TableSchema, error) {
	rows, err := db.QueryContext(ctx, "SELECT tablename FROM pg_tables WHERE schemaname = $1 ORDER BY tablename", schemaName)
	if err != nil {
		return nil, err
	}
	var names []string
	for rows.Next() {
		var name string
		if rows.Scan(&name) == nil && validateIdentifier(name) == nil {
			names = append(names, name)
		}
	}
	_ = rows.Close()

	out := make([]TableSchema, 0, len(names))
	for _, tName := range names {
		var count int64
		_ = db.QueryRowContext(ctx, fmt.Sprintf("SELECT COUNT(*) FROM %s.%s", quoteIdent("postgres", schemaName), quoteIdent("postgres", tName))).Scan(&count)
		cols := make([]ColumnMeta, 0)
		cRows, cErr := db.QueryContext(ctx, "SELECT column_name, data_type, is_nullable FROM information_schema.columns WHERE table_schema=$1 AND table_name=$2 ORDER BY ordinal_position", schemaName, tName)
		if cErr == nil {
			for cRows.Next() {
				var cName, dType, isNull string
				if cRows.Scan(&cName, &dType, &isNull) == nil {
					cols = append(cols, ColumnMeta{
						Name:     cName,
						Type:     dType,
						Nullable: strings.EqualFold(isNull, "YES"),
					})
				}
			}
			_ = cRows.Close()
		}
		out = append(out, TableSchema{
			Name:     tName,
			Schema:   schemaName,
			RowCount: strconv.FormatInt(count, 10),
			Columns:  cols,
		})
	}
	return out, nil
}

func inspectMariaDBSchema(ctx context.Context, db *sql.DB, dbName string) ([]TableSchema, error) {
	rows, err := db.QueryContext(ctx, "SHOW TABLES")
	if err != nil {
		return nil, err
	}
	var names []string
	for rows.Next() {
		var name string
		if rows.Scan(&name) == nil && validateIdentifier(name) == nil {
			names = append(names, name)
		}
	}
	_ = rows.Close()

	out := make([]TableSchema, 0, len(names))
	for _, tName := range names {
		var count int64
		_ = db.QueryRowContext(ctx, fmt.Sprintf("SELECT COUNT(*) FROM %s", quoteIdent("mariadb", tName))).Scan(&count)
		out = append(out, TableSchema{
			Name:     tName,
			Schema:   dbName,
			RowCount: strconv.FormatInt(count, 10),
		})
	}
	return out, nil
}

func scanExactRows(rows *sql.Rows, maxRows int) ([]ColumnMeta, []map[string]any, error) {
	colTypes, err := rows.ColumnTypes()
	if err != nil {
		return nil, nil, security.NewError(security.CodeDeskE101, 500, "failed inspecting result columns", err)
	}
	cols := make([]ColumnMeta, len(colTypes))
	for i, ct := range colTypes {
		nullable, _ := ct.Nullable()
		cols[i] = ColumnMeta{
			Name:     ct.Name(),
			Type:     ct.DatabaseTypeName(),
			Nullable: nullable,
		}
	}

	result := make([]map[string]any, 0)
	for rows.Next() {
		if len(result) >= maxRows {
			break
		}
		dest := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range dest {
			ptrs[i] = &dest[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, nil, security.NewError(security.CodeDeskE101, 500, "failed scanning result row", err)
		}
		rec := make(map[string]any, len(cols))
		for i, col := range cols {
			rec[col.Name] = formatWireCell(dest[i])
		}
		result = append(result, rec)
	}
	return cols, result, nil
}

// formatWireCell preserves exact ADR-025c base-10 integer/decimal/text wire strings.
func formatWireCell(val any) any {
	switch v := val.(type) {
	case nil:
		return nil
	case int64:
		return strconv.FormatInt(v, 10)
	case int32:
		return strconv.FormatInt(int64(v), 10)
	case int:
		return strconv.Itoa(v)
	case uint64:
		return strconv.FormatUint(v, 10)
	case float64:
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return ""
		}
		return strconv.FormatFloat(v, 'f', -1, 64)
	case bool:
		return v
	case []byte:
		return string(v)
	case time.Time:
		return v.UTC().Format(time.RFC3339Nano)
	case string:
		return v
	default:
		return fmt.Sprintf("%v", v)
	}
}

func validateIdentifier(name string) error {
	if !reSafeIdent.MatchString(strings.TrimSpace(name)) {
		return security.NewError(security.CodeDeskE101, 400, fmt.Sprintf("invalid SQL identifier %q", name), nil)
	}
	return nil
}

func quoteIdent(engine, name string) string {
	clean := strings.TrimSpace(name)
	if engine == "mariadb" {
		return "`" + clean + "`"
	}
	return `"` + clean + `"`
}

func normalizeFilterOp(raw string) (string, error) {
	switch strings.ToUpper(strings.TrimSpace(raw)) {
	case "", "=":
		return "=", nil
	case "!=", "<>":
		return "!=", nil
	case "<", ">", "<=", ">=", "LIKE":
		return strings.ToUpper(strings.TrimSpace(raw)), nil
	default:
		return "", security.NewError(security.CodeDeskE101, 400, fmt.Sprintf("unsupported filter operator %q", raw), nil)
	}
}

func parsePositiveInt(raw, field string, fallback, maxVal int) (int, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return fallback, nil
	}
	if !security.IsExactIntString(trimmed) {
		return 0, security.NewError(security.CodeDeskE101, 400, fmt.Sprintf("%s must be a base-10 integer string", field), nil)
	}
	val, err := strconv.Atoi(trimmed)
	if err != nil || val <= 0 || val > maxVal {
		return 0, security.NewError(security.CodeDeskE101, 400, fmt.Sprintf("%s must be between 1 and %d", field, maxVal), nil)
	}
	return val, nil
}

func buildTargetKey(t TargetSpec, engine string) string {
	if engine == "sqlite" {
		cleaned := strings.TrimSpace(strings.ReplaceAll(t.SQLitePath, "\\", "/"))
		return "db:sqlite:" + strings.TrimPrefix(cleaned, "./")
	}
	return fmt.Sprintf("db:%s:%s/%s", engine, strings.TrimSpace(t.TunnelID), strings.TrimSpace(t.DBName))
}

func stripSQLComments(sqlText string) string {
	var b strings.Builder
	inSingle := false
	inDouble := false
	n := len(sqlText)
	for i := 0; i < n; i++ {
		c := sqlText[i]
		if c == '\'' && !inDouble {
			inSingle = !inSingle
			b.WriteByte(c)
			continue
		}
		if c == '"' && !inSingle {
			inDouble = !inDouble
			b.WriteByte(c)
			continue
		}
		if !inSingle && !inDouble {
			if c == '-' && i+1 < n && sqlText[i+1] == '-' {
				for i < n && sqlText[i] != '\n' {
					i++
				}
				b.WriteByte(' ')
				continue
			}
			if c == '/' && i+1 < n && sqlText[i+1] == '*' {
				i += 2
				for i+1 < n && !(sqlText[i] == '*' && sqlText[i+1] == '/') {
					i++
				}
				i++
				b.WriteByte(' ')
				continue
			}
		}
		b.WriteByte(c)
	}
	return b.String()
}

func hasUnquotedSemicolon(s string) bool {
	inSingle := false
	inDouble := false
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '\'':
			if !inDouble {
				inSingle = !inSingle
			}
		case '"':
			if !inSingle {
				inDouble = !inDouble
			}
		case ';':
			if !inSingle && !inDouble {
				return true
			}
		}
	}
	return false
}

func splitSQLStatements(rawSQL string) ([]string, error) {
	cleaned := stripSQLComments(rawSQL)
	var out []string
	var cur strings.Builder
	inSingle := false
	inDouble := false
	for i := 0; i < len(cleaned); i++ {
		c := cleaned[i]
		if c == '\'' && !inDouble {
			inSingle = !inSingle
		} else if c == '"' && !inSingle {
			inDouble = !inDouble
		} else if c == ';' && !inSingle && !inDouble {
			stmt := strings.TrimSpace(cur.String())
			if stmt != "" {
				out = append(out, stmt)
			}
			cur.Reset()
			continue
		}
		cur.WriteByte(c)
	}
	if inSingle || inDouble {
		return nil, fmt.Errorf("unterminated SQL string literal")
	}
	if tail := strings.TrimSpace(cur.String()); tail != "" {
		out = append(out, tail)
	}
	return out, nil
}
