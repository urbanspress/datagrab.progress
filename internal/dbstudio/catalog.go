package dbstudio

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"sort"
	"strings"
)

const postgresColumns = `
SELECT c.relname, a.attname, format_type(a.atttypid, a.atttypmod), NOT a.attnotnull,
       EXISTS (SELECT 1 FROM pg_catalog.pg_index i WHERE i.indrelid=c.oid AND i.indisprimary AND a.attnum=ANY(i.indkey)),
       COALESCE(pg_get_expr(d.adbin,d.adrelid),'')
FROM pg_catalog.pg_class c
JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace
JOIN pg_catalog.pg_attribute a ON a.attrelid=c.oid AND a.attnum>0 AND NOT a.attisdropped
LEFT JOIN pg_catalog.pg_attrdef d ON d.adrelid=c.oid AND d.adnum=a.attnum
WHERE n.nspname=$1 AND c.relkind IN ('r','p')
ORDER BY c.relname, a.attnum`

const postgresKeys = `
SELECT src.relname, a.attname, nt.nspname, dst.relname, b.attname
FROM pg_catalog.pg_constraint k
JOIN pg_catalog.pg_class src ON src.oid=k.conrelid
JOIN pg_catalog.pg_namespace ns ON ns.oid=src.relnamespace
JOIN pg_catalog.pg_class dst ON dst.oid=k.confrelid
JOIN pg_catalog.pg_namespace nt ON nt.oid=dst.relnamespace
CROSS JOIN LATERAL unnest(k.conkey,k.confkey) WITH ORDINALITY AS cols(source,target,pos)
JOIN pg_catalog.pg_attribute a ON a.attrelid=src.oid AND a.attnum=cols.source
JOIN pg_catalog.pg_attribute b ON b.attrelid=dst.oid AND b.attnum=cols.target
WHERE k.contype='f' AND ns.nspname=$1
ORDER BY src.relname,k.conname,cols.pos`

const mariaColumns = `
SELECT c.TABLE_NAME,c.COLUMN_NAME,c.COLUMN_TYPE,c.IS_NULLABLE='YES',c.COLUMN_KEY='PRI',COALESCE(c.COLUMN_DEFAULT,'')
FROM information_schema.COLUMNS c JOIN information_schema.TABLES t
 ON t.TABLE_SCHEMA=c.TABLE_SCHEMA AND t.TABLE_NAME=c.TABLE_NAME
WHERE c.TABLE_SCHEMA=? AND t.TABLE_TYPE='BASE TABLE'
ORDER BY c.TABLE_NAME,c.ORDINAL_POSITION`

const mariaKeys = `
SELECT TABLE_NAME,COLUMN_NAME,REFERENCED_TABLE_SCHEMA,REFERENCED_TABLE_NAME,REFERENCED_COLUMN_NAME
FROM information_schema.KEY_COLUMN_USAGE
WHERE TABLE_SCHEMA=? AND REFERENCED_TABLE_NAME IS NOT NULL
ORDER BY TABLE_NAME,CONSTRAINT_NAME,ORDINAL_POSITION`

func inspectPostgresSchema(ctx context.Context, db *sql.DB, schema string) ([]TableSchema, error) {
	return inspectNetworkCatalog(ctx, db, "postgres", schema, postgresColumns, postgresKeys,
		"SELECT tablename,indexname FROM pg_catalog.pg_indexes WHERE schemaname=$1 ORDER BY tablename,indexname")
}

func inspectMariaDBSchema(ctx context.Context, db *sql.DB, schema string) ([]TableSchema, error) {
	return inspectNetworkCatalog(ctx, db, "mariadb", schema, mariaColumns, mariaKeys,
		"SELECT DISTINCT TABLE_NAME,INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA=? ORDER BY TABLE_NAME,INDEX_NAME")
}

func catalogRows(ctx context.Context, tx *sql.Tx, query, schema string, read func(*sql.Rows) error) error {
	rows, err := tx.QueryContext(ctx, query, schema)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		if err := read(rows); err != nil {
			return err
		}
	}
	return rows.Err()
}

func inspectNetworkCatalog(ctx context.Context, db *sql.DB, engine, schema, columns, keys, indexes string) ([]TableSchema, error) {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	tables := make(map[string]*TableSchema)
	err = catalogRows(ctx, tx, columns, schema, func(rows *sql.Rows) error {
		var name string
		var col ColumnMeta
		if err := rows.Scan(&name, &col.Name, &col.Type, &col.Nullable, &col.PrimaryKey, &col.DefaultVal); err != nil {
			return err
		}
		if tables[name] == nil {
			tables[name] = &TableSchema{Name: name, Schema: schema}
		}
		tables[name].Columns = append(tables[name].Columns, col)
		return nil
	})
	if err != nil {
		return nil, err
	}
	err = catalogRows(ctx, tx, keys, schema, func(rows *sql.Rows) error {
		var name string
		var key ForeignKeyMeta
		if err := rows.Scan(&name, &key.Column, &key.RefSchema, &key.RefTable, &key.RefColumn); err != nil {
			return err
		}
		if table := tables[name]; table != nil {
			table.ForeignKeys = append(table.ForeignKeys, key)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	err = catalogRows(ctx, tx, indexes, schema, func(rows *sql.Rows) error {
		var name, index string
		if err := rows.Scan(&name, &index); err != nil {
			return err
		}
		if table := tables[name]; table != nil {
			table.Indexes = append(table.Indexes, index)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(tables))
	for name := range tables {
		names = append(names, name)
	}
	sort.Strings(names)
	out := make([]TableSchema, 0, len(names))
	for _, name := range names {
		table := tables[name]
		qualified := quoteIdent(engine, schema) + "." + quoteIdent(engine, name)
		if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+qualified).Scan(&table.RowCount); err != nil {
			return nil, err
		}
		out = append(out, *table)
	}
	return out, tx.Commit()
}

type CompareRequest struct {
	Source      TargetSpec `json:"source"`
	Destination TargetSpec `json:"destination"`
}

type SchemaChange struct {
	Table       string `json:"table"`
	Object      string `json:"object"`
	Kind        string `json:"kind"`
	Source      any    `json:"source,omitempty"`
	Destination any    `json:"destination,omitempty"`
	ProposedSQL string `json:"proposed_sql,omitempty"`
	Review      string `json:"review,omitempty"`
}

// CompareSchema reads the two catalogs independently and describes differences.
func (m *Manager) CompareSchema(ctx context.Context, r CompareRequest) (map[string]any, error) {
	source, err := m.GetSchema(ctx, r.Source)
	if err != nil {
		return nil, err
	}
	destination, err := m.GetSchema(ctx, r.Destination)
	if err != nil {
		return nil, err
	}
	left, lok := source["tables"].([]TableSchema)
	right, rok := destination["tables"].([]TableSchema)
	if !lok || !rok {
		return nil, errors.New("database catalogs unavailable")
	}
	changes := make([]SchemaChange, 0)
	sourceByName := map[string]TableSchema{}
	destByName := map[string]TableSchema{}
	for _, t := range left {
		sourceByName[t.Name] = t
	}
	for _, t := range right {
		destByName[t.Name] = t
	}
	for _, t := range left {
		other, exists := destByName[t.Name]
		if !exists {
			changes = append(changes, SchemaChange{Table: t.Name, Kind: "table_missing_in_destination", Source: t})
			continue
		}
		columns := map[string]ColumnMeta{}
		targetColumns := map[string]ColumnMeta{}
		for _, c := range t.Columns {
			columns[c.Name] = c
		}
		for _, c := range other.Columns {
			targetColumns[c.Name] = c
		}
		for _, c := range t.Columns {
			d, exists := targetColumns[c.Name]
			switch {
			case !exists:
				changes = append(changes, SchemaChange{Table: t.Name, Object: c.Name, Kind: "column_missing_in_destination", Source: c})
			case c != d:
				changes = append(changes, SchemaChange{Table: t.Name, Object: c.Name, Kind: "column_definition_differs", Source: c, Destination: d})
			}
		}
		for _, c := range other.Columns {
			if _, exists := columns[c.Name]; !exists {
				changes = append(changes, SchemaChange{Table: t.Name, Object: c.Name, Kind: "column_only_in_destination", Destination: c})
			}
		}
		if !reflect.DeepEqual(t.Indexes, other.Indexes) && len(t.Indexes)+len(other.Indexes) > 0 {
			changes = append(changes, SchemaChange{Table: t.Name, Kind: "index_names_differ", Source: t.Indexes, Destination: other.Indexes})
		}
		if !reflect.DeepEqual(t.ForeignKeys, other.ForeignKeys) && len(t.ForeignKeys)+len(other.ForeignKeys) > 0 {
			changes = append(changes, SchemaChange{Table: t.Name, Kind: "foreign_keys_differ", Source: t.ForeignKeys, Destination: other.ForeignKeys})
		}
	}
	for _, t := range right {
		if _, exists := sourceByName[t.Name]; !exists {
			changes = append(changes, SchemaChange{Table: t.Name, Kind: "table_only_in_destination", Destination: t})
		}
	}
	if engine := normalizeEngine(r.Destination.Engine); engine == "postgres" || engine == "mariadb" {
		schema := firstNonEmpty(r.Destination.SchemaName, "public")
		if engine == "mariadb" {
			schema = r.Destination.DBName
		}
		types := map[string]string{"text": "text", "integer": "integer", "smallint": "smallint", "bigint": "bigint", "boolean": "boolean"}
		if engine == "mariadb" {
			types = map[string]string{"text": "text", "integer": "int", "smallint": "smallint", "bigint": "bigint", "boolean": "tinyint(1)"}
		}
		for i := range changes {
			change := &changes[i]
			change.Review = "Review the destination definition and dependencies before writing a migration."
			column, ok := change.Source.(ColumnMeta)
			if change.Kind != "column_missing_in_destination" || !ok || !column.Nullable || column.PrimaryKey || column.DefaultVal != "" ||
				validateIdentifier(schema) != nil || validateIdentifier(change.Table) != nil || validateIdentifier(column.Name) != nil {
				continue
			}
			// Never place a source-provided type expression in executable SQL.
			if typ, ok := types[strings.ToLower(column.Type)]; ok {
				change.ProposedSQL = "ALTER TABLE " + quoteIdent(engine, schema) + "." + quoteIdent(engine, change.Table) +
					" ADD COLUMN " + quoteIdent(engine, column.Name) + " " + typ
				change.Review = "Adds a nullable column. Review source constraints, collation and indexes separately."
			}
		}
	}
	return map[string]any{"source": source, "destination": destination, "changes": changes}, nil
}
