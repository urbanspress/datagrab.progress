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
	"sort"
	"strings"

	"groxy-desk/internal/security"
)

// A MariaDB logical image is a deterministic JSON document of every table
// definition, row, view and trigger in one database. It is built from server
// definitions (SHOW CREATE) and exact text or hexadecimal row values, so a
// replay can be verified byte for byte by capturing the database again.

const (
	mariaImageFormat = 1
	maxMariaImageRow = 200000
)

type mariaImage struct {
	Format   int               `json:"format"`
	Database string            `json:"database"`
	Tables   []mariaTableImage `json:"tables"`
	Views    []mariaDefinition `json:"views"`
	Triggers []mariaTrigger    `json:"triggers"`
}

type mariaTableImage struct {
	Name    string      `json:"name"`
	Create  string      `json:"create"`
	Columns []string    `json:"columns"`
	Binary  []bool      `json:"binary"`
	Rows    [][]*string `json:"rows"`
}

type mariaDefinition struct {
	Name   string `json:"name"`
	Create string `json:"create"`
}

type mariaTrigger struct {
	Name    string `json:"name"`
	Table   string `json:"table"`
	SQLMode string `json:"sql_mode"`
	Create  string `json:"create"`
}

var mariaModeText = regexp.MustCompile(`^[A-Z_,]*$`)

func mariaUnsupportedType(dataType string) bool {
	switch dataType {
	case "float", "double", "real", "bit", "geometry", "point", "linestring", "polygon", "multipoint", "multilinestring",
		"multipolygon", "geometrycollection":
		return true
	}
	return false
}

func mariaBinaryType(dataType string) bool {
	switch dataType {
	case "binary", "varbinary", "tinyblob", "blob", "mediumblob", "longblob":
		return true
	}
	return false
}

func mariaTable(scope networkScope, name string) string {
	return quoteIdent("mariadb", scope.database) + "." + quoteIdent("mariadb", name)
}

// mariaLogicalImage reads the whole target database in one read-only snapshot.
func mariaLogicalImage(ctx context.Context, conn *sql.Conn, scope networkScope) ([]byte, error) {
	tx, err := conn.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var selected, definer string
	if err := tx.QueryRowContext(ctx, "SELECT COALESCE(DATABASE(), ''), CURRENT_USER()").Scan(&selected, &definer); err != nil || selected != scope.database {
		return nil, errors.Join(unsupportedNetworkScope(), err)
	}
	image := mariaImage{Format: mariaImageFormat, Database: scope.database, Tables: []mariaTableImage{}, Views: []mariaDefinition{}, Triggers: []mariaTrigger{}}

	var objects int
	if err := tx.QueryRowContext(ctx, `SELECT (SELECT COUNT(*) FROM information_schema.ROUTINES WHERE ROUTINE_SCHEMA = ?)
		+ (SELECT COUNT(*) FROM information_schema.EVENTS WHERE EVENT_SCHEMA = ?)`, scope.database, scope.database).Scan(&objects); err != nil {
		return nil, err
	}
	if objects != 0 {
		return nil, unsupportedNetworkScope()
	}
	rows, err := tx.QueryContext(ctx, `SELECT TABLE_NAME, TABLE_TYPE, COALESCE(ENGINE, '') FROM information_schema.TABLES WHERE TABLE_SCHEMA = ?`, scope.database)
	if err != nil {
		return nil, err
	}
	var tables, views []string
	for rows.Next() {
		var name, kind, engine string
		if err := rows.Scan(&name, &kind, &engine); err != nil {
			rows.Close()
			return nil, err
		}
		switch {
		case kind == "BASE TABLE" && slices.Contains([]string{"InnoDB", "MyISAM", "Aria"}, engine):
			tables = append(tables, name)
		case kind == "VIEW":
			views = append(views, name)
		default:
			rows.Close()
			return nil, unsupportedNetworkScope()
		}
	}
	if err := errors.Join(rows.Err(), rows.Close()); err != nil {
		return nil, err
	}
	sort.Strings(tables)
	sort.Strings(views)

	type column struct {
		name   string
		binary bool
		key    bool
	}
	columns := map[string][]column{}
	rows, err = tx.QueryContext(ctx, `SELECT c.TABLE_NAME, c.COLUMN_NAME, c.DATA_TYPE, c.EXTRA, c.COLUMN_KEY = 'PRI'
		FROM information_schema.COLUMNS c JOIN information_schema.TABLES t
		ON t.TABLE_SCHEMA = c.TABLE_SCHEMA AND t.TABLE_NAME = c.TABLE_NAME
		WHERE c.TABLE_SCHEMA = ? AND t.TABLE_TYPE = 'BASE TABLE' ORDER BY c.TABLE_NAME, c.ORDINAL_POSITION`, scope.database)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var table, name, dataType, extra string
		var key bool
		if err := rows.Scan(&table, &name, &dataType, &extra, &key); err != nil {
			rows.Close()
			return nil, err
		}
		if strings.Contains(strings.ToUpper(extra), "GENERATED") {
			continue
		}
		if mariaUnsupportedType(strings.ToLower(dataType)) {
			rows.Close()
			return nil, unsupportedNetworkScope()
		}
		columns[table] = append(columns[table], column{name, mariaBinaryType(strings.ToLower(dataType)), key})
	}
	if err := errors.Join(rows.Err(), rows.Close()); err != nil {
		return nil, err
	}

	total, totalRows := 0, 0
	for _, name := range tables {
		cols := columns[name]
		if len(cols) == 0 {
			return nil, unsupportedNetworkScope()
		}
		table := mariaTableImage{Name: name, Rows: [][]*string{}}
		var shown, create string
		if err := tx.QueryRowContext(ctx, "SHOW CREATE TABLE "+mariaTable(scope, name)).Scan(&shown, &create); err != nil || shown != name ||
			!strings.HasPrefix(create, "CREATE TABLE "+quoteIdent("mariadb", name)+" (") {
			return nil, errors.Join(unsupportedNetworkScope(), err)
		}
		table.Create = create
		expressions := make([]string, len(cols))
		var order, all []string
		for i, col := range cols {
			quoted := quoteIdent("mariadb", col.name)
			table.Columns = append(table.Columns, col.name)
			table.Binary = append(table.Binary, col.binary)
			expressions[i] = "CAST(" + quoted + " AS CHAR CHARACTER SET utf8mb4)"
			if col.binary {
				expressions[i] = "HEX(" + quoted + ")"
			}
			all = append(all, quoted)
			if col.key {
				order = append(order, quoted)
			}
		}
		if len(order) == 0 {
			order = all
		}
		data, err := tx.QueryContext(ctx, "SELECT "+strings.Join(expressions, ",")+" FROM "+mariaTable(scope, name)+" ORDER BY "+strings.Join(order, ","))
		if err != nil {
			return nil, err
		}
		for data.Next() {
			row, dest := make([]*string, len(cols)), make([]any, len(cols))
			for i := range row {
				dest[i] = &row[i]
			}
			if err := data.Scan(dest...); err != nil {
				data.Close()
				return nil, err
			}
			totalRows++
			for _, value := range row {
				if value != nil {
					total += len(*value)
				}
			}
			if totalRows > maxMariaImageRow || total > maxNetworkSnapshotBytes/2 {
				data.Close()
				return nil, unsupportedNetworkScope()
			}
			table.Rows = append(table.Rows, row)
		}
		if err := errors.Join(data.Err(), data.Close()); err != nil {
			return nil, err
		}
		image.Tables = append(image.Tables, table)
	}

	definers := map[string]string{}
	rows, err = tx.QueryContext(ctx, `SELECT TABLE_NAME, DEFINER FROM information_schema.VIEWS WHERE TABLE_SCHEMA = ?`, scope.database)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var name, owner string
		if err := rows.Scan(&name, &owner); err != nil {
			rows.Close()
			return nil, err
		}
		definers[name] = owner
	}
	if err := errors.Join(rows.Err(), rows.Close()); err != nil {
		return nil, err
	}
	for _, name := range views {
		var shown, create, charset, collation string
		if definers[name] != definer {
			return nil, unsupportedNetworkScope()
		}
		if err := tx.QueryRowContext(ctx, "SHOW CREATE VIEW "+mariaTable(scope, name)).Scan(&shown, &create, &charset, &collation); err != nil || shown != name ||
			!strings.HasPrefix(create, "CREATE ") || !strings.Contains(create, " VIEW "+quoteIdent("mariadb", name)+" AS ") {
			return nil, errors.Join(unsupportedNetworkScope(), err)
		}
		image.Views = append(image.Views, mariaDefinition{Name: name, Create: create})
	}

	type triggerKey struct{ name, table, owner string }
	var triggers []triggerKey
	rows, err = tx.QueryContext(ctx, `SELECT TRIGGER_NAME, EVENT_OBJECT_TABLE, DEFINER FROM information_schema.TRIGGERS WHERE TRIGGER_SCHEMA = ?
		ORDER BY EVENT_OBJECT_TABLE, ACTION_TIMING, EVENT_MANIPULATION, ACTION_ORDER, TRIGGER_NAME`, scope.database)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var key triggerKey
		if err := rows.Scan(&key.name, &key.table, &key.owner); err != nil {
			rows.Close()
			return nil, err
		}
		triggers = append(triggers, key)
	}
	if err := errors.Join(rows.Err(), rows.Close()); err != nil {
		return nil, err
	}
	for _, key := range triggers {
		if key.owner != definer || !slices.Contains(tables, key.table) {
			return nil, unsupportedNetworkScope()
		}
		out, err := tx.QueryContext(ctx, "SHOW CREATE TRIGGER "+mariaTable(scope, key.name))
		if err != nil {
			return nil, err
		}
		fields, err := out.Columns()
		if err != nil || len(fields) < 3 || !out.Next() {
			out.Close()
			return nil, errors.Join(unsupportedNetworkScope(), err)
		}
		values := make([]sql.NullString, len(fields))
		dest := make([]any, len(fields))
		for i := range values {
			dest[i] = &values[i]
		}
		if err := out.Scan(dest...); err != nil {
			out.Close()
			return nil, err
		}
		if err := errors.Join(out.Err(), out.Close()); err != nil {
			return nil, err
		}
		create := values[2].String
		if values[0].String != key.name || !mariaModeText.MatchString(values[1].String) || !validMariaTriggerCreate(scope.database, key.name, create) {
			return nil, unsupportedNetworkScope()
		}
		image.Triggers = append(image.Triggers, mariaTrigger{Name: key.name, Table: key.table, SQLMode: values[1].String, Create: create})
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	raw, err := json.Marshal(image)
	if err != nil {
		return nil, err
	}
	if len(raw) > maxNetworkSnapshotBytes {
		return nil, unsupportedNetworkScope()
	}
	return raw, nil
}

// parseMariaImage verifies the structure Desk will execute during a replay.
func parseMariaImage(raw []byte, scope networkScope) (mariaImage, error) {
	var image mariaImage
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&image); err != nil || image.Format != mariaImageFormat || image.Database != scope.database {
		return image, networkRecoveryError(fmt.Errorf("invalid MariaDB logical snapshot"))
	}
	bad := func() (mariaImage, error) {
		return image, networkRecoveryError(fmt.Errorf("invalid MariaDB logical snapshot"))
	}
	names := map[string]bool{}
	for _, table := range image.Tables {
		if names[table.Name] || !strings.HasPrefix(table.Create, "CREATE TABLE "+quoteIdent("mariadb", table.Name)+" (") ||
			len(table.Columns) == 0 || len(table.Columns) != len(table.Binary) {
			return bad()
		}
		names[table.Name] = true
		for _, row := range table.Rows {
			if len(row) != len(table.Columns) {
				return bad()
			}
		}
	}
	for _, view := range image.Views {
		if names[view.Name] || !strings.HasPrefix(view.Create, "CREATE ") || !strings.Contains(view.Create, " VIEW "+quoteIdent("mariadb", view.Name)+" AS ") {
			return bad()
		}
		names[view.Name] = true
	}
	for _, trigger := range image.Triggers {
		if !mariaModeText.MatchString(trigger.SQLMode) || !validMariaTriggerCreate(scope.database, trigger.Name, trigger.Create) {
			return bad()
		}
	}
	return image, nil
}

func validMariaTriggerCreate(database, name, create string) bool {
	if validateIdentifier(name) != nil || !strings.HasPrefix(create, "CREATE ") {
		return false
	}
	quoted := quoteIdent("mariadb", name)
	dbPrefix := quoteIdent("mariadb", database) + "."
	return strings.Contains(create, " TRIGGER "+quoted+" ") ||
		strings.Contains(create, " TRIGGER "+name+" ") ||
		strings.Contains(create, " TRIGGER "+dbPrefix+quoted+" ") ||
		strings.Contains(create, " TRIGGER "+database+"."+name+" ")
}

// mariaReplayState tells the caller whether a failed replay left the target
// changed. DDL is not transactional, so a changed target cannot be rolled back.
type mariaReplayFailure struct {
	err error
}

func (f mariaReplayFailure) Error() string { return f.err.Error() }
func (f mariaReplayFailure) Unwrap() []error {
	return []error{f.err, errMariaKeepOff}
}

func insertMariaRows(ctx context.Context, conn *sql.Conn, scope networkScope, table mariaTableImage) error {
	if len(table.Rows) == 0 {
		return nil
	}
	names, params := make([]string, len(table.Columns)), make([]string, len(table.Columns))
	for i, name := range table.Columns {
		names[i], params[i] = quoteIdent("mariadb", name), "?"
		if table.Binary[i] {
			params[i] = "UNHEX(?)"
		}
	}
	statement, err := conn.PrepareContext(ctx, "INSERT INTO "+mariaTable(scope, table.Name)+" ("+strings.Join(names, ",")+") VALUES ("+strings.Join(params, ",")+")")
	if err != nil {
		return err
	}
	defer statement.Close()
	for _, row := range table.Rows {
		args := make([]any, len(row))
		for i, value := range row {
			args[i] = value
		}
		if _, err := statement.ExecContext(ctx, args...); err != nil {
			return err
		}
	}
	return nil
}

// mariaObjects lists the tables and views in the target without interpreting
// them, so a replay can discard a state that cannot be captured.
func mariaObjects(ctx context.Context, conn *sql.Conn, scope networkScope) (tables, views []string, err error) {
	rows, err := conn.QueryContext(ctx, `SELECT TABLE_NAME, TABLE_TYPE FROM information_schema.TABLES WHERE TABLE_SCHEMA = ? ORDER BY TABLE_NAME`, scope.database)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var name, kind string
		if err := rows.Scan(&name, &kind); err != nil {
			return nil, nil, err
		}
		if kind == "VIEW" {
			views = append(views, name)
		} else {
			tables = append(tables, name)
		}
	}
	return tables, views, rows.Err()
}

// replayMariaImage replaces the database contents with a saved image. With
// verifyBaseline it first proves the database still matches the baseline, and
// afterwards it always verifies the result. Once the first object is dropped,
// any failure keeps admission closed.
func replayMariaImage(ctx context.Context, conn *sql.Conn, scope networkScope, baseline string, saved []byte, verifyBaseline bool) error {
	image, err := parseMariaImage(saved, scope)
	if err != nil {
		return err
	}
	if verifyBaseline {
		current, err := mariaLogicalImage(ctx, conn, scope)
		if err != nil {
			return err
		}
		if security.SHA256Hex(current) != baseline {
			return changedNetworkTarget()
		}
	}
	tables, views, err := mariaObjects(ctx, conn, scope)
	if err != nil {
		return err
	}
	fail := func(err error) error { return mariaReplayFailure{err} }
	for _, statement := range []string{"SET SESSION sql_mode = '" + mariaSessionMode + "'", "SET SESSION foreign_key_checks = 0"} {
		if _, err := conn.ExecContext(ctx, statement); err != nil {
			return err
		}
	}
	for _, view := range views {
		if _, err := conn.ExecContext(ctx, "DROP VIEW IF EXISTS "+mariaTable(scope, view)); err != nil {
			return fail(err)
		}
	}
	for _, table := range tables {
		if _, err := conn.ExecContext(ctx, "DROP TABLE IF EXISTS "+mariaTable(scope, table)); err != nil {
			return fail(err)
		}
	}
	for _, table := range image.Tables {
		if _, err := conn.ExecContext(ctx, table.Create); err != nil {
			return fail(err)
		}
		if err := insertMariaRows(ctx, conn, scope, table); err != nil {
			return fail(err)
		}
	}
	// Views may depend on one another. Create what resolves until none remain.
	pending := slices.Clone(image.Views)
	for len(pending) > 0 {
		var unresolved []mariaDefinition
		var last error
		for _, view := range pending {
			if _, err := conn.ExecContext(ctx, view.Create); err != nil {
				unresolved, last = append(unresolved, view), err
			}
		}
		if len(unresolved) == len(pending) {
			return fail(last)
		}
		pending = unresolved
	}
	for _, trigger := range image.Triggers {
		if _, err := conn.ExecContext(ctx, "SET SESSION sql_mode = '"+trigger.SQLMode+"'"); err != nil {
			return fail(err)
		}
		if _, err := conn.ExecContext(ctx, trigger.Create); err != nil {
			return fail(err)
		}
	}
	for _, statement := range []string{"SET SESSION sql_mode = '" + mariaSessionMode + "'", "SET SESSION foreign_key_checks = 1"} {
		if _, err := conn.ExecContext(ctx, statement); err != nil {
			return fail(err)
		}
	}
	actual, err := mariaLogicalImage(ctx, conn, scope)
	if err != nil {
		return fail(err)
	}
	if !bytes.Equal(actual, saved) {
		return fail(errors.New("the replayed MariaDB database differs from the saved logical snapshot"))
	}
	return nil
}
