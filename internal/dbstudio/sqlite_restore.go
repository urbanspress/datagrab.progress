package dbstudio

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
)

type sqliteObject struct {
	kind string
	name string
	ddl  string
}

func sqliteObjects(ctx context.Context, conn *sql.Conn, schema string) ([]sqliteObject, error) {
	rows, err := conn.QueryContext(ctx, "SELECT type,name,sql FROM "+schema+".sqlite_schema WHERE sql IS NOT NULL AND (substr(name,1,7) != 'sqlite_' OR name='sqlite_sequence') ORDER BY type,name COLLATE BINARY")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var objects []sqliteObject
	for rows.Next() {
		var obj sqliteObject
		if err := rows.Scan(&obj.kind, &obj.name, &obj.ddl); err != nil {
			return nil, err
		}
		switch obj.kind {
		case "table":
			if !strings.HasPrefix(strings.ToUpper(obj.ddl), "CREATE TABLE ") {
				return nil, fmt.Errorf("SQLite table %q is outside the supported recovery scope", obj.name)
			}
		case "index", "trigger", "view":
		default:
			return nil, fmt.Errorf("unsupported SQLite schema object %q", obj.kind)
		}
		objects = append(objects, obj)
	}
	return objects, rows.Err()
}

func sqliteIdent(name string) string { return `"` + strings.ReplaceAll(name, `"`, `""`) + `"` }

func sqliteCopyColumns(ctx context.Context, conn *sql.Conn, table string) ([]string, error) {
	var withoutRowID int
	if err := conn.QueryRowContext(ctx, "SELECT wr FROM pragma_table_list WHERE schema='desk_backup' AND name=?", table).Scan(&withoutRowID); err != nil {
		return nil, err
	}
	rows, err := conn.QueryContext(ctx, "SELECT name,hidden FROM pragma_table_xinfo(?, 'desk_backup') ORDER BY cid", table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var columns []string
	names := make(map[string]bool)
	for rows.Next() {
		var name string
		var hidden int
		if err := rows.Scan(&name, &hidden); err != nil {
			return nil, err
		}
		names[strings.ToLower(name)] = true
		if hidden == 0 {
			columns = append(columns, sqliteIdent(name))
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if withoutRowID != 0 || table == "sqlite_sequence" {
		return columns, nil
	}
	for _, candidate := range []string{"rowid", "_rowid_", "oid"} {
		if !names[candidate] {
			return append(columns, candidate), nil
		}
	}
	return nil, fmt.Errorf("SQLite table %q has no recoverable rowid alias", table)
}

func verifySQLiteRows(ctx context.Context, conn *sql.Conn, table string, cols []string) error {
	// SQLite's quote() truncates text with embedded NULs. Compare storage types
	// and full byte encodings, including multiplicities in WITHOUT ROWID tables.
	expressions := make([]string, len(cols))
	for i, col := range cols {
		expressions[i] = "typeof(" + col + ") || ':' || CASE typeof(" + col + ") WHEN 'real' THEN printf('%!.26g'," + col + ") ELSE hex(" + col + ") END"
	}
	expr := strings.Join(expressions, ",")
	identity := sqliteIdent(table)
	left := "SELECT " + expr + ",COUNT(*) FROM main." + identity + " GROUP BY " + expr
	right := "SELECT " + expr + ",COUNT(*) FROM desk_backup." + identity + " GROUP BY " + expr
	query := "SELECT EXISTS(SELECT * FROM (" + left + " EXCEPT " + right + ") UNION ALL SELECT * FROM (" + right + " EXCEPT " + left + "))"
	var differs bool
	if err := conn.QueryRowContext(ctx, query).Scan(&differs); err != nil {
		return err
	}
	if differs {
		return fmt.Errorf("restored SQLite rows differ for %q", table)
	}
	return nil
}

func restoreSQLiteContents(ctx context.Context, conn *sql.Conn, backup string) error {
	if _, err := conn.ExecContext(ctx, "ATTACH DATABASE ? AS desk_backup", sqliteURI(backup, "ro")); err != nil {
		return err
	}
	current, err := sqliteObjects(ctx, conn, "main")
	if err != nil {
		return err
	}
	expected, err := sqliteObjects(ctx, conn, "desk_backup")
	if err != nil {
		return err
	}
	for _, kind := range []string{"trigger", "view", "table"} {
		for _, obj := range current {
			if obj.kind != kind || obj.name == "sqlite_sequence" {
				continue
			}
			if _, err := conn.ExecContext(ctx, "DROP "+kind+" main."+sqliteIdent(obj.name)); err != nil {
				return err
			}
		}
	}
	var tables []sqliteObject
	var sequenceExpected bool
	for _, obj := range expected {
		if obj.kind != "table" {
			continue
		}
		if obj.name == "sqlite_sequence" {
			sequenceExpected = true
			continue
		}
		tables = append(tables, obj)
	}
	if sequenceExpected {
		// A snapshot can contain sqlite_sequence after all its AUTOINCREMENT
		// tables were dropped. Initialize it before restoring application tables.
		if _, err := conn.ExecContext(ctx, `CREATE TABLE main.desk_restore_sequence (id INTEGER PRIMARY KEY AUTOINCREMENT); DROP TABLE main.desk_restore_sequence;`); err != nil {
			return err
		}
	}
	for _, obj := range tables {
		if _, err := conn.ExecContext(ctx, obj.ddl); err != nil {
			return err
		}
	}
	if sequenceExpected {
		tables = append(tables, sqliteObject{name: "sqlite_sequence"})
	}
	for _, obj := range tables {
		table := sqliteIdent(obj.name)
		if obj.name == "sqlite_sequence" {
			if _, err := conn.ExecContext(ctx, "DELETE FROM main.sqlite_sequence"); err != nil {
				return err
			}
		}
		cols, err := sqliteCopyColumns(ctx, conn, obj.name)
		if err != nil {
			return err
		}
		columns := strings.Join(cols, ",")
		if _, err := conn.ExecContext(ctx, "INSERT INTO main."+table+"("+columns+") SELECT "+columns+" FROM desk_backup."+table); err != nil {
			return err
		}
		if err := verifySQLiteRows(ctx, conn, obj.name, cols); err != nil {
			return err
		}
	}
	for _, kind := range []string{"index", "view", "trigger"} {
		for _, obj := range expected {
			if obj.kind == kind {
				if _, err := conn.ExecContext(ctx, obj.ddl); err != nil {
					return err
				}
			}
		}
	}
	for _, field := range []string{"user_version", "application_id"} {
		var value int64
		if err := conn.QueryRowContext(ctx, "PRAGMA desk_backup."+field).Scan(&value); err != nil {
			return err
		}
		if _, err := conn.ExecContext(ctx, "PRAGMA main."+field+"="+strconv.FormatInt(value, 10)); err != nil {
			return err
		}
	}
	restored, err := sqliteObjects(ctx, conn, "main")
	if err != nil {
		return err
	}
	// SQLite retains its internal sequence table after the last AUTOINCREMENT
	// table is dropped. It has no application rows in that case.
	if !sequenceExpected {
		for i, obj := range restored {
			if obj.name == "sqlite_sequence" {
				var entries int
				if err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM main.sqlite_sequence`).Scan(&entries); err != nil {
					return err
				}
				if entries != 0 {
					return fmt.Errorf("unexpected rows in restored SQLite sequence")
				}
				restored = append(restored[:i], restored[i+1:]...)
				break
			}
		}
	}
	if len(restored) != len(expected) {
		return fmt.Errorf("restored SQLite schema object count differs")
	}
	for i, obj := range expected {
		if restored[i] != obj {
			return fmt.Errorf("restored SQLite schema differs for %q", obj.name)
		}
	}
	return sqliteIntegrity(ctx, conn, "main")
}

func verifySQLiteRestore(ctx context.Context, backup string) error {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		return err
	}
	defer db.Close()
	conn, err := db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		return err
	}
	defer conn.ExecContext(context.Background(), "ROLLBACK")
	if err := restoreSQLiteContents(ctx, conn, backup); err != nil {
		return err
	}
	_, err = conn.ExecContext(ctx, "COMMIT")
	return err
}
