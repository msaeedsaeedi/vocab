package seed

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/msaeedsaeedi/vocab/internal/word"
	_ "modernc.org/sqlite"
)

func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open :memory:: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS words (
			id             INTEGER PRIMARY KEY AUTOINCREMENT,
			text           TEXT NOT NULL,
			definition     TEXT NOT NULL DEFAULT '',
			example        TEXT NOT NULL DEFAULT '',
			pos            TEXT NOT NULL DEFAULT '',
			box            INTEGER NOT NULL DEFAULT 0,
			next_due       TEXT NOT NULL DEFAULT (date('now')),
			stability      REAL NOT NULL DEFAULT 1.0,
			difficulty     REAL NOT NULL DEFAULT 0.3,
			bkt_alpha      REAL NOT NULL DEFAULT 1.0,
			bkt_beta       REAL NOT NULL DEFAULT 1.0,
			last_reviewed  TEXT NOT NULL DEFAULT '',
			review_count   INTEGER NOT NULL DEFAULT 0,
			lapse_count    INTEGER NOT NULL DEFAULT 0,
			exposure_phase TEXT NOT NULL DEFAULT ''
		)
	`)
	if err != nil {
		t.Fatalf("create table: %v", err)
	}
	return db
}

func writeTempJSON(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "words.json")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write temp json: %v", err)
	}
	return path
}

func TestFromJSON(t *testing.T) {
	db := newTestDB(t)
	path := writeTempJSON(t, `[
		{"word":"alpha","meaning":"first","usage":"Alpha is first"},
		{"word":"beta","meaning":"second","usage":"Beta is second"}
	]`)

	if err := FromJSON(db, path); err != nil {
		t.Fatalf("FromJSON: %v", err)
	}

	all, err := word.GetAll(db)
	if err != nil {
		t.Fatalf("GetAll: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("expected 2 words, got %d", len(all))
	}
	if all[0].Text != "alpha" || all[0].Definition != "first" || all[0].Example != "Alpha is first" {
		t.Fatalf("unexpected word: %+v", all[0])
	}
	if all[0].Box != 0 || all[0].NextDue != "1970-01-01" {
		t.Fatalf("expected box=0 next_due=1970-01-01, got box=%d next_due=%s", all[0].Box, all[0].NextDue)
	}
}

func TestFromJSONEmpty(t *testing.T) {
	db := newTestDB(t)
	path := writeTempJSON(t, `[]`)
	if err := FromJSON(db, path); err != nil {
		t.Fatalf("FromJSON empty: %v", err)
	}
	n, _ := word.Count(db)
	if n != 0 {
		t.Fatalf("expected 0 words, got %d", n)
	}
}

func TestFromJSONFileNotFound(t *testing.T) {
	db := newTestDB(t)
	err := FromJSON(db, "/nonexistent/words.json")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestFromJSONInvalidJSON(t *testing.T) {
	db := newTestDB(t)
	path := writeTempJSON(t, `{bad json`)
	err := FromJSON(db, path)
	if err == nil {
		t.Fatal("expected error for invalid json")
	}
}

func TestNeededTrue(t *testing.T) {
	db := newTestDB(t)
	needed, err := Needed(db)
	if err != nil {
		t.Fatalf("Needed: %v", err)
	}
	if !needed {
		t.Fatal("expected needed=true for empty db")
	}
}

func TestNeededFalse(t *testing.T) {
	db := newTestDB(t)
	db.Exec(`INSERT INTO words (text) VALUES ('test')`)
	needed, err := Needed(db)
	if err != nil {
		t.Fatalf("Needed: %v", err)
	}
	if needed {
		t.Fatal("expected needed=false for non-empty db")
	}
}
