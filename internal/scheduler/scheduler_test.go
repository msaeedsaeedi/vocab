package scheduler

import (
	"database/sql"
	"math"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"github.com/msaeedsaeedi/vocab/internal/word"
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
		);
		CREATE TABLE IF NOT EXISTS review_log (
			id             INTEGER PRIMARY KEY AUTOINCREMENT,
			word_id        INTEGER NOT NULL,
			rating         INTEGER NOT NULL,
			elapsed_hours  REAL NOT NULL DEFAULT 0,
			stability      REAL NOT NULL DEFAULT 1.0,
			timestamp      TEXT NOT NULL DEFAULT (datetime('now'))
		);
	`)
	if err != nil {
		t.Fatalf("create tables: %v", err)
	}
	return db
}

func insertTestWord(db *sql.DB, id int64, stability, difficulty float64) {
	db.Exec(`INSERT INTO words (id, text, box, next_due, stability, difficulty)
		VALUES (?, 'test', 0, '2026-01-01', ?, ?)`, id, stability, difficulty)
}

func TestRecallProbability(t *testing.T) {
	p := RecallProbability(1.0, 24.0)
	if p <= 0 || p >= 1.0 {
		t.Errorf("recall after 1 day at stability=1.0: got %f, expected 0 < p < 1", p)
	}

	pFar := RecallProbability(1.0, 240.0)
	if pFar >= p {
		t.Error("recall should decrease over time")
	}

	if got := RecallProbability(0, 24); got != 1.0 {
		t.Errorf("zero stability should return 1.0, got %f", got)
	}
}

func TestBktSkill(t *testing.T) {
	if got := BktSkill(0, 0); got != 0.5 {
		t.Errorf("uninitialized BKT should return 0.5, got %f", got)
	}
	if got := BktSkill(3, 1); got != 0.75 {
		t.Errorf("3/4 should return 0.75, got %f", got)
	}
	if got := BktSkill(10, 0); got != 1.0 {
		t.Errorf("10/10 should return 1.0, got %f", got)
	}
}

func TestMemoryStateUpdate(t *testing.T) {
	m := &memoryState{stability: 1.0, difficulty: 0.3, alpha: 1.0, beta: 1.0}

	m.update(2, 1.0)
	if m.alpha != 2.0 {
		t.Errorf("correct answer: alpha should increment, got %f", m.alpha)
	}
	if m.stability <= 1.0 {
		t.Errorf("correct answer: stability should increase, got %f", m.stability)
	}
}

func TestMemoryStateForgot(t *testing.T) {
	m := &memoryState{stability: 5.0, difficulty: 0.3, alpha: 5.0, beta: 1.0}

	m.update(0, 5.0)
	if m.beta != 2.0 {
		t.Errorf("forgot: beta should increment, got %f", m.beta)
	}
	if m.stability != 0.5 {
		t.Errorf("forgot: stability should reset to 0.5, got %f", m.stability)
	}
	if m.difficulty <= 0.3 {
		t.Errorf("forgot: difficulty should increase, got %f", m.difficulty)
	}
}

func TestNextInterval(t *testing.T) {
	m := &memoryState{stability: 2.0, difficulty: 0.3, alpha: 3, beta: 1}

	if got := m.nextInterval(0); got != 0.25 {
		t.Errorf("rating 0: interval should be 0.25, got %f", got)
	}
	if got := m.nextInterval(3); got <= m.nextInterval(2) {
		t.Errorf("rating 3 interval should be >= rating 2 interval")
	}
}

func TestPriority(t *testing.T) {
	hardWord := word.Word{Stability: 0.5, Difficulty: 0.8, BktAlpha: 2, BktBeta: 8}
	easyWord := word.Word{Stability: 10.0, Difficulty: 0.1, BktAlpha: 10, BktBeta: 1}

	hardScore := Priority(&hardWord, 12)
	easyScore := Priority(&easyWord, 12)

	if hardScore <= easyScore {
		t.Errorf("hard/unstable word should have higher priority (hard=%.3f easy=%.3f)", hardScore, easyScore)
	}
}

func TestScheduleReviewCorrect(t *testing.T) {
	db := newTestDB(t)
	insertTestWord(db, 1, 1.0, 0.3)

	w, err := word.GetWord(db, 1)
	if err != nil {
		t.Fatalf("get word: %v", err)
	}

	days, err := ScheduleReview(db, w, 2)
	if err != nil {
		t.Fatalf("schedule review: %v", err)
	}

	if days <= 0 {
		t.Errorf("expected positive days, got %f", days)
	}

	updated, err := word.GetWord(db, 1)
	if err != nil {
		t.Fatalf("get updated word: %v", err)
	}
	if updated.ReviewCount != 1 {
		t.Errorf("review_count should be 1, got %d", updated.ReviewCount)
	}
	if updated.BktAlpha != 2.0 {
		t.Errorf("bkt_alpha should be 2.0, got %f", updated.BktAlpha)
	}
	if updated.Stability <= 1.0 {
		t.Errorf("stability should increase from 1.0, got %f", updated.Stability)
	}

	var logCount int
	db.QueryRow(`SELECT COUNT(*) FROM review_log WHERE word_id = 1`).Scan(&logCount)
	if logCount != 1 {
		t.Errorf("expected 1 review log entry, got %d", logCount)
	}
}

func TestScheduleReviewForgot(t *testing.T) {
	db := newTestDB(t)
	insertTestWord(db, 1, 10.0, 0.1)

	w, err := word.GetWord(db, 1)
	if err != nil {
		t.Fatalf("get word: %v", err)
	}

	days, err := ScheduleReview(db, w, 0)
	if err != nil {
		t.Fatalf("schedule review: %v", err)
	}

	if days != 0.25 {
		t.Errorf("rating 0 should give 0.25 days, got %f", days)
	}

	updated, _ := word.GetWord(db, 1)
	if updated.LapseCount != 1 {
		t.Errorf("lapse_count should be 1, got %d", updated.LapseCount)
	}
	if updated.Stability != 0.5 {
		t.Errorf("stability should reset to 0.5, got %f", updated.Stability)
	}
}

func TestSelectNextWord(t *testing.T) {
	now := time.Now().Format("2006-01-02")
	words := []word.Word{
		{ID: 1, NextDue: now, Stability: 10.0, Difficulty: 0.1, BktAlpha: 10, BktBeta: 1},
		{ID: 2, NextDue: now, Stability: 0.5, Difficulty: 0.8, BktAlpha: 2, BktBeta: 8},
		{ID: 3, NextDue: now, Stability: 2.0, Difficulty: 0.4, BktAlpha: 5, BktBeta: 3},
	}

	selected := SelectNextWord(nil, words)
	if selected == nil {
		t.Fatal("expected non-nil result")
	}
	if selected.ID != 2 {
		t.Errorf("expected word 2 (hardest/most urgent), got %d", selected.ID)
	}

	var single []word.Word
	if got := SelectNextWord(nil, single); got != nil {
		t.Error("empty list should return nil")
	}

	singleWord := []word.Word{{ID: 1, Stability: 1.0}}
	if got := SelectNextWord(nil, singleWord); got == nil || got.ID != 1 {
		t.Error("single word should be selected")
	}
}

func TestScheduleReviewNonExistent(t *testing.T) {
	db := newTestDB(t)
	w := &word.Word{ID: 999, Stability: 1.0, Difficulty: 0.3, BktAlpha: 1, BktBeta: 1}
	_, err := ScheduleReview(db, w, 2)
	if err != nil {
		t.Fatalf("expected no error for non-existent word (idempotent), got: %v", err)
	}
}

func TestPriorityOverdueBias(t *testing.T) {
	w := word.Word{Stability: 2.0, Difficulty: 0.3, BktAlpha: 5, BktBeta: 2}
	normal := Priority(&w, 2)
	overdue := Priority(&w, 100)

	if overdue <= normal {
		t.Errorf("overdue word should have higher priority (normal=%.3f overdue=%.3f)", normal, overdue)
	}
}

func TestMemoryStateConverges(t *testing.T) {
	m := &memoryState{stability: 1.0, difficulty: 0.5, alpha: 1.0, beta: 1.0}

	for i := 0; i < 10; i++ {
		m.update(3, m.stability)
		if m.stability > 100 {
			break
		}
	}

	if m.difficulty >= 0.5 {
		t.Errorf("rating 3 (easy) should lower difficulty, got %f", m.difficulty)
	}
	if m.alpha <= 5 {
		t.Errorf("alpha should have grown, got %f", m.alpha)
	}
}

func TestRecallProbabilityEdge(t *testing.T) {
	if got := RecallProbability(5.0, 0); got != 1.0 {
		t.Errorf("zero elapsed should return 1.0, got %f", got)
	}
	if got := RecallProbability(1.0, 1000); got > 0.01 {
		t.Errorf("very long elapsed should approach 0, got %f", got)
	}
}

func TestBktSkillMonotonic(t *testing.T) {
	better := BktSkill(10, 1)
	worse := BktSkill(1, 10)
	if better <= worse {
		t.Errorf("10/1 should be > 1/10, got %.3f vs %.3f", better, worse)
	}
}

func TestPriorityDeterministic(t *testing.T) {
	w := word.Word{Stability: 3.0, Difficulty: 0.3, BktAlpha: 4, BktBeta: 2}
	a := Priority(&w, 5)
	b := Priority(&w, 5)
	if math.Abs(a-b) > 0.0001 {
		t.Errorf("priority should be deterministic: %.6f vs %.6f", a, b)
	}
}
