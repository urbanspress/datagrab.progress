package dbstudio

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf16"
)

var excelEscape = regexp.MustCompile(`(?i)_x([0-9a-f]{4})_`)
var cellReference = regexp.MustCompile(`^([A-Z]{1,3})[1-9][0-9]*$`)

type excelString struct {
	Text string `xml:"t"`
	Runs []struct {
		Text string `xml:"t"`
	} `xml:"r"`
}

func (text excelString) String() string {
	var combined strings.Builder
	combined.WriteString(text.Text)
	for _, run := range text.Runs {
		combined.WriteString(run.Text)
	}
	input := combined.String()
	matches := excelEscape.FindAllStringSubmatchIndex(input, -1)
	if len(matches) == 0 {
		return input
	}
	var out strings.Builder
	var encoded []uint16
	end := 0
	for _, match := range matches {
		if match[0] > end {
			out.WriteString(string(utf16.Decode(encoded)))
			encoded = encoded[:0]
			out.WriteString(input[end:match[0]])
		}
		value, _ := strconv.ParseUint(input[match[2]:match[3]], 16, 16)
		encoded = append(encoded, uint16(value))
		end = match[1]
	}
	out.WriteString(string(utf16.Decode(encoded)))
	out.WriteString(input[end:])
	return out.String()
}

type excelInputCell struct {
	Reference string      `xml:"r,attr"`
	Type      string      `xml:"t,attr"`
	Value     *string     `xml:"v"`
	Formula   *string     `xml:"f"`
	Inline    excelString `xml:"is"`
}

func excelPart(book *zip.Reader, path string, dst any) (bool, error) {
	budget := uint64(exportLimit)
	for _, f := range book.File {
		if f.Name != path {
			continue
		}
		if f.UncompressedSize64 > budget {
			return false, fmt.Errorf("Excel part exceeds size limit")
		}
		stream, err := f.Open()
		if err != nil {
			return false, err
		}
		defer stream.Close()
		raw, err := io.ReadAll(io.LimitReader(stream, int64(budget)+1))
		if err != nil || uint64(len(raw)) > budget {
			return false, fmt.Errorf("invalid Excel part")
		}
		return true, xml.Unmarshal(raw, dst)
	}
	return false, nil
}

func excelInputValue(cell excelInputCell, shared []excelString) (json.RawMessage, error) {
	var value any
	switch {
	case cell.Formula != nil:
		value = "=" + *cell.Formula
	case cell.Type == "inlineStr":
		value = cell.Inline.String()
	case cell.Type == "s":
		if cell.Value == nil {
			return nil, fmt.Errorf("missing shared string index")
		}
		i, err := strconv.Atoi(*cell.Value)
		if err != nil || i < 0 || i >= len(shared) {
			return nil, fmt.Errorf("invalid shared string index")
		}
		value = shared[i].String()
	case cell.Type == "str" && cell.Value != nil:
		value = *cell.Value
	case cell.Value != nil:
		return nil, fmt.Errorf("use text cells for imports; numeric cells may already have lost precision")
	}
	return json.Marshal(value)
}

func readExcelRows(body []byte) (importTable, error) {
	file, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		return importTable{}, invalidImport("invalid Excel archive", err)
	}
	var shared struct {
		Strings []excelString `xml:"si"`
	}
	if _, err = excelPart(file, "xl/sharedStrings.xml", &shared); err != nil {
		return importTable{}, invalidImport("invalid Excel shared strings", err)
	}
	var sheet struct {
		Rows []struct {
			Cells []excelInputCell `xml:"c"`
		} `xml:"sheetData>row"`
	}
	found, err := excelPart(file, "xl/worksheets/sheet1.xml", &sheet)
	if err != nil || !found || len(sheet.Rows) < 2 || len(sheet.Rows) > MaxRowsPerPage+1 {
		return importTable{}, invalidImport("Excel imports require sheet1 with a header and 1 to 100 data rows", err)
	}
	width := len(sheet.Rows[0].Cells)
	if width == 0 || width > 16384 {
		return importTable{}, invalidImport("missing Excel column headers", nil)
	}
	table := importTable{columns: make([]string, width)}
	for number, row := range sheet.Rows {
		values := make([]json.RawMessage, width)
		for _, cell := range row.Cells {
			match := cellReference.FindStringSubmatch(cell.Reference)
			if len(match) != 2 {
				return importTable{}, invalidImport("invalid Excel cell reference", nil)
			}
			index := 0
			for _, letter := range match[1] {
				index = index*26 + int(letter-'A'+1)
			}
			index--
			if index < 0 || index >= width || len(values[index]) != 0 {
				return importTable{}, invalidImport("Excel row does not match the header", nil)
			}
			v, err := excelInputValue(cell, shared.Strings)
			if err != nil {
				return importTable{}, invalidImport("Excel imports require exact text or literal formulas", err)
			}
			values[index] = v
		}
		for i, value := range values {
			if number == 0 {
				if len(value) == 0 || json.Unmarshal(value, &table.columns[i]) != nil || table.columns[i] == "" {
					return importTable{}, invalidImport("Excel headers must contain column names", nil)
				}
			} else if len(value) == 0 {
				values[i] = json.RawMessage("null")
			}
		}
		if number > 0 {
			table.rows = append(table.rows, values)
		}
	}
	return table, nil
}
