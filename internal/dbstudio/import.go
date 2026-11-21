package dbstudio

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"groxy-desk/internal/security"
)

const maxImportBytes = 256 << 10

type ImportRequest struct {
	Target        TargetSpec `json:"target"`
	Path          string     `json:"path"`
	Table         string     `json:"table,omitempty"`
	ApprovalToken string     `json:"approval_token,omitempty"`
}

func invalidImport(message string, err error) error {
	return security.NewError(security.CodeDeskE101, 422, message, err)
}

func (m *Manager) readImport(path string) ([]byte, error) {
	_, rel, err := security.ConfineWorkspacePath(m.workspaceRoot, path)
	if err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(m.workspaceRoot)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	f, err := root.Open(filepath.FromSlash(rel))
	if err != nil {
		return nil, invalidImport("cannot open the selected import file", err)
	}
	defer f.Close()
	stat, err := f.Stat()
	if err != nil || !stat.Mode().IsRegular() || stat.Size() > maxImportBytes {
		return nil, invalidImport("import requires a regular workspace file of at most 256 KiB", err)
	}
	body, err := io.ReadAll(io.LimitReader(f, maxImportBytes+1))
	if err != nil || len(body) > maxImportBytes {
		return nil, invalidImport("import exceeds the size limit", err)
	}
	return body, nil
}

type importTable struct {
	columns []string
	rows    [][]json.RawMessage
}

func readJSONRows(body []byte) (importTable, error) {
	var records []map[string]json.RawMessage
	if err := json.Unmarshal(body, &records); err != nil {
		return importTable{}, invalidImport("JSON imports require an array of row objects", err)
	}
	if len(records) == 0 || len(records) > MaxRowsPerPage {
		return importTable{}, invalidImport("import must contain 1 to 100 data rows", nil)
	}
	var table importTable
	for name := range records[0] {
		table.columns = append(table.columns, name)
	}
	sort.Strings(table.columns)
	for _, record := range records {
		if len(record) != len(table.columns) {
			return importTable{}, invalidImport("all JSON rows must specify the same columns", nil)
		}
		row := make([]json.RawMessage, len(table.columns))
		for i, name := range table.columns {
			row[i] = record[name]
			if len(row[i]) == 0 {
				return importTable{}, invalidImport("all JSON rows must specify the same columns", nil)
			}
		}
		table.rows = append(table.rows, row)
	}
	return table, nil
}

func readCSVRows(body []byte) (importTable, error) {
	input := csv.NewReader(bytes.NewReader(bytes.TrimPrefix(body, []byte("\xef\xbb\xbf"))))
	names, err := input.Read()
	if err != nil {
		return importTable{}, invalidImport("CSV imports require a column header", err)
	}
	for i, name := range names {
		names[i] = unescapeSpreadsheetCSV(name)
	}
	table := importTable{columns: names}
	for {
		fields, err := input.Read()
		if err == io.EOF {
			return table, nil
		}
		if err != nil {
			return importTable{}, invalidImport("CSV rows must match the column header", err)
		}
		if len(table.rows) >= MaxRowsPerPage {
			return importTable{}, invalidImport("import must contain at most 100 data rows", nil)
		}
		row := make([]json.RawMessage, len(fields))
		for i, v := range fields {
			if v == `\N` {
				row[i] = json.RawMessage("null")
				continue
			}
			if strings.HasPrefix(v, `\\`) {
				v = v[1:]
			}
			row[i], _ = json.Marshal(unescapeSpreadsheetCSV(v))
		}
		table.rows = append(table.rows, row)
	}
}

func sqliteLiteral(raw json.RawMessage, dataType string) (string, error) {
	kind, err := cellKind(cellColumn{dataType: dataType}, "")
	if err != nil {
		return "", err
	}
	if !utf8.Valid(raw) {
		return "", invalidImport("SQLite transfer requires valid UTF-8 text", nil)
	}
	value, err := editValue(raw, kind)
	if err != nil {
		return "", err
	}
	switch v := value.(type) {
	case nil:
		return "NULL", nil
	case int64:
		return strconv.FormatInt(v, 10), nil
	case string:
		return "CAST(X'" + hex.EncodeToString([]byte(v)) + "' AS TEXT)", nil
	default:
		return "", invalidImport("unsupported SQLite transfer type", nil)
	}
}

func sqliteInsert(table string, columns []ColumnMeta, rows [][]json.RawMessage) (string, error) {
	if len(columns) == 0 || len(rows) == 0 {
		return "", invalidImport("there are no data rows or column mappings", nil)
	}
	names := make([]string, len(columns))
	seen := map[string]bool{}
	for i, column := range columns {
		name := strings.ToLower(column.Name)
		if seen[name] {
			return "", invalidImport("the import names a column more than once", nil)
		}
		seen[name] = true
		names[i] = sqliteIdent(column.Name)
	}
	var out strings.Builder
	for _, row := range rows {
		if len(row) != len(columns) {
			return "", invalidImport("row does not match the selected columns", nil)
		}
		literals := make([]string, len(columns))
		for i, value := range row {
			literal, err := sqliteLiteral(value, columns[i].Type)
			if err != nil {
				return "", err
			}
			literals[i] = literal
		}
		fmt.Fprintf(&out, "INSERT INTO %s (%s) VALUES (%s);\n", sqliteIdent(table), strings.Join(names, ", "), strings.Join(literals, ", "))
		if out.Len() > exportLimit {
			return "", invalidImport("generated SQL exceeds the transfer limit", nil)
		}
	}
	return out.String(), nil
}

func (m *Manager) importOperation(ctx context.Context, r ImportRequest) (string, string, error) {
	engine := normalizeEngine(r.Target.Engine)
	switch {
	case engine == "postgres" && r.Target.MaintenanceUser != "":
		if _, err := m.postgresFullScope(r.Target); err != nil {
			return "", "", err
		}
	case engine == "mariadb":
		if _, err := m.mariadbFullScope(ctx, r.Target, true); err != nil {
			return "", "", err
		}
	case engine == "postgres":
		return "", "", m.fullNetworkWriteUnavailable(r.Target)
	default:
		if _, _, err := m.sqliteTarget(r.Target); err != nil {
			return "", "", err
		}
	}
	raw, err := m.readImport(r.Path)
	if err != nil {
		return "", "", err
	}
	if !utf8.Valid(raw) && !strings.EqualFold(filepath.Ext(r.Path), ".xlsx") {
		return "", "", invalidImport("import file must be UTF-8", nil)
	}
	var data importTable
	switch strings.ToLower(filepath.Ext(r.Path)) {
	case ".sql":
		validate := validatedMutation
		switch engine {
		case "postgres":
			validate = postgresWriteStatements
		case "mariadb":
			validate = mariadbWriteStatements
		}
		stmts, err := validate(string(raw))
		if err != nil {
			return "", "", err
		}
		return string(raw), fmt.Sprintf("Execute %d reviewed SQL statements.", len(stmts)), nil
	case ".csv":
		data, err = readCSVRows(raw)
	case ".json":
		data, err = readJSONRows(raw)
	case ".xlsx":
		data, err = readExcelRows(raw)
	default:
		return "", "", invalidImport("choose a .csv, .json, .xlsx, or .sql workspace file", nil)
	}
	if err != nil {
		return "", "", err
	}
	if err := validateIdentifier(r.Table); err != nil {
		return "", "", err
	}
	catalog, err := m.GetSchema(ctx, r.Target)
	if err != nil {
		return "", "", err
	}
	for _, table := range catalog["tables"].([]TableSchema) {
		if table.Name != r.Table {
			continue
		}
		byName := map[string]ColumnMeta{}
		for _, column := range table.Columns {
			byName[column.Name] = column
		}
		mapped := make([]ColumnMeta, len(data.columns))
		fieldLabels := make([]string, len(data.columns))
		for i, name := range data.columns {
			meta, ok := byName[name]
			if !ok {
				return "", "", invalidImport(fmt.Sprintf("column %q does not exist in %s", name, r.Table), nil)
			}
			mapped[i] = meta
			fieldLabels[i] = name + " " + meta.Type
		}
		var query string
		switch engine {
		case "postgres":
			query, err = postgresInsert(r.Table, mapped, data.rows)
		case "mariadb":
			query, err = mariadbInsert(r.Table, mapped, data.rows)
		default:
			query, err = sqliteInsert(r.Table, mapped, data.rows)
		}
		if err != nil {
			return "", "", err
		}
		return query, fmt.Sprintf("Append %d rows to %s (%s).", len(data.rows), r.Table, strings.Join(fieldLabels, ", ")), nil
	}
	return "", "", invalidImport("select an existing destination table", nil)
}

func mariadbInsert(table string, columns []ColumnMeta, rows [][]json.RawMessage) (string, error) {
	if len(columns) == 0 || len(rows) == 0 {
		return "", invalidImport("the import needs columns and rows", nil)
	}
	names := make([]string, len(columns))
	for i, col := range columns {
		lower := strings.ToLower(col.Type)
		if strings.Contains(lower, "blob") || strings.Contains(lower, "binary") || strings.HasPrefix(lower, "bit") ||
			strings.HasPrefix(lower, "float") || strings.HasPrefix(lower, "double") || strings.HasPrefix(lower, "real") {
			return "", invalidImport("MariaDB imports support exact text, integer, decimal and date values only", nil)
		}
		names[i] = quoteIdent("mariadb", col.Name)
	}
	var out strings.Builder
	for _, row := range rows {
		if len(row) != len(columns) {
			return "", invalidImport("import row does not match the selected columns", nil)
		}
		values := make([]string, len(row))
		for i, raw := range row {
			var value *string
			if err := json.Unmarshal(raw, &value); err != nil {
				return "", invalidImport("import exact MariaDB values as strings or null", err)
			}
			values[i] = "NULL"
			if value != nil {
				// Desk runs reviewed statements without NO_BACKSLASH_ESCAPES.
				values[i] = "'" + strings.NewReplacer("\\", "\\\\", "'", "''", "\x00", "\\0", "\x1a", "\\Z").Replace(*value) + "'"
			}
		}
		fmt.Fprintf(&out, "INSERT INTO %s (%s) VALUES (%s);\n", quoteIdent("mariadb", table), strings.Join(names, ", "), strings.Join(values, ", "))
		if out.Len() > exportLimit {
			return "", invalidImport("generated SQL exceeds the transfer limit", nil)
		}
	}
	return out.String(), nil
}

func postgresInsert(table string, columns []ColumnMeta, rows [][]json.RawMessage) (string, error) {
	if len(columns) == 0 || len(rows) == 0 {
		return "", invalidImport("the import needs columns and rows", nil)
	}
	names := make([]string, len(columns))
	for i, col := range columns {
		names[i] = quoteIdent("postgres", col.Name)
	}
	var sql strings.Builder
	for _, row := range rows {
		if len(row) != len(columns) {
			return "", invalidImport("import row does not match the selected columns", nil)
		}
		values := make([]string, len(row))
		for i, raw := range row {
			var value *string
			if err := json.Unmarshal(raw, &value); err != nil {
				return "", invalidImport("import exact PostgreSQL values as strings or null", err)
			}
			values[i] = "NULL"
			if value != nil {
				// An explicit escape literal is independent of the database's
				// standard_conforming_strings setting. PostgreSQL converts the
				// resulting unknown literal directly to the destination type.
				escaped := strings.NewReplacer("\\", "\\\\", "'", "''").Replace(*value)
				values[i] = "E'" + escaped + "'"
			}
		}
		fmt.Fprintf(&sql, "INSERT INTO %s (%s) VALUES (%s);\n", quoteIdent("postgres", table), strings.Join(names, ", "), strings.Join(values, ", "))
		if sql.Len() > exportLimit {
			return "", invalidImport("generated SQL exceeds the transfer limit", nil)
		}
	}
	return sql.String(), nil
}

func (m *Manager) PlanImport(ctx context.Context, r ImportRequest) (map[string]any, error) {
	query, summary, err := m.importOperation(ctx, r)
	if err != nil {
		return nil, err
	}
	plan, err := m.PlanMutation(ctx, MutatePlanRequest{Target: r.Target, SQL: query})
	if err != nil {
		return nil, err
	}
	plan["plain_summary"] = summary + " " + fmt.Sprint(plan["plain_summary"])
	if strings.EqualFold(filepath.Ext(r.Path), ".sql") {
		plan["reviewed_sql"] = query
	}
	return plan, nil
}

func (m *Manager) ApplyImport(ctx context.Context, r ImportRequest) (map[string]any, error) {
	query, _, err := m.importOperation(ctx, r)
	if err != nil {
		return nil, err
	}
	return m.ApplyMutation(ctx, MutateApplyRequest{Target: r.Target, SQL: query, ApprovalToken: r.ApprovalToken})
}
