package seed

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/msaeedsaeedi/vocab/internal/database"
	"github.com/msaeedsaeedi/vocab/internal/word"
)

func newTestDB(t *testing.T) *database.DB {
	t.Helper()
	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatalf("database.Open(\":memory:\"): %v", err)
	}
	t.Cleanup(func() { db.Close() })
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
