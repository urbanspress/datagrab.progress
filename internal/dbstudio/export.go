package dbstudio

import (
	"archive/zip"
	"context"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf16"
	"unicode/utf8"

	"groxy-desk/internal/security"
)

type ExportPageRequest struct {
	RowsRequest
	Path string `json:"path"`
}

const exportLimit = 8 << 20

type exportWriter struct {
	dst  io.Writer
	left int
}

func (w *exportWriter) Write(data []byte) (int, error) {
	if len(data) > w.left {
		return 0, errors.New("database export exceeds the size limit")
	}
	n, err := w.dst.Write(data)
	w.left -= n
	return n, err
}

// ExportPage writes a bounded, read-only table page into a new workspace file.
func (m *Manager) ExportPage(ctx context.Context, r ExportPageRequest) (map[string]any, error) {
	extension := strings.ToLower(filepath.Ext(r.Path))
	switch extension {
	case ".csv", ".json", ".xlsx", ".sql":
	default:
		return nil, security.NewError(security.CodeDeskE101, 422, "choose a .csv, .json, .xlsx, or .sql workspace path", nil)
	}
	_, path, err := security.ConfineWorkspacePath(m.workspaceRoot, r.Path)
	if err != nil {
		return nil, err
	}
	result, err := m.BrowseRows(ctx, r.RowsRequest)
	if err != nil {
		return nil, err
	}
	columns := result["columns"].([]ColumnMeta)
	rows := result["rows"].([]map[string]any)
	for _, column := range columns {
		if !utf8.ValidString(column.Name) {
			return nil, invalidImport("table page export requires UTF-8 column names", nil)
		}
		for _, row := range rows {
			if value, ok := row[column.Name].(string); ok && !utf8.ValidString(value) {
				return nil, invalidImport("table page export requires UTF-8 values", nil)
			}
		}
	}
	if err := m.writeExport(ctx, path, func(writer io.Writer) error {
		switch extension {
		case ".csv":
			return writeCSVPage(writer, columns, rows)
		case ".json":
			return json.NewEncoder(writer).Encode(rows)
		case ".xlsx":
			return writeExcelPage(writer, columns, rows)
		default:
			return writeSQLPage(writer, r, columns, rows)
		}
	}); err != nil {
		return nil, err
	}
	return map[string]any{"path": path, "engine": result["engine"], "table": r.Table, "page": result["page"], "rows_exported": fmt.Sprint(len(rows))}, nil
}

func (m *Manager) writeExport(ctx context.Context, path string, encode func(io.Writer) error) error {
	root, err := os.OpenRoot(m.workspaceRoot)
	if err != nil {
		return err
	}
	defer root.Close()
	file, err := root.OpenFile(filepath.FromSlash(path), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if errors.Is(err, os.ErrExist) {
		return security.NewError(security.CodeDeskE409, 409, "workspace export path already exists", nil)
	}
	if err != nil {
		return security.NewError(security.CodeDeskE101, 422, "cannot create the selected workspace export", err)
	}
	success := false
	defer func() {
		_ = file.Close()
		if !success {
			_ = root.Remove(filepath.FromSlash(path))
		}
	}()
	if err := encode(&exportWriter{file, exportLimit}); err != nil {
		return security.NewError(security.CodeDeskE101, 422, "database export could not be encoded", err)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	success = true
	return nil
}

func writeSQLPage(dst io.Writer, r ExportPageRequest, columns []ColumnMeta, rows []map[string]any) error {
	engine := normalizeEngine(r.Target.Engine)
	if engine == "sqlite" || engine == "sqlite-bridge" {
		raw := make([][]json.RawMessage, len(rows))
		for i, row := range rows {
			raw[i] = make([]json.RawMessage, len(columns))
			for j, column := range columns {
				value, err := json.Marshal(row[column.Name])
				if err != nil {
					return err
				}
				raw[i][j] = value
			}
		}
		statements, err := sqliteInsert(r.Table, columns, raw)
		if err != nil {
			return err
		}
		_, err = io.WriteString(dst, statements)
		return err
	}
	schema := r.Target.DBName
	if engine == "postgres" {
		schema = firstNonEmpty(r.Target.SchemaName, "public")
	}
	if validateIdentifier(schema) != nil || validateIdentifier(r.Table) != nil {
		return invalidImport("SQL export requires an identified source table and schema", nil)
	}
	names := make([]string, len(columns))
	for i, column := range columns {
		if validateIdentifier(column.Name) != nil || strings.TrimSpace(column.Name) != column.Name {
			return invalidImport("SQL export encountered an unsupported column identifier", nil)
		}
		names[i] = quoteIdent(engine, column.Name)
	}
	insert := "INSERT INTO " + quoteIdent(engine, schema) + "." + quoteIdent(engine, r.Table) + " (" + strings.Join(names, ", ") + ") VALUES ("
	values := make([]string, len(columns))
	for _, row := range rows {
		for i, column := range columns {
			value, err := networkSQLLiteral(engine, column.Type, row[column.Name])
			if err != nil {
				return err
			}
			values[i] = value
		}
		if _, err := io.WriteString(dst, insert+strings.Join(values, ", ")+");\n"); err != nil {
			return err
		}
	}
	return nil
}

func networkSQLLiteral(engine, dataType string, value any) (string, error) {
	if value == nil {
		return "NULL", nil
	}
	if logical, ok := value.(bool); ok && engine == "postgres" && strings.EqualFold(dataType, "bool") {
		if logical {
			return "TRUE", nil
		}
		return "FALSE", nil
	}
	text, ok := value.(string)
	if ok {
		switch strings.TrimPrefix(strings.ToLower(dataType), "unsigned ") {
		case "smallint", "integer", "bigint", "int", "int2", "int4", "int8", "tinyint", "mediumint":
			if security.IsExactIntString(text) {
				return text, nil
			}
		case "decimal", "numeric":
			if security.IsExactDecimalString(text) || security.IsExactIntString(text) {
				return text, nil
			}
		case "text", "varchar", "character varying", "char", "bpchar":
			hexText := hex.EncodeToString([]byte(text))
			if engine == "postgres" && !strings.ContainsRune(text, '\x00') {
				return "pg_catalog.convert_from(pg_catalog.decode('" + hexText + "', 'hex'), 'UTF8')", nil
			}
			if engine == "mariadb" {
				return "CONVERT(X'" + hexText + "' USING utf8mb4)", nil
			}
		}
	}
	return "", invalidImport("SQL export cannot preserve a value in this column type", nil)
}

func csvFormula(text string) bool {
	first := strings.TrimLeftFunc(text, func(r rune) bool { return unicode.IsSpace(r) || r == '\ufeff' || unicode.IsControl(r) })
	r, _ := utf8.DecodeRuneInString(first)
	return strings.ContainsRune("=+-@＝＋－＠", r) && !security.IsExactDecimalString(text) && !security.IsExactIntString(text)
}

func safeSpreadsheetCSV(text string) string {
	if strings.HasPrefix(text, "'") || csvFormula(text) {
		return "'" + text
	}
	return text
}

func unescapeSpreadsheetCSV(text string) string {
	if strings.HasPrefix(text, "'") && (strings.HasPrefix(text[1:], "'") || csvFormula(text[1:])) {
		return text[1:]
	}
	return text
}

func writeCSVPage(dst io.Writer, columns []ColumnMeta, rows []map[string]any) error {
	out := csv.NewWriter(dst)
	fields := make([]string, len(columns))
	for i, col := range columns {
		fields[i] = safeSpreadsheetCSV(col.Name)
	}
	if err := out.Write(fields); err != nil {
		return err
	}
	for _, row := range rows {
		for i, col := range columns {
			value := row[col.Name]
			switch {
			case value == nil:
				fields[i] = `\N`
			default:
				text := fmt.Sprint(value)
				if strings.HasPrefix(text, `\`) {
					text = `\` + text
				}
				fields[i] = safeSpreadsheetCSV(text)
			}
		}
		if err := out.Write(fields); err != nil {
			return err
		}
	}
	out.Flush()
	return out.Error()
}

var excelParts = []struct{ path, xml string }{
	{"[Content_Types].xml", `<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/><Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/></Types>`},
	{"_rels/.rels", `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/></Relationships>`},
	{"xl/workbook.xml", `<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets><sheet name="Table page" sheetId="1" r:id="rId1"/></sheets></workbook>`},
	{"xl/_rels/workbook.xml.rels", `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/></Relationships>`},
}

func excelColumn(index int) string {
	index++
	result := ""
	for index > 0 {
		index--
		result = string(rune('A'+index%26)) + result
		index /= 26
	}
	return result
}

type excelTextCell struct {
	XMLName   xml.Name        `xml:"c"`
	Reference string          `xml:"r,attr"`
	Type      string          `xml:"t,attr"`
	Inline    excelInlineText `xml:"is>t"`
}

type excelInlineText struct {
	Preserve string `xml:"xml:space,attr"`
	Text     string `xml:",chardata"`
}

func writeExcelPage(dst io.Writer, columns []ColumnMeta, rows []map[string]any) error {
	if len(columns) > 16384 {
		return errors.New("too many columns for Excel")
	}
	book := zip.NewWriter(dst)
	for _, part := range excelParts {
		w, err := book.Create(part.path)
		if err != nil {
			return err
		}
		if _, err := io.WriteString(w, part.xml); err != nil {
			return err
		}
	}
	file, err := book.Create("xl/worksheets/sheet1.xml")
	if err != nil {
		return err
	}
	sheet := &exportWriter{file, exportLimit}
	if _, err = io.WriteString(sheet, `<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData>`); err != nil {
		return err
	}
	encoder := xml.NewEncoder(sheet)
	// SpreadsheetML uses _xHHHH_ for literal escapes, so quote any escape prefix
	// supplied by the database before passing the text to Excel.
	for rowNumber := 1; rowNumber <= len(rows)+1; rowNumber++ {
		if _, err := fmt.Fprintf(sheet, `<row r="%d">`, rowNumber); err != nil {
			return err
		}
		for i, col := range columns {
			var value any = col.Name
			if rowNumber > 1 {
				value = rows[rowNumber-2][col.Name]
			}
			if value == nil {
				continue
			}
			content := fmt.Sprint(value)
			if !utf8.ValidString(content) || utf16Length(content) > 32767 || strings.ContainsFunc(content, func(r rune) bool {
				return (r < ' ' && r != '\t' && r != '\n' && r != '\r') || r == '\ufffe' || r == '\uffff'
			}) {
				return errors.New("cell cannot be represented as Excel text")
			}
			content = excelEscape.ReplaceAllStringFunc(content, func(escape string) string { return "_x005F_" + escape[1:] })
			cell := excelTextCell{Reference: fmt.Sprintf("%s%d", excelColumn(i), rowNumber), Type: "inlineStr", Inline: excelInlineText{"preserve", content}}
			if err := encoder.Encode(cell); err != nil {
				return err
			}
		}
		if _, err := io.WriteString(sheet, "</row>"); err != nil {
			return err
		}
	}
	if _, err := io.WriteString(sheet, "</sheetData></worksheet>"); err != nil {
		return err
	}
	return book.Close()
}

func utf16Length(text string) int {
	length := 0
	for _, r := range text {
		length += utf16.RuneLen(r)
	}
	return length
}
