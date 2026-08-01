package word

import (
	"testing"

	"github.com/msaeedsaeedi/vocab/internal/database"
)

func newTestDB(t *testing.T) *database.DB {
	t.Helper()
	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func item(id string, due string) Word {
	return Word{LexemeID: id, DatasetVersion: "0.4.0", NextDue: due}
}

func TestInsertStoresOnlyLearnerReference(t *testing.T) {
	db := newTestDB(t)
	w := item("en:record:noun", "2026-01-01")
	w.Text, w.Definition, w.Example = "record", "a thing", "a record exists"
	if err := Insert(db, &w); err != nil {
		t.Fatal(err)
	}
	var lexeme, dataset string
	if err := db.QueryRow("SELECT lexeme_id, dataset_version FROM learning_items WHERE id = ?", w.ID).Scan(&lexeme, &dataset); err != nil {
		t.Fatal(err)
	}
	if lexeme != w.LexemeID || dataset != w.DatasetVersion {
		t.Fatalf("stored %q/%q", lexeme, dataset)
	}
	var columns int
	if err := db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('learning_items') WHERE name IN ('text', 'definition', 'example')").Scan(&columns); err != nil {
		t.Fatal(err)
	}
	if columns != 0 {
		t.Fatal("learner table contains canonical text columns")
	}
}

func TestDueItemsAndAdaptiveState(t *testing.T) {
	db := newTestDB(t)
	for _, w := range []Word{item("a", "2026-01-01"), item("b", "2026-01-03")} {
		if err := Insert(db, &w); err != nil {
			t.Fatal(err)
		}
	}
	due, err := GetDueWords(db, "2026-01-02")
	if err != nil {
		t.Fatal(err)
	}
	if len(due) != 1 || due[0].LexemeID != "a" {
		t.Fatalf("due=%+v", due)
	}
	if err := UpdateAdaptive(db, due[0].ID, 2, .2, 3, 1, 1, 0, "2026-01-02 12:00:00", "2026-01-05", "recall"); err != nil {
		t.Fatal(err)
	}
	got, err := GetWord(db, due[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Stability != 2 || got.BktAlpha != 3 || got.ExposurePhase != "recall" {
		t.Fatalf("got %+v", got)
	}
	if err := InsertReviewLog(db, &ReviewLog{WordID: got.ID, Rating: 2, Stability: 2}); err != nil {
		t.Fatal(err)
	}
	var events int
	if err := db.QueryRow("SELECT COUNT(*) FROM review_events WHERE learning_item_id = ?", got.ID).Scan(&events); err != nil {
		t.Fatal(err)
	}
	if events != 1 {
		t.Fatalf("events=%d", events)
	}
}
