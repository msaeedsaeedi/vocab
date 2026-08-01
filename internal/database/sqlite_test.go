package database

import (
	"os"
	"testing"
)

func TestOpen(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open(\":memory:\"): %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		t.Fatalf("Ping: %v", err)
	}

	var tables int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='learning_items'`).Scan(&tables); err != nil {
		t.Fatalf("count tables: %v", err)
	}
	if tables != 1 {
		t.Fatal("learning_items table not found after Open")
	}
}

func TestOpenFile(t *testing.T) {
	path := t.TempDir() + "/test.db"
	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	db.Close()

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("db file was not created")
	}
}

func TestOpenInvalidPath(t *testing.T) {
	_, err := Open("/nonexistent/deep/db.sqlite")
	if err == nil {
		t.Fatal("expected error for invalid path")
	}
}

func TestReopenIdempotent(t *testing.T) {
	path := t.TempDir() + "/test.db"
	db, err := Open(path)
	if err != nil {
		t.Fatalf("first Open: %v", err)
	}
	db.Close()

	db, err = Open(path)
	if err != nil {
		t.Fatalf("second Open: %v", err)
	}
	db.Close()
}

func TestSchemaTablesCreated(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	tables := map[string]bool{
		"learning_items":     false,
		"review_events":      false,
		"exposures":          false,
		"engagement":         false,
		"settings":           false,
		"daemon_state":       false,
		"installed_datasets": false,
	}
	for name := range tables {
		var count int
		if err := db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?", name).Scan(&count); err != nil {
			t.Fatalf("query %s: %v", name, err)
		}
		tables[name] = count == 1
	}
	for name, found := range tables {
		if !found {
			t.Errorf("table %s not created", name)
		}
	}
}
