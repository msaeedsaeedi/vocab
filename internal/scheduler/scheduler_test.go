package scheduler

import (
	"database/sql"
	"strings"
	"testing"
	"time"

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
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			text TEXT NOT NULL,
			definition TEXT NOT NULL DEFAULT '',
			example TEXT NOT NULL DEFAULT '',
			box INTEGER NOT NULL DEFAULT 0,
			next_due TEXT NOT NULL DEFAULT (date('now'))
		)
	`)
	if err != nil {
		t.Fatalf("create table: %v", err)
	}
	return db
}

func TestBoxInterval(t *testing.T) {
	tests := []struct {
		box      int
		expected int
	}{
		{0, 1},
		{1, 2},
		{2, 4},
		{3, 8},
		{4, 16},
		{5, 32},
		{6, 32},
		{-1, 1},
	}
	for _, tc := range tests {
		got := BoxInterval(tc.box)
		if got != tc.expected {
			t.Errorf("BoxInterval(%d) = %d; want %d", tc.box, got, tc.expected)
		}
	}
}

func TestNextDue(t *testing.T) {
	today := time.Now().Format("2006-01-02")
	due := NextDue(0)
	if due < today {
		t.Fatalf("NextDue(0) = %s, expected >= %s", due, today)
	}

	due5 := NextDue(5)
	due0 := NextDue(0)
	if due5 <= due0 {
		t.Fatal("NextDue(5) should be further than NextDue(0)")
	}
}

func TestNextDueFormat(t *testing.T) {
	due := NextDue(0)
	if !strings.Contains(due, "-") {
		t.Fatalf("NextDue returned non-date format: %s", due)
	}
}

func TestRecordFeedbackKnewIt(t *testing.T) {
	db := newTestDB(t)
	db.Exec(`INSERT INTO words (id, text, box, next_due) VALUES (1, 'test', 0, '2026-01-01')`)

	if err := RecordFeedback(db, 1, true); err != nil {
		t.Fatalf("RecordFeedback(true): %v", err)
	}

	var box int
	var nextDue string
	db.QueryRow(`SELECT box, next_due FROM words WHERE id = 1`).Scan(&box, &nextDue)
	if box != 1 {
		t.Fatalf("expected box=1, got %d", box)
	}
	if nextDue <= "2026-01-01" {
		t.Fatalf("expected next_due > 2026-01-01, got %s", nextDue)
	}
}

func TestRecordFeedbackForgot(t *testing.T) {
	db := newTestDB(t)
	db.Exec(`INSERT INTO words (id, text, box, next_due) VALUES (1, 'test', 3, '2026-01-01')`)

	if err := RecordFeedback(db, 1, false); err != nil {
		t.Fatalf("RecordFeedback(false): %v", err)
	}

	var box int
	db.QueryRow(`SELECT box FROM words WHERE id = 1`).Scan(&box)
	if box != 0 {
		t.Fatalf("expected box=0, got %d", box)
	}
}

func TestRecordFeedbackMaxBox(t *testing.T) {
	db := newTestDB(t)
	db.Exec(`INSERT INTO words (id, text, box, next_due) VALUES (1, 'test', 5, '2026-01-01')`)

	if err := RecordFeedback(db, 1, true); err != nil {
		t.Fatalf("RecordFeedback(true) at box 5: %v", err)
	}

	var box int
	db.QueryRow(`SELECT box FROM words WHERE id = 1`).Scan(&box)
	if box != 5 {
		t.Fatalf("expected box=5 (capped), got %d", box)
	}
}

func TestRecordFeedbackNotFound(t *testing.T) {
	db := newTestDB(t)
	err := RecordFeedback(db, 999, true)
	if err == nil {
		t.Fatal("expected error for non-existent word")
	}
}

func TestRecordFeedbackFromBox0Forgot(t *testing.T) {
	db := newTestDB(t)
	db.Exec(`INSERT INTO words (id, text, box, next_due) VALUES (1, 'test', 0, '2026-01-01')`)

	if err := RecordFeedback(db, 1, false); err != nil {
		t.Fatalf("RecordFeedback(false) at box 0: %v", err)
	}

	var box int
	db.QueryRow(`SELECT box FROM words WHERE id = 1`).Scan(&box)
	if box != 0 {
		t.Fatalf("expected box=0 (stays 0), got %d", box)
	}
}
