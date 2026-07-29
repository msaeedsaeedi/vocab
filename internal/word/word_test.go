package word

import (
	"database/sql"
	"os"
	"testing"

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

func TestInsert(t *testing.T) {
	db := newTestDB(t)
	w := Word{Text: "serendipity", Definition: "luck", Example: "Pure serendipity", Box: 0, NextDue: "2026-01-01"}
	if err := Insert(db, &w); err != nil {
		t.Fatalf("Insert: %v", err)
	}
	if w.ID == 0 {
		t.Fatal("expected non-zero ID after insert")
	}

	var got Word
	err := db.QueryRow(`SELECT id, text, definition, example, box, next_due FROM words WHERE id = ?`, w.ID).
		Scan(&got.ID, &got.Text, &got.Definition, &got.Example, &got.Box, &got.NextDue)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if got.Text != "serendipity" || got.Definition != "luck" || got.Box != 0 {
		t.Fatalf("got %+v", got)
	}
}

func TestGetDueWords(t *testing.T) {
	db := newTestDB(t)

	words := []Word{
		{Text: "alpha", NextDue: "2026-01-01"},
		{Text: "beta", NextDue: "2026-01-03"},
		{Text: "gamma", NextDue: "2026-01-05"},
	}
	for _, w := range words {
		if err := Insert(db, &w); err != nil {
			t.Fatalf("insert %s: %v", w.Text, err)
		}
	}

	due, err := GetDueWords(db, "2026-01-02")
	if err != nil {
		t.Fatalf("GetDueWords: %v", err)
	}
	if len(due) != 1 || due[0].Text != "alpha" {
		t.Fatalf("expected 1 due word (alpha), got %d: %+v", len(due), due)
	}

	due, err = GetDueWords(db, "2026-01-05")
	if err != nil {
		t.Fatalf("GetDueWords: %v", err)
	}
	if len(due) != 3 {
		t.Fatalf("expected 3 due words, got %d", len(due))
	}
}

func TestGetDueWordsOrderByBox(t *testing.T) {
	db := newTestDB(t)

	db.Exec(`INSERT INTO words (text, box, next_due) VALUES ('a', 2, '2026-01-01')`)
	db.Exec(`INSERT INTO words (text, box, next_due) VALUES ('b', 0, '2026-01-01')`)
	db.Exec(`INSERT INTO words (text, box, next_due) VALUES ('c', 1, '2026-01-01')`)

	due, err := GetDueWords(db, "2026-01-01")
	if err != nil {
		t.Fatalf("GetDueWords: %v", err)
	}
	if len(due) != 3 {
		t.Fatalf("expected 3 due, got %d", len(due))
	}
	if due[0].Text != "b" || due[1].Text != "c" || due[2].Text != "a" {
		t.Fatalf("expected box order b(0) c(1) a(2), got %+v", due)
	}
}

func TestGetDueWordsEmpty(t *testing.T) {
	db := newTestDB(t)
	due, err := GetDueWords(db, "2026-01-01")
	if err != nil {
		t.Fatalf("GetDueWords: %v", err)
	}
	if len(due) != 0 {
		t.Fatal("expected empty result")
	}
}

func TestUpdateFeedback(t *testing.T) {
	db := newTestDB(t)
	db.Exec(`INSERT INTO words (text, next_due) VALUES ('test', '2026-01-01')`)

	if err := UpdateFeedback(db, 1, 2, "2026-02-01"); err != nil {
		t.Fatalf("UpdateFeedback: %v", err)
	}

	var box int
	var nextDue string
	db.QueryRow(`SELECT box, next_due FROM words WHERE id = 1`).Scan(&box, &nextDue)
	if box != 2 || nextDue != "2026-02-01" {
		t.Fatalf("got box=%d next_due=%s, expected box=2 next_due=2026-02-01", box, nextDue)
	}
}

func TestUpdateFeedbackNotFound(t *testing.T) {
	db := newTestDB(t)
	err := UpdateFeedback(db, 999, 1, "2026-01-01")
	if err == nil {
		t.Fatal("expected error for non-existent word")
	}
}

func TestGetAll(t *testing.T) {
	db := newTestDB(t)
	db.Exec(`INSERT INTO words (text) VALUES ('a')`)
	db.Exec(`INSERT INTO words (text) VALUES ('b')`)

	all, err := GetAll(db)
	if err != nil {
		t.Fatalf("GetAll: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("expected 2, got %d", len(all))
	}
}

func TestCount(t *testing.T) {
	db := newTestDB(t)
	n, err := Count(db)
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if n != 0 {
		t.Fatalf("expected 0, got %d", n)
	}

	db.Exec(`INSERT INTO words (text) VALUES ('a')`)
	n, _ = Count(db)
	if n != 1 {
		t.Fatalf("expected 1, got %d", n)
	}
}

func TestCountDue(t *testing.T) {
	db := newTestDB(t)
	db.Exec(`INSERT INTO words (text, next_due) VALUES ('a', '2026-01-01')`)
	db.Exec(`INSERT INTO words (text, next_due) VALUES ('b', '2026-01-05')`)

	n, err := CountDue(db, "2026-01-02")
	if err != nil {
		t.Fatalf("CountDue: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected 1 due, got %d", n)
	}
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
