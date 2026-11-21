package dbstudio

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"groxy-desk/internal/security"
)

type CellEdit struct {
	Table     string                     `json:"table"`
	Column    string                     `json:"column"`
	Key       map[string]json.RawMessage `json:"key"`
	Before    json.RawMessage            `json:"before"`
	Value     json.RawMessage            `json:"value"`
	ValueKind string                     `json:"value_kind,omitempty"`
}

type cellColumn struct {
	name     string
	dataType string
	required bool
	primary  bool
}

func invalidCell(message string) error {
	return security.NewError(security.CodeDeskE101, 422, message, nil)
}

func editValue(raw json.RawMessage, kind string) (any, error) {
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return nil, nil
	}
	var value string
	if len(raw) == 0 || json.Unmarshal(raw, &value) != nil {
		return nil, invalidCell("cell values must be JSON strings or null; send exact numbers as base-10 strings")
	}
	switch kind {
	case "", "text":
		return value, nil
	case "integer":
		if !security.IsExactIntString(value) {
			return nil, invalidCell("an exact base-10 integer string is required")
		}
		integer, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return nil, invalidCell("integer exceeds SQLite's signed 64-bit storage range")
		}
		return integer, nil
	case "decimal":
		if !security.IsExactDecimalString(value) {
			return nil, invalidCell("an exact base-10 decimal string is required")
		}
		return value, nil
	default:
		return nil, invalidCell("unsupported cell value type")
	}
}

func cellKind(column cellColumn, requested string) (string, error) {
	dataType := strings.ToUpper(column.dataType)
	switch {
	case strings.Contains(dataType, "INT"):
		if requested != "" && requested != "integer" {
			return "", invalidCell("this column requires an integer")
		}
		return "integer", nil
	case strings.Contains(dataType, "TEXT"), strings.Contains(dataType, "CHAR"), strings.Contains(dataType, "CLOB"):
		if requested == "integer" {
			return "", invalidCell("choose decimal or text to preserve text affinity")
		}
		return requested, nil
	default:
		return "", invalidCell("this SQLite column cannot guarantee exact inline round trips")
	}
}

func prepareCell(ctx context.Context, conn *sql.Conn, edit CellEdit) (string, []any, error) {
	if validateIdentifier(edit.Table) != nil || validateIdentifier(edit.Column) != nil ||
		len(edit.Value) == 0 || len(edit.Before) == 0 {
		return "", nil, invalidCell("a table, column, old value and replacement value are required")
	}
	if _, err := editValue(edit.Value, edit.ValueKind); err != nil {
		return "", nil, err
	}
	if _, err := editValue(edit.Before, ""); err != nil {
		return "", nil, err
	}
	rows, err := conn.QueryContext(ctx, `SELECT name,type,"notnull",pk,hidden FROM pragma_table_xinfo(?, 'main') ORDER BY cid`, edit.Table)
	if err != nil {
		return "", nil, err
	}
	defer rows.Close()
	var keys []cellColumn
	var selected *cellColumn
	for rows.Next() {
		var meta cellColumn
		var notnull, pk, hidden int
		if err := rows.Scan(&meta.name, &meta.dataType, &notnull, &pk, &hidden); err != nil {
			return "", nil, err
		}
		meta.required = notnull != 0
		meta.primary = pk != 0
		if meta.primary {
			keys = append(keys, meta)
		}
		if meta.name == edit.Column && hidden == 0 && !meta.primary {
			selected = &meta
		}
	}
	if err := rows.Err(); err != nil {
		return "", nil, err
	}
	if err := rows.Close(); err != nil {
		return "", nil, err
	}
	if selected == nil || len(keys) == 0 || len(edit.Key) != len(keys) {
		return "", nil, invalidCell("inline editing requires an ordinary column and a complete declared primary key")
	}
	args := make([]any, 0, len(keys))
	predicates := make([]string, 0, len(keys))
	for _, key := range keys {
		kind, err := cellKind(key, "")
		if err != nil {
			return "", nil, err
		}
		value, err := editValue(edit.Key[key.name], kind)
		if err != nil {
			return "", nil, err
		}
		if value == nil {
			return "", nil, invalidCell("inline editing requires non-null primary key values")
		}
		args = append(args, value)
		predicates = append(predicates, sqliteIdent(key.name)+" = ? COLLATE BINARY")
	}
	where := strings.Join(predicates, " AND ")
	table := "main." + sqliteIdent(edit.Table)
	column := sqliteIdent(edit.Column)
	var value any
	err = conn.QueryRowContext(ctx, "SELECT "+column+" FROM "+table+" WHERE "+where, args...).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil, security.NewError(security.CodeDeskE409, 409, "primary key no longer identifies a row", nil)
	}
	if err != nil {
		return "", nil, err
	}
	before := formatWireCell(value)
	expected, err := editValue(edit.Before, "")
	if err != nil {
		return "", nil, err
	}
	if before != expected {
		return "", nil, security.NewError(security.CodeDeskE409, 409, "cell value changed since readback", nil)
	}
	kind, err := cellKind(*selected, edit.ValueKind)
	if err != nil {
		return "", nil, err
	}
	replacement, err := editValue(edit.Value, kind)
	if err != nil {
		return "", nil, err
	}
	if replacement == nil && selected.required {
		return "", nil, invalidCell("this column does not accept null")
	}
	if _, ok := value.(float64); ok {
		return "", nil, invalidCell("stored cell is approximate and cannot be edited as an exact value")
	}
	return fmt.Sprintf("UPDATE %s SET %s = ? WHERE %s", table, column, where), append([]any{replacement}, args...), nil
}
