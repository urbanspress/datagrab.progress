package dbstudio

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

func describePostgresTable(ctx context.Context, tx *sql.Tx, s networkScope) (networkTable, error) {
	var id int64
	err := tx.QueryRowContext(ctx, `SELECT c.oid::bigint FROM pg_catalog.pg_class c
		JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace
		JOIN pg_catalog.pg_am am ON am.oid=c.relam
		WHERE n.nspname=$1 AND c.relname=$2 AND c.relkind='r' AND c.relpersistence='p'
		AND am.amname='heap' AND NOT c.relrowsecurity AND c.reloptions IS NULL AND c.reltablespace=0
		AND NOT EXISTS (SELECT FROM pg_catalog.pg_inherits i WHERE c.oid IN (i.inhparent,i.inhrelid))
		AND NOT EXISTS (SELECT FROM pg_catalog.pg_constraint k
			WHERE k.confrelid=c.oid OR k.conrelid=c.oid AND k.contype NOT IN ('p','u','n'))
		AND NOT EXISTS (SELECT FROM pg_catalog.pg_trigger t WHERE t.tgrelid=c.oid)
		AND NOT EXISTS (SELECT FROM pg_catalog.pg_rewrite r WHERE r.ev_class=c.oid)`, s.schema, s.table).Scan(&id)
	if err == sql.ErrNoRows {
		return networkTable{}, unsupportedNetworkScope()
	}
	if err != nil {
		return networkTable{}, err
	}
	result := networkTable{Name: s.table, SchemaDefinition: fmt.Sprintf("relation %d", id)}
	rows, err := tx.QueryContext(ctx, `SELECT a.attname, pg_catalog.format_type(a.atttypid,a.atttypmod), a.attnotnull,
		(a.attgenerated='' AND a.attidentity='' AND NOT a.atthasdef AND a.attcollation=t.typcollation AND n.nspname='pg_catalog')
		FROM pg_catalog.pg_attribute a JOIN pg_catalog.pg_type t ON t.oid=a.atttypid
		JOIN pg_catalog.pg_namespace n ON n.oid=t.typnamespace
		WHERE a.attrelid=$1::bigint::oid AND a.attnum>0 AND NOT a.attisdropped ORDER BY a.attnum`, id)
	if err != nil {
		return networkTable{}, err
	}
	defer rows.Close()
	var definitions []string
	for rows.Next() {
		var col networkColumn
		var simple bool
		if err := rows.Scan(&col.Name, &col.Type, &col.Required, &simple); err != nil {
			return networkTable{}, err
		}
		if !simple || !supportedNetworkColumn(col) {
			return networkTable{}, unsupportedNetworkScope()
		}
		definition := quoteIdent(s.engine, col.Name) + " " + col.Type
		if col.Required {
			definition += " NOT NULL"
		}
		definitions = append(definitions, definition)
		result.Columns = append(result.Columns, col)
	}
	if err := rows.Err(); err != nil {
		return networkTable{}, err
	}
	indexes, err := tx.QueryContext(ctx, `SELECT ic.relname, i.indisprimary,i.indisunique,
		(i.indisvalid AND i.indimmediate AND i.indisready AND i.indnkeyatts=i.indnatts
		 AND i.indpred IS NULL AND i.indexprs IS NULL AND am.amname='btree'
		 AND ic.reltablespace=0 AND ic.reloptions IS NULL),
		(SELECT string_agg(pg_catalog.pg_get_indexdef(i.indexrelid,n,false), ',' ORDER BY n)
		 FROM generate_series(1,i.indnatts) n), pg_catalog.pg_get_indexdef(i.indexrelid)
		FROM pg_catalog.pg_index i JOIN pg_catalog.pg_class ic ON ic.oid=i.indexrelid
		JOIN pg_catalog.pg_am am ON am.oid=ic.relam WHERE i.indrelid=$1::bigint::oid ORDER BY ic.relname`, id)
	if err != nil {
		return networkTable{}, err
	}
	defer indexes.Close()
	for indexes.Next() {
		var name, fields, definition string
		var primary, unique, simple bool
		if err := indexes.Scan(&name, &primary, &unique, &simple, &fields, &definition); err != nil {
			return networkTable{}, err
		}
		if !simple || strings.Contains(definition, " NULLS NOT DISTINCT") {
			return networkTable{}, unsupportedNetworkScope()
		}
		result.SchemaDefinition += "\n" + definition
		names := strings.Split(fields, ",")
		for i, name := range names {
			bare := strings.Trim(name, `"`)
			if validateIdentifier(bare) != nil || name != bare && name != quoteIdent(s.engine, bare) {
				return networkTable{}, unsupportedNetworkScope()
			}
			names[i] = bare
		}
		if primary {
			if len(names) != 1 {
				return networkTable{}, unsupportedNetworkScope()
			}
			result.Primary = names[0]
		}
		for i, name := range names {
			names[i] = quoteIdent(s.engine, name)
		}
		keys := "(" + strings.Join(names, ",") + ")"
		if primary {
			definitions = append(definitions, "PRIMARY KEY "+keys)
			continue
		}
		prefix := "CREATE INDEX ON {table} "
		if unique {
			prefix = "CREATE UNIQUE INDEX ON {table} "
		}
		result.Indexes = append(result.Indexes, prefix+keys)
	}
	if err := indexes.Err(); err != nil {
		return networkTable{}, err
	}
	if result.Primary == "" {
		return networkTable{}, unsupportedNetworkScope()
	}
	result.Create = "(" + strings.Join(definitions, ",") + ")"
	return result, nil
}

func describeMariaTable(ctx context.Context, tx *sql.Tx, s networkScope) (networkTable, error) {
	var privileges int
	// Metadata filtering must never hide a trigger or a cross-schema dependency.
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(DISTINCT PRIVILEGE_TYPE) FROM information_schema.USER_PRIVILEGES
		WHERE GRANTEE = CONCAT(QUOTE(SUBSTRING_INDEX(CURRENT_USER(),'@',1)),'@',QUOTE(SUBSTRING_INDEX(CURRENT_USER(),'@',-1)))
		AND PRIVILEGE_TYPE IN ('SELECT','TRIGGER')`).Scan(&privileges); err != nil {
		return networkTable{}, err
	}
	if privileges != 2 {
		return networkTable{}, unsupportedNetworkScope()
	}
	var eligible bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT * FROM information_schema.TABLES
		WHERE TABLE_SCHEMA=? AND TABLE_NAME=? AND ENGINE='InnoDB' AND TABLE_TYPE='BASE TABLE')
		AND NOT EXISTS (SELECT * FROM information_schema.TRIGGERS WHERE EVENT_OBJECT_SCHEMA=? AND EVENT_OBJECT_TABLE=?)
		AND NOT EXISTS (SELECT * FROM information_schema.KEY_COLUMN_USAGE WHERE REFERENCED_TABLE_NAME IS NOT NULL
			AND (TABLE_SCHEMA=? AND TABLE_NAME=? OR REFERENCED_TABLE_SCHEMA=? AND REFERENCED_TABLE_NAME=?))
		AND NOT EXISTS (SELECT * FROM information_schema.PARTITIONS WHERE TABLE_SCHEMA=? AND TABLE_NAME=? AND PARTITION_NAME IS NOT NULL)`,
		s.schema, s.table, s.schema, s.table, s.schema, s.table, s.schema, s.table, s.schema, s.table).Scan(&eligible); err != nil {
		return networkTable{}, err
	}
	if !eligible {
		return networkTable{}, unsupportedNetworkScope()
	}
	result := networkTable{Name: s.table}
	rows, err := tx.QueryContext(ctx, `SELECT COLUMN_NAME,COLUMN_TYPE,IS_NULLABLE='NO',COLUMN_KEY='PRI',EXTRA
		FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=? AND TABLE_NAME=? ORDER BY ORDINAL_POSITION`, s.schema, s.table)
	if err != nil {
		return networkTable{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var col networkColumn
		var key bool
		var extra string
		if err := rows.Scan(&col.Name, &col.Type, &col.Required, &key, &extra); err != nil {
			return networkTable{}, err
		}
		if extra != "" || !supportedNetworkColumn(col) || key && result.Primary != "" {
			return networkTable{}, unsupportedNetworkScope()
		}
		if key {
			result.Primary = col.Name
		}
		result.Columns = append(result.Columns, col)
	}
	if err := rows.Err(); err != nil {
		return networkTable{}, err
	}
	var name, ddl string
	if err := tx.QueryRowContext(ctx, "SHOW CREATE TABLE "+s.tableSQL()).Scan(&name, &ddl); err != nil {
		return networkTable{}, err
	}
	prefix := "CREATE TABLE " + quoteIdent(s.engine, s.table) + " "
	if name != s.table || !strings.HasPrefix(ddl, prefix+"(") || result.Primary == "" {
		return networkTable{}, unsupportedNetworkScope()
	}
	result.Create = strings.TrimPrefix(ddl, prefix)
	return result, nil
}

func supportedNetworkColumn(col networkColumn) bool {
	return validateIdentifier(col.Name) == nil && (networkInteger.MatchString(col.Type) ||
		networkDecimal.MatchString(col.Type) || networkText.MatchString(col.Type))
}
