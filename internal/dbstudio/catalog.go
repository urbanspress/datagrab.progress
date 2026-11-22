package dbstudio

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
)

// A tagged result stream keeps the engine's metadata ordering and types.
const postgresCatalog = `
WITH relations AS (
	SELECT c.oid, c.relname, c.relkind
	FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace
	WHERE n.nspname=$1 AND c.relkind IN ('r','p','v','m','f')
), catalog (kind, name, field1, field2, field3, field4, field5, sort_name, position) AS (
	SELECT 0, relname, '', '', '', '', '', '', 0 FROM relations WHERE relkind IN ('r','p')
	UNION ALL
	SELECT 1, r.relname, a.attname, pg_catalog.format_type(a.atttypid,a.atttypmod),
	       CASE WHEN a.attnotnull THEN '0' ELSE '1' END,
	       CASE WHEN EXISTS (SELECT 1 FROM pg_catalog.pg_index i
	                         WHERE i.indrelid=r.oid AND i.indisprimary AND a.attnum=ANY(i.indkey))
	            THEN '1' ELSE '0' END,
	       COALESCE(pg_catalog.pg_get_expr(d.adbin,d.adrelid),''), '', a.attnum
	FROM relations r
	JOIN pg_catalog.pg_attribute a ON a.attrelid=r.oid AND a.attnum>0 AND NOT a.attisdropped
	LEFT JOIN pg_catalog.pg_attrdef d ON d.adrelid=r.oid AND d.adnum=a.attnum
	WHERE r.relkind IN ('r','p')
	UNION ALL
	SELECT 2, src.relname, a.attname, nt.nspname, dst.relname, b.attname, '', k.conname, cols.pos
	FROM relations src
	JOIN pg_catalog.pg_constraint k ON k.conrelid=src.oid AND k.contype='f'
	JOIN pg_catalog.pg_class dst ON dst.oid=k.confrelid
	JOIN pg_catalog.pg_namespace nt ON nt.oid=dst.relnamespace
	CROSS JOIN LATERAL unnest(k.conkey,k.confkey) WITH ORDINALITY AS cols(source,target,pos)
	JOIN pg_catalog.pg_attribute a ON a.attrelid=src.oid AND a.attnum=cols.source
	JOIN pg_catalog.pg_attribute b ON b.attrelid=dst.oid AND b.attnum=cols.target
	UNION ALL
	SELECT 3, tablename, indexname, '', '', '', '', indexname, 0
	FROM pg_catalog.pg_indexes WHERE schemaname=$1
	UNION ALL
	SELECT 4, relname, '', '', '', '', '', '', 0 FROM relations WHERE relkind IN ('v','m')
	UNION ALL
	SELECT 5, t.tgname, c.relname, '', '', '', '', c.relname, 0
	FROM pg_catalog.pg_trigger t JOIN relations c ON c.oid=t.tgrelid
	WHERE NOT t.tgisinternal
)
SELECT kind, name, field1, field2, field3, field4, field5
FROM catalog ORDER BY kind, name, sort_name, position`

const mariaCatalog = `
WITH base_tables AS (
	SELECT TABLE_NAME FROM information_schema.TABLES
	WHERE TABLE_SCHEMA=DATABASE() AND TABLE_TYPE='BASE TABLE'
), catalog (kind, name, field1, field2, field3, field4, field5, sort_name, position) AS (
	SELECT 0, TABLE_NAME, '', '', '', '', '', '', 0 FROM base_tables
	UNION ALL
	SELECT 1, c.TABLE_NAME, c.COLUMN_NAME, c.COLUMN_TYPE,
	       CASE WHEN c.IS_NULLABLE='YES' THEN '1' ELSE '0' END,
	       CASE WHEN c.COLUMN_KEY='PRI' THEN '1' ELSE '0' END,
	       COALESCE(c.COLUMN_DEFAULT,''), '', c.ORDINAL_POSITION
	FROM information_schema.COLUMNS c JOIN base_tables t ON t.TABLE_NAME=c.TABLE_NAME
	WHERE c.TABLE_SCHEMA=DATABASE()
	UNION ALL
	SELECT 2, TABLE_NAME, COLUMN_NAME, REFERENCED_TABLE_SCHEMA, REFERENCED_TABLE_NAME,
	       REFERENCED_COLUMN_NAME, '', CONSTRAINT_NAME, ORDINAL_POSITION
	FROM information_schema.KEY_COLUMN_USAGE
	WHERE TABLE_SCHEMA=DATABASE() AND REFERENCED_TABLE_NAME IS NOT NULL
	UNION ALL
	SELECT DISTINCT 3, TABLE_NAME, INDEX_NAME, '', '', '', '', INDEX_NAME, 0
	FROM information_schema.STATISTICS WHERE TABLE_SCHEMA=DATABASE()
	UNION ALL
	SELECT 4, TABLE_NAME, '', '', '', '', '', '', 0
	FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_TYPE='VIEW'
	UNION ALL
	SELECT DISTINCT 5, TRIGGER_NAME, EVENT_OBJECT_TABLE, '', '', '', '', EVENT_OBJECT_TABLE, 0
	FROM information_schema.TRIGGERS WHERE TRIGGER_SCHEMA=DATABASE()
)
SELECT kind, name, field1, field2, field3, field4, field5
FROM catalog ORDER BY kind, name, sort_name, position`

type schemaObject struct {
	Name  string `json:"name"`
	Table string `json:"table,omitempty"`
}

type networkCatalog struct {
	tables   []TableSchema
	views    []schemaObject
	triggers []schemaObject
}

func inspectNetworkCatalog(ctx context.Context, db *sql.DB, engine, schema string) (networkCatalog, error) {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return networkCatalog{}, err
	}
	defer tx.Rollback()

	var rows *sql.Rows
	if engine == "postgres" {
		// Bind the schema without a separate statement-description exchange.
		rows, err = tx.QueryContext(ctx, postgresCatalog, pgx.QueryExecModeExec, schema)
	} else {
		rows, err = tx.QueryContext(ctx, mariaCatalog)
	}
	if err != nil {
		return networkCatalog{}, err
	}
	defer rows.Close()

	tables := make(map[string]*TableSchema)
	result := networkCatalog{views: []schemaObject{}, triggers: []schemaObject{}}
	for rows.Next() {
		var kind int
		var name string
		var f [5]string
		if err := rows.Scan(&kind, &name, &f[0], &f[1], &f[2], &f[3], &f[4]); err != nil {
			return networkCatalog{}, err
		}
		switch kind {
		case 0:
			tables[name] = &TableSchema{Name: name, Schema: schema}
		case 1:
			table := tables[name]
			if table != nil {
				table.Columns = append(table.Columns, ColumnMeta{
					Name: f[0], Type: f[1], Nullable: f[2] == "1", PrimaryKey: f[3] == "1", DefaultVal: f[4],
				})
			}
		case 2:
			if table := tables[name]; table != nil {
				table.ForeignKeys = append(table.ForeignKeys, ForeignKeyMeta{
					Column: f[0], RefSchema: f[1], RefTable: f[2], RefColumn: f[3],
				})
			}
		case 3:
			if table := tables[name]; table != nil {
				table.Indexes = append(table.Indexes, f[0])
			}
		case 4:
			result.views = append(result.views, schemaObject{Name: name})
		case 5:
			result.triggers = append(result.triggers, schemaObject{Name: name, Table: f[0]})
		}
	}
	if err := rows.Err(); err != nil {
		return networkCatalog{}, err
	}

	result.tables = make([]TableSchema, 0, len(tables))
	for _, table := range tables {
		result.tables = append(result.tables, *table)
	}
	sort.Slice(result.tables, func(i, j int) bool { return result.tables[i].Name < result.tables[j].Name })

	if err := batchCatalogRowCounts(ctx, tx, engine, schema, result.tables); err != nil {
		return networkCatalog{}, err
	}
	return result, tx.Commit()
}

func batchCatalogRowCounts(ctx context.Context, tx *sql.Tx, engine, schema string, tables []TableSchema) error {
	if len(tables) == 0 {
		return nil
	}
	// Quote catalog identifiers without trimming name characters.
	quote := sqliteIdent
	if engine == "mariadb" {
		quote = func(name string) string {
			return "`" + strings.ReplaceAll(name, "`", "``") + "`"
		}
	}
	prefix := quote(schema) + "."
	selects := make([]string, len(tables))
	for i, table := range tables {
		selects[i] = "SELECT " + strconv.Itoa(i) + ", COUNT(*) FROM " + prefix + quote(table.Name)
	}
	rows, err := tx.QueryContext(ctx, strings.Join(selects, " UNION ALL "))
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var i int
		var count string
		if err := rows.Scan(&i, &count); err != nil {
			return err
		}
		tables[i].RowCount = count
	}
	return rows.Err()
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
