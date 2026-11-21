package dbstudio

import (
	"context"
	"database/sql"
	"encoding/csv"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"groxy-desk/internal/groxybridge"
	"groxy-desk/internal/security"
)

func TestSQLiteWorkbenchTransfers(t *testing.T) {
	ctx := context.Background()
	workspace := t.TempDir()
	redactor := security.NewSecretRedactor()
	approvals, err := groxybridge.NewApprovalManager(t.TempDir(), redactor)
	if err != nil {
		t.Fatal(err)
	}
	m := NewManager(workspace, approvals, redactor)
	defer m.CloseAll()
	makeTarget := func(name string) TargetSpec {
		t.Helper()
		db, err := sql.Open("sqlite", filepath.Join(workspace, name))
		if err != nil {
			t.Fatal(err)
		}
		defer db.Close()
		_, err = db.Exec(`CREATE TABLE books (id INTEGER PRIMARY KEY, price TEXT, label TEXT, missing TEXT);`)
		if err != nil {
			t.Fatal(err)
		}
		if name == "source.db" {
			_, err = db.Exec(`INSERT INTO books VALUES (9007199254740993,'0.00',' =SUM(1,2)_x0041_',NULL),(9007199254740994,'0.10','''=literal','\N');`)
			if err != nil {
				t.Fatal(err)
			}
		}
		return TargetSpec{Engine: "sqlite", SQLitePath: name}
	}
	source := makeTarget("source.db")
	rows := RowsRequest{Target: source, Table: "books"}
	before, err := m.BrowseRows(ctx, rows)
	if err != nil {
		t.Fatal(err)
	}

	for _, format := range []string{"csv", "json", "xlsx", "sql"} {
		t.Run(format, func(t *testing.T) {
			path := "books." + format
			export := ExportPageRequest{RowsRequest: rows, Path: path}
			if _, err := m.ExportPage(ctx, export); err != nil {
				t.Fatal(err)
			}
			if _, err := m.ExportPage(ctx, export); security.ErrorStatus(err, 0) != 409 {
				t.Fatalf("expected existing export protection, got %v", err)
			}
			if format == "csv" {
				raw, err := os.ReadFile(filepath.Join(workspace, path))
				if err != nil {
					t.Fatal(err)
				}
				entries, err := csv.NewReader(strings.NewReader(string(raw))).ReadAll()
				if err != nil || entries[1][2] != "' =SUM(1,2)_x0041_" {
					t.Fatalf("unescaped or invalid CSV %#v, %v", entries, err)
				}
			}
			target := makeTarget(format + ".db")
			request := ImportRequest{Target: target, Table: "books", Path: path}
			plan, err := m.PlanImport(ctx, request)
			if err != nil {
				t.Fatal(err)
			}
			request.ApprovalToken = plan["approval_token"].(string)
			if _, err := m.ApplyImport(ctx, request); err != nil {
				t.Fatal(err)
			}
			after, err := m.BrowseRows(ctx, RowsRequest{Target: target, Table: "books"})
			if err != nil || !reflect.DeepEqual(after["rows"], before["rows"]) {
				t.Fatalf("expected rows %#v, got %#v, %v", before["rows"], after["rows"], err)
			}
		})
	}
	if err := os.WriteFile(filepath.Join(workspace, "invalid.json"), []byte(`[{"id":9.007199254740993e15,"price":"0.00"}]`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := m.PlanImport(ctx, ImportRequest{Target: source, Table: "books", Path: "invalid.json"}); security.ErrorStatus(err, 0) != 422 {
		t.Fatalf("expected raw numeric rejection, got %v", err)
	}
	if _, err := m.ExportPage(ctx, ExportPageRequest{RowsRequest: rows, Path: "../books.csv"}); security.ErrorCode(err, "") != security.CodeGrabE401 {
		t.Fatalf("expected workspace boundary rejection, got %v", err)
	}
}
