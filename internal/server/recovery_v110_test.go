package server

import (
	"database/sql"
	"net/http"
	"path/filepath"
	"testing"
)

func TestSQLiteWALCellMutationApprovalAndRecovery(t *testing.T) {
	srv, root, _ := setupTestServer(t)
	sess := srv.Session()
	db, err := sql.Open("sqlite", filepath.Join(root, "wal.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	const fixture = `
		PRAGMA journal_mode=WAL;
		PRAGMA wal_autocheckpoint=0;
		CREATE TABLE authors (id INTEGER PRIMARY KEY, name TEXT NOT NULL);
		CREATE TABLE books (id INTEGER PRIMARY KEY, author INTEGER REFERENCES authors(id), price TEXT, note TEXT);
		CREATE INDEX books_by_author ON books(author);
		CREATE VIEW listed_books AS SELECT id,price FROM books;
		INSERT INTO authors VALUES (1,'Ada');
		INSERT INTO books VALUES (9007199254740993,1,'0.00',NULL);
	`
	if _, err := db.Exec(fixture); err != nil {
		t.Fatal(err)
	}
	target := map[string]any{"engine": "sqlite", "sqlite_path": "wal.db"}
	post := func(path, csrf string, body map[string]any) (int, map[string]any, string) {
		t.Helper()
		res, out, raw := doAPIReq(t, http.MethodPost, sess.URL+path, sess.BearerToken, csrf, body)
		return res.StatusCode, out, raw
	}
	edit := func(value any) map[string]any {
		return map[string]any{"table": "books", "column": "price", "key": map[string]any{"id": "9007199254740993"},
			"before": "0.00", "value_kind": "decimal", "value": value}
	}
	for _, invalid := range []any{0.25, "0e0", "1E-2"} {
		status, _, raw := post("/api/db/mutate/plan", sess.CSRFToken, map[string]any{"target": target, "edit": edit(invalid)})
		if status != http.StatusUnprocessableEntity {
			t.Fatalf("invalid numeric input returned %d: %s", status, raw)
		}
	}
	plan := map[string]any{"target": target, "edit": edit("1.00")}
	status, _, raw := post("/api/db/mutate/plan", "", plan)
	if status != http.StatusForbidden {
		t.Fatalf("no CSRF returned %d: %s", status, raw)
	}
	status, planned, raw := post("/api/db/mutate/plan", sess.CSRFToken, plan)
	if status != http.StatusOK {
		t.Fatalf("WAL plan returned %d: %s", status, raw)
	}
	token := planned["approval_token"]
	snapshot := planned["snapshot"].(map[string]any)["snapshot_id"]
	status, _, raw = post("/api/db/mutate/apply", sess.CSRFToken, map[string]any{"target": target, "edit": edit("2.00"), "approval_token": token})
	if status != http.StatusForbidden {
		t.Fatalf("changed value returned %d: %s", status, raw)
	}

	codes := make(chan int, 2)
	for range 2 {
		go func() {
			code, _, _ := post("/api/db/mutate/apply", sess.CSRFToken, map[string]any{"target": target, "edit": edit("1.00"), "approval_token": token})
			codes <- code
		}()
	}
	first, second := <-codes, <-codes
	if !((first == 201 && second == 409) || (first == 409 && second == 201)) {
		t.Fatalf("approval race returned %d and %d", first, second)
	}
	var price string
	if err := db.QueryRow("SELECT price FROM books").Scan(&price); err != nil || price != "1.00" {
		t.Fatalf("cell value %q, %v", price, err)
	}
	restore := map[string]any{"target": target, "snapshot_id": snapshot}
	status, _, raw = post("/api/db/snapshots/restore", sess.CSRFToken, restore)
	if status != http.StatusForbidden {
		t.Fatalf("unapproved restore returned %d: %s", status, raw)
	}
	status, rp, raw := post("/api/db/snapshots/restore/plan", sess.CSRFToken, restore)
	if status != http.StatusOK {
		t.Fatalf("restore plan returned %d: %s", status, raw)
	}
	restore["approval_token"] = rp["approval_token"]
	status, _, raw = post("/api/db/snapshots/restore", sess.CSRFToken, restore)
	if status != http.StatusCreated {
		t.Fatalf("restore returned %d: %s", status, raw)
	}
	status, _, raw = post("/api/db/snapshots/restore", sess.CSRFToken, restore)
	if status != http.StatusConflict {
		t.Fatalf("restore replay returned %d: %s", status, raw)
	}
	var id int64
	var note sql.NullString
	if err := db.QueryRow("SELECT l.id,l.price,b.note FROM listed_books l JOIN books b ON b.id=l.id").Scan(&id, &price, &note); err != nil ||
		id != 9007199254740993 || price != "0.00" || note.Valid {
		t.Fatalf("restored row id=%d price=%q note=%v err=%v", id, price, note, err)
	}
	var indexCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM sqlite_schema WHERE name='books_by_author'").Scan(&indexCount); err != nil || indexCount != 1 {
		t.Fatalf("restored index count=%d err=%v", indexCount, err)
	}
	for _, engine := range []string{"postgres", "mariadb"} {
		status, _, raw := post("/api/db/mutate/plan", sess.CSRFToken, map[string]any{"target": map[string]any{"engine": engine}, "sql": "DELETE FROM books"})
		if status != http.StatusForbidden {
			t.Fatalf("%s write plan returned %d: %s", engine, status, raw)
		}
	}
}
