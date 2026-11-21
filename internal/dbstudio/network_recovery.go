package dbstudio

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"groxy-desk/internal/groxybridge"
	"groxy-desk/internal/security"
)

const (
	maxNetworkSnapshotRows  = 1000
	maxNetworkSnapshotBytes = 16 << 20
)

type networkColumn struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Required bool   `json:"required"`
}

type networkTable struct {
	SchemaDefinition string          `json:"schema_definition,omitempty"`
	Name             string          `json:"table"`
	Columns          []networkColumn `json:"columns"`
	Primary          string          `json:"primary_key"`
	Create           string          `json:"create_table_suffix"`
	Indexes          []string        `json:"create_index_suffixes,omitempty"`
	Rows             [][]*string     `json:"rows"`
}

type networkSnapshot struct {
	Table networkTable `json:"data_and_schema"`
	Edit  CellEdit     `json:"edit"`
}

type networkScope struct {
	engine, schema, database, table, key, label string
	tunnel                                      *managedTunnel
}

var (
	networkInteger = regexp.MustCompile(`^(smallint|integer|bigint|tinyint|mediumint|int)(\([0-9]+\))?$`)
	networkDecimal = regexp.MustCompile(`^(numeric|decimal)\(([0-9]+),([0-9]+)\)$`)
	networkText    = regexp.MustCompile(`^(text|character varying(\([0-9]+\))?|varchar\([0-9]+\))$`)
)

func unsupportedNetworkScope() error {
	return security.NewError(security.CodeDeskE406, 403, "this target or operation is outside the verified network recovery scope", nil)
}

func (m *Manager) fullNetworkWriteUnavailable(target TargetSpec) error {
	if _, err := m.networkWriteTunnel(target); err != nil {
		return err
	}
	if normalizeEngine(target.Engine) == "mariadb" {
		return writeFenceUnavailable("Desk cannot exclude all writers to this MariaDB database through replay and verification. The request requires target-only isolation.", nil)
	}
	return writeFenceUnavailable("Full database writes require a verified write fence so Desk can restore the database without overwriting concurrent changes.", nil)
}

func networkRecoveryError(err error) error {
	var deskErr *security.DeskError
	if errors.As(err, &deskErr) {
		return err
	}
	return security.NewError(security.CodeDeskE406, 422, "network database recovery verification failed", err)
}

func (m *Manager) networkWriteTunnel(target TargetSpec) (*managedTunnel, error) {
	engine := normalizeEngine(target.Engine)
	if engine != "postgres" && engine != "mariadb" || target.DirectHost != "" {
		return nil, unsupportedNetworkScope()
	}
	tun := m.tunnels.activeTunnel(target.TunnelID)
	if tun == nil || tun.view.Engine != engine || tun.sshClient == nil {
		return nil, security.NewError(security.CodeDeskE405, 403, "network writes require an active, authenticated SSH tunnel", nil)
	}
	return tun, nil
}

func (m *Manager) networkScopeFor(target TargetSpec, table string) (networkScope, error) {
	tun, err := m.networkWriteTunnel(target)
	if err != nil {
		return networkScope{}, err
	}
	engine := tun.view.Engine
	database, schema, user := target.DBName, target.DBName, target.DBUser
	if engine == "postgres" {
		database = firstNonEmpty(target.DBName, "postgres")
		schema = firstNonEmpty(target.SchemaName, "public")
		user = firstNonEmpty(target.DBUser, "postgres")
	}
	if validateIdentifier(database) != nil || validateIdentifier(schema) != nil || validateIdentifier(table) != nil || user == "" {
		return networkScope{}, invalidCell("select a named database, schema, table, and database principal")
	}
	switch strings.ToLower(schema) {
	case "information_schema", "mysql", "performance_schema", "sys":
		return networkScope{}, unsupportedNetworkScope()
	}
	if strings.HasPrefix(strings.ToLower(schema), "pg_") {
		return networkScope{}, unsupportedNetworkScope()
	}
	identity, _ := json.Marshal([]string{engine, tun.view.SSHHost, tun.view.SSHPort, tun.view.PinnedFingerprint,
		tun.remoteAddr, database, schema, user})
	return networkScope{engine: engine, schema: schema, database: database, table: table, tunnel: tun,
		key:   "db:" + engine + ":" + security.SHA256Hex(identity),
		label: fmt.Sprintf("%s / %s.%s via %s", database, schema, table, tun.view.SSHHost)}, nil
}

func (s networkScope) tableSQL() string {
	return quoteIdent(s.engine, s.schema) + "." + quoteIdent(s.engine, s.table)
}

func (m *Manager) withNetworkScope(ctx context.Context, target TargetSpec, scope networkScope, run func(context.Context, *sql.Tx) error) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
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
	return withNetworkTableTransaction(ctx, conn, scope, run)
}

func withNetworkTableTransaction(ctx context.Context, conn *sql.Conn, scope networkScope, run func(context.Context, *sql.Tx) error) error {
	tx, err := conn.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if scope.engine == "postgres" {
		if _, err := tx.ExecContext(ctx, "SET LOCAL search_path = pg_catalog, pg_temp"); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, "LOCK TABLE "+scope.tableSQL()+" IN SHARE ROW EXCLUSIVE MODE"); err != nil {
			return err
		}
	} else {
		// Acquire a transaction-held metadata lock before inspecting the engine or
		// dependencies. The full primary-key locking read also locks insertion gaps.
		rows, err := tx.QueryContext(ctx, "SELECT 1 FROM "+scope.tableSQL()+" LIMIT 1 FOR UPDATE")
		if err != nil {
			return err
		}
		if err := rows.Close(); err != nil {
			return err
		}
	}
	if err := run(ctx, tx); err != nil {
		return err
	}
	return tx.Commit()
}

func snapshotNetworkTable(ctx context.Context, tx *sql.Tx, s networkScope) (networkTable, error) {
	var table networkTable
	var err error
	if s.engine == "postgres" {
		table, err = describePostgresTable(ctx, tx, s)
	} else {
		table, err = describeMariaTable(ctx, tx, s)
	}
	if err != nil {
		return table, err
	}
	table.Rows, err = readNetworkRows(ctx, tx, s.engine, s.tableSQL(), table, s.engine == "mariadb")
	return table, err
}

func readNetworkRows(ctx context.Context, tx *sql.Tx, engine, tableSQL string, table networkTable, lock bool) ([][]*string, error) {
	cols := make([]string, 0, len(table.Columns))
	for _, column := range table.Columns {
		name := quoteIdent(engine, column.Name)
		if engine == "postgres" {
			cols = append(cols, name+"::text")
		} else {
			cols = append(cols, "CAST("+name+" AS CHAR CHARACTER SET utf8mb4)")
		}
	}
	query := "SELECT " + strings.Join(cols, ",") + " FROM " + tableSQL + " ORDER BY " + quoteIdent(engine, table.Primary) +
		" LIMIT " + strconv.Itoa(maxNetworkSnapshotRows+1)
	if lock {
		query += " FOR UPDATE"
	}
	result, err := tx.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer result.Close()
	rows := make([][]*string, 0)
	var totalBytes int
	for result.Next() {
		if len(rows) >= maxNetworkSnapshotRows {
			return nil, unsupportedNetworkScope()
		}
		row, dest := make([]*string, len(cols)), make([]any, len(cols))
		for i := range row {
			dest[i] = &row[i]
		}
		if err := result.Scan(dest...); err != nil {
			return nil, err
		}
		for _, value := range row {
			if value != nil {
				totalBytes += len(*value)
			}
		}
		if totalBytes > maxNetworkSnapshotBytes {
			return nil, unsupportedNetworkScope()
		}
		rows = append(rows, row)
	}
	return rows, result.Err()
}

func networkParam(engine string, pos int, column networkColumn) string {
	if engine == "mariadb" {
		return "?"
	}
	placeholder := "$" + strconv.Itoa(pos) + "::text"
	if !networkText.MatchString(column.Type) {
		placeholder += "::" + column.Type
	}
	return placeholder
}

func reconstructNetworkTable(ctx context.Context, tx *sql.Tx, engine string, table networkTable) error {
	id, err := security.RandomTokenHex(12)
	if err != nil {
		return err
	}
	name := quoteIdent(engine, "desk_restore_"+id)
	create := "CREATE TEMPORARY TABLE " + name + " " + table.Create
	if engine == "postgres" {
		create += " ON COMMIT DROP"
	}
	if _, err := tx.ExecContext(ctx, create); err != nil {
		return err
	}
	for _, index := range table.Indexes {
		if _, err := tx.ExecContext(ctx, strings.Replace(index, "{table}", name, 1)); err != nil {
			return err
		}
	}
	if err := insertNetworkRows(ctx, tx, engine, name, table); err != nil {
		return err
	}
	restored, err := readNetworkRows(ctx, tx, engine, name, table, false)
	if err != nil {
		return err
	}
	if !sameNetworkRows(restored, table.Rows) {
		return fmt.Errorf("restored network snapshot rows differ from the original")
	}
	return nil
}

func insertNetworkRows(ctx context.Context, tx *sql.Tx, engine, name string, table networkTable) error {
	params := make([]string, 0, len(table.Columns))
	for i, col := range table.Columns {
		params = append(params, networkParam(engine, i+1, col))
	}
	query := "INSERT INTO " + name + " VALUES (" + strings.Join(params, ",") + ")"
	for _, row := range table.Rows {
		args := make([]any, len(row))
		for i, val := range row {
			args[i] = val
		}
		if _, err := tx.ExecContext(ctx, query, args...); err != nil {
			return err
		}
	}
	return nil
}

func sameNetworkRows(a, b [][]*string) bool {
	return slices.EqualFunc(a, b, func(a, b []*string) bool { return slices.EqualFunc(a, b, sameNetworkValue) })
}

func sameNetworkValue(a, b *string) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

func networkCellValue(raw json.RawMessage, column networkColumn) (*string, error) {
	var value *string
	if len(raw) == 0 || json.Unmarshal(raw, &value) != nil {
		return nil, invalidCell("use JSON strings or null for cell values")
	}
	if value == nil {
		if column.Required {
			return nil, invalidCell("this column cannot be null")
		}
		return nil, nil
	}
	if networkInteger.MatchString(column.Type) {
		n, err := strconv.ParseInt(*value, 10, 64)
		bits := 32
		switch {
		case strings.HasPrefix(column.Type, "bigint"):
			bits = 64
		case strings.HasPrefix(column.Type, "smallint"):
			bits = 16
		case strings.HasPrefix(column.Type, "mediumint"):
			bits = 24
		case strings.HasPrefix(column.Type, "tinyint"):
			bits = 8
		}
		if err != nil || strconv.FormatInt(n, 10) != *value || n < -1<<(bits-1) || n > 1<<(bits-1)-1 {
			return nil, invalidCell("integer must fit the column and use an exact base-10 string")
		}
	}
	if match := networkDecimal.FindStringSubmatch(column.Type); match != nil {
		precision, _ := strconv.Atoi(match[2])
		scale, _ := strconv.Atoi(match[3])
		whole, fraction, _ := strings.Cut(strings.TrimPrefix(*value, "-"), ".")
		if !security.IsExactDecimalString(*value) || len(strings.TrimLeft(whole, "0")) > precision-scale || len(fraction) != scale {
			return nil, invalidCell(fmt.Sprintf("use an exact base-10 value with %d fractional digits within %s", scale, column.Type))
		}
	}
	return value, nil
}

func networkEditLocation(table networkTable, edit CellEdit) (int, int, int, *string, error) {
	col := slices.IndexFunc(table.Columns, func(c networkColumn) bool { return c.Name == edit.Column })
	key := slices.IndexFunc(table.Columns, func(c networkColumn) bool { return c.Name == table.Primary })
	if col < 0 || key < 0 || col == key || len(edit.Key) != 1 {
		return 0, 0, 0, nil, invalidCell("the verified table needs a single primary key and an editable non-key column")
	}
	keyValue, err := networkCellValue(edit.Key[table.Primary], table.Columns[key])
	if err != nil {
		return 0, 0, 0, nil, err
	}
	before, err := networkCellValue(edit.Before, table.Columns[col])
	if err != nil {
		return 0, 0, 0, nil, err
	}
	after, err := networkCellValue(edit.Value, table.Columns[col])
	if err != nil {
		return 0, 0, 0, nil, err
	}
	row := slices.IndexFunc(table.Rows, func(row []*string) bool { return sameNetworkValue(row[key], keyValue) })
	if row < 0 || !sameNetworkValue(table.Rows[row][col], before) {
		return 0, 0, 0, nil, changedNetworkTarget()
	}
	return row, col, key, after, nil
}

func changeNetworkCell(ctx context.Context, tx *sql.Tx, scope networkScope, table networkTable, edit CellEdit) (networkTable, error) {
	row, col, key, replacement, err := networkEditLocation(table, edit)
	if err != nil {
		return table, err
	}
	if sameNetworkValue(table.Rows[row][col], replacement) {
		return table, invalidCell("replacement must change the selected cell")
	}
	query := "UPDATE " + scope.tableSQL() + " SET " + quoteIdent(scope.engine, edit.Column) + "=" + networkParam(scope.engine, 1, table.Columns[col]) +
		" WHERE " + quoteIdent(scope.engine, table.Primary) + "=" + networkParam(scope.engine, 2, table.Columns[key])
	res, err := tx.ExecContext(ctx, query, replacement, table.Rows[row][key])
	if err != nil {
		return table, err
	}
	if n, err := res.RowsAffected(); err != nil || n != 1 {
		return table, changedNetworkTarget()
	}
	table.Rows[row][col] = replacement
	actual, err := snapshotNetworkTable(ctx, tx, scope)
	if err != nil {
		return table, err
	}
	if !bytes.Equal(networkTableJSON(actual), networkTableJSON(table)) {
		return table, invalidCell("the database could not preserve the exact approved cell value and surrounding data")
	}
	return actual, nil
}

func networkTableJSON(table networkTable) []byte {
	raw, _ := json.Marshal(table)
	return raw
}

func changedNetworkTarget() error {
	return security.NewError(security.CodeDeskE409, 409, "network database data or schema changed; review a fresh plan", nil)
}

func (m *Manager) readNetworkSnapshot(id, target string) (networkSnapshot, error) {
	raw, err := m.approvals.ReadDBSnapshot(id, target)
	if err != nil {
		return networkSnapshot{}, err
	}
	var snap networkSnapshot
	if len(raw) > maxNetworkSnapshotBytes || json.Unmarshal(raw, &snap) != nil || snap.Table.Primary == "" ||
		snap.Table.Create == "" || snap.Table.Name != snap.Edit.Table {
		return networkSnapshot{}, networkRecoveryError(fmt.Errorf("invalid recoverable network snapshot"))
	}
	return snap, nil
}

func (m *Manager) planNetworkCell(ctx context.Context, req MutatePlanRequest, operation string) (map[string]any, error) {
	if req.Edit == nil {
		return nil, m.fullNetworkWriteUnavailable(req.Target)
	}
	scope, err := m.networkScopeFor(req.Target, req.Edit.Table)
	if err != nil {
		return nil, err
	}
	var record groxybridge.SnapshotRecord
	err = m.withNetworkScope(ctx, req.Target, scope, func(ctx context.Context, tx *sql.Tx) error {
		table, err := snapshotNetworkTable(ctx, tx, scope)
		if err != nil {
			return err
		}
		row, col, _, next, err := networkEditLocation(table, *req.Edit)
		if err != nil {
			return err
		}
		if sameNetworkValue(table.Rows[row][col], next) {
			return invalidCell("replacement must change the selected cell")
		}
		raw, err := json.Marshal(networkSnapshot{Table: table, Edit: *req.Edit})
		if err != nil {
			return err
		}
		if len(raw) > maxNetworkSnapshotBytes {
			return unsupportedNetworkScope()
		}
		record, err = m.approvals.CreatePrefixedVirtualSnapshot("snap_db_", scope.key, string(raw))
		if err != nil {
			return err
		}
		stored, err := m.readNetworkSnapshot(record.SnapshotID, scope.key)
		if err != nil {
			return err
		}
		return reconstructNetworkTable(ctx, tx, scope.engine, stored.Table)
	})
	if err != nil {
		return nil, networkRecoveryError(err)
	}
	approval, err := m.approvals.IssueSnapshotBoundToken(groxybridge.ActionDBMutate, mutationBinding(scope.key, operation), record.SnapshotID)
	if err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "engine": scope.engine, "target_label": scope.label, "target_key": scope.key,
		"statement_count": "1", "snapshot": record, "approval_token": approval.Token,
		"plain_summary": "Restored and checked a SHA-256 snapshot of the selected table's data and schema. Review the exact cell change."}, nil
}

func (m *Manager) applyNetworkCell(ctx context.Context, req MutateApplyRequest, operation string) (map[string]any, error) {
	if req.Edit == nil {
		return nil, m.fullNetworkWriteUnavailable(req.Target)
	}
	scope, err := m.networkScopeFor(req.Target, req.Edit.Table)
	if err != nil {
		return nil, err
	}
	approval, err := m.approvals.ConsumeToken(req.ApprovalToken, groxybridge.ActionDBMutate, mutationBinding(scope.key, operation))
	if err != nil {
		return nil, err
	}
	snap, err := m.readNetworkSnapshot(approval.SnapshotID, scope.key)
	if err != nil {
		return nil, err
	}
	err = m.withNetworkScope(ctx, req.Target, scope, func(ctx context.Context, tx *sql.Tx) error {
		before, err := snapshotNetworkTable(ctx, tx, scope)
		if err != nil {
			return err
		}
		if !bytes.Equal(networkTableJSON(before), networkTableJSON(snap.Table)) {
			return changedNetworkTarget()
		}
		after, err := changeNetworkCell(ctx, tx, scope, before, *req.Edit)
		if err != nil {
			return err
		}
		return m.approvals.WriteDBOutcome(approval.SnapshotID, scope.key, security.SHA256Hex(networkTableJSON(after)))
	})
	if err != nil {
		return nil, networkRecoveryError(err)
	}
	return map[string]any{"ok": true, "engine": scope.engine, "snapshot_id": approval.SnapshotID,
		"statements_executed": "1", "affected_rows": "1"}, nil
}

func (m *Manager) networkRestoration(req RestoreRequest) (networkScope, networkSnapshot, string, error) {
	identity, err := m.networkScopeFor(req.Target, "restore")
	if err != nil {
		return networkScope{}, networkSnapshot{}, "", err
	}
	snap, err := m.readNetworkSnapshot(req.SnapshotID, identity.key)
	if err != nil {
		return networkScope{}, networkSnapshot{}, "", err
	}
	scope, err := m.networkScopeFor(req.Target, snap.Edit.Table)
	if err != nil || scope.tunnel != identity.tunnel {
		return networkScope{}, networkSnapshot{}, "", changedNetworkTarget()
	}
	baseline, err := m.approvals.ReadDBOutcome(req.SnapshotID, identity.key)
	return scope, snap, baseline, err
}

func (m *Manager) networkRestore(ctx context.Context, req RestoreRequest, apply bool) (map[string]any, error) {
	scope, snap, baseline, err := m.networkRestoration(req)
	if err != nil {
		return nil, err
	}
	if req.Force {
		return m.forceNetworkTableRestore(ctx, req, scope, snap.Table, apply)
	}
	if req.BaselineSHA != "" {
		return nil, invalidCell("a reviewed baseline is only used for forced recovery")
	}
	binding := restoreBinding(scope.key, req.SnapshotID, baseline)
	if !apply {
		err := m.withNetworkScope(ctx, req.Target, scope, func(ctx context.Context, tx *sql.Tx) error {
			current, err := snapshotNetworkTable(ctx, tx, scope)
			if err == nil && security.SHA256Hex(networkTableJSON(current)) != baseline {
				return changedNetworkTarget()
			}
			return err
		})
		if err != nil {
			return nil, networkRecoveryError(err)
		}
		approval, err := m.approvals.IssueSnapshotBoundToken(groxybridge.ActionDBRestore, binding, req.SnapshotID)
		if err != nil {
			return nil, err
		}
		return map[string]any{"ok": true, "snapshot_id": req.SnapshotID, "target_label": scope.label,
			"approval_token": approval.Token, "plain_summary": scope.restoreSummary("Restore the previous cell value if the table still matches the recorded operation.")}, nil
	}
	if _, err := m.approvals.ConsumeToken(req.ApprovalToken, groxybridge.ActionDBRestore, binding); err != nil {
		return nil, err
	}
	err = m.withNetworkRestoreScope(ctx, req.Target, scope, true, func(ctx context.Context, tx *sql.Tx) error {
		current, err := snapshotNetworkTable(ctx, tx, scope)
		if err != nil {
			return err
		}
		if security.SHA256Hex(networkTableJSON(current)) != baseline {
			return changedNetworkTarget()
		}
		undo := snap.Edit
		undo.Before, undo.Value = undo.Value, undo.Before
		restored, err := changeNetworkCell(ctx, tx, scope, current, undo)
		if err != nil {
			return err
		}
		if !bytes.Equal(networkTableJSON(restored), networkTableJSON(snap.Table)) {
			return fmt.Errorf("restored table differs from the verified snapshot")
		}
		return nil
	})
	if err != nil {
		return nil, networkRecoveryError(err)
	}
	return m.finishNetworkRestore(req, scope, security.SHA256Hex(networkTableJSON(snap.Table)))
}

func sameNetworkSchema(a, b networkTable) bool {
	a.Rows, b.Rows = nil, nil
	return bytes.Equal(networkTableJSON(a), networkTableJSON(b))
}

func (m *Manager) forceNetworkTableRestore(ctx context.Context, req RestoreRequest, scope networkScope, saved networkTable, apply bool) (map[string]any, error) {
	if apply {
		binding := restoreBinding(scope.key, req.SnapshotID, "force:"+req.BaselineSHA)
		if _, err := m.approvals.ConsumeToken(req.ApprovalToken, groxybridge.ActionDBRestore, binding); err != nil {
			return nil, err
		}
	}
	var baseline string
	digest := security.SHA256Hex(networkTableJSON(saved))
	err := m.withNetworkRestoreScope(ctx, req.Target, scope, apply, func(ctx context.Context, tx *sql.Tx) error {
		current, err := snapshotNetworkTable(ctx, tx, scope)
		if err != nil {
			return err
		}
		if !sameNetworkSchema(current, saved) {
			return changedNetworkTarget()
		}
		baseline = security.SHA256Hex(networkTableJSON(current))
		if !apply {
			return nil
		}
		if baseline != req.BaselineSHA {
			return changedNetworkTarget()
		}
		if _, err := tx.ExecContext(ctx, "DELETE FROM "+scope.tableSQL()); err != nil {
			return err
		}
		if err := insertNetworkRows(ctx, tx, scope.engine, scope.tableSQL(), saved); err != nil {
			return err
		}
		restored, err := snapshotNetworkTable(ctx, tx, scope)
		if err != nil {
			return err
		}
		if security.SHA256Hex(networkTableJSON(restored)) != digest {
			return fmt.Errorf("restored table differs from the verified snapshot")
		}
		return nil
	})
	if err != nil {
		return nil, networkRecoveryError(err)
	}
	if apply {
		return m.finishNetworkRestore(req, scope, digest)
	}
	binding := restoreBinding(scope.key, req.SnapshotID, "force:"+baseline)
	approval, err := m.approvals.IssueSnapshotBoundToken(groxybridge.ActionDBRestore, binding, req.SnapshotID)
	if err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "snapshot_id": req.SnapshotID, "target_label": scope.label, "baseline_sha256": baseline,
		"approval_token": approval.Token, "force": true,
		"plain_summary": scope.restoreSummary(fmt.Sprintf("Replace every row in %s with the saved %d rows, overwriting newer edits to that table.", scope.label, len(saved.Rows)))}, nil
}

func (s networkScope) restoreSummary(summary string) string {
	if s.engine == "postgres" {
		return summary + " Desk temporarily closes PostgreSQL connections to this database and rolls back other sessions' uncommitted work during recovery."
	}
	if s.engine == "mariadb" && strings.HasSuffix(s.key, ":full") {
		return summary + " Desk temporarily revokes other accounts' access to this database and ends their open sessions during recovery, then restores their exact grants."
	}
	return summary
}

func (m *Manager) finishNetworkRestore(req RestoreRequest, scope networkScope, digest string) (map[string]any, error) {
	if err := m.approvals.RecordDBRestore(req.SnapshotID, scope.key, digest, req.Force); err != nil {
		return nil, security.NewError(security.CodeDeskE502, 502, "database restore committed and verified but the snapshot record could not be updated", err)
	}
	return map[string]any{"ok": true, "status": "restored", "engine": scope.engine, "snapshot_id": req.SnapshotID,
		"restored_sha256": digest, "forced": req.Force}, nil
}
