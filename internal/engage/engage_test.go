package engage

import (
	"math"
	"testing"
	"time"

	"github.com/msaeedsaeedi/vocab/internal/database"
	"github.com/msaeedsaeedi/vocab/internal/state"
)

func newTestTracker(t *testing.T) (*Tracker, *database.DB, *state.Store) {
	t.Helper()
	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatalf("Open(:memory:): %v", err)
	}
	t.Cleanup(func() { db.Close() })
	store := state.New(db)
	return New(db, store), db, store
}

func seedEngagement(t *testing.T, db *database.DB, rows []struct {
	hour     int
	sent     int
	answered int
	score    float64
}) {
	t.Helper()
	for _, r := range rows {
		if _, err := db.Exec(
			`INSERT INTO engagement (hour, score, sample_count, notifications_sent, notifications_answered)
			 VALUES (?, ?, 0, ?, ?)`,
			r.hour, r.score, r.sent, r.answered,
		); err != nil {
			t.Fatalf("seed engagement hour %d: %v", r.hour, err)
		}
	}
}

func TestRecordEngagement(t *testing.T) {
	tr, db, _ := newTestTracker(t)

	tr.RecordEngagement()

	hour := time.Now().Hour()
	var score float64
	var count int
	if err := db.QueryRow(
		`SELECT score, sample_count FROM engagement WHERE hour = ?`, hour,
	).Scan(&score, &count); err != nil {
		t.Fatalf("query engagement: %v", err)
	}
	if score != 1.0 || count != 1 {
		t.Fatalf("after one engagement: score=%v count=%d, want 1 1", score, count)
	}

	tr.RecordEngagement()
	if err := db.QueryRow(
		`SELECT score, sample_count FROM engagement WHERE hour = ?`, hour,
	).Scan(&score, &count); err != nil {
		t.Fatalf("query engagement: %v", err)
	}
	if count != 2 || score != 1.0 {
		t.Fatalf("after two engagements: score=%v count=%d, want 1 2", score, count)
	}
}

func TestRecordNotificationStats(t *testing.T) {
	tr, db, _ := newTestTracker(t)

	tr.RecordNotificationSent()
	tr.RecordNotificationSent()
	tr.RecordNotificationAnswered()

	hour := time.Now().Hour()
	var sent, answered int
	if err := db.QueryRow(
		`SELECT notifications_sent, notifications_answered FROM engagement WHERE hour = ?`, hour,
	).Scan(&sent, &answered); err != nil {
		t.Fatalf("query engagement: %v", err)
	}
	if sent != 2 || answered != 1 {
		t.Fatalf("notif stats sent=%d answered=%d, want 2 1", sent, answered)
	}
}

func TestPutTimes(t *testing.T) {
	tr, _, store := newTestTracker(t)

	tm := time.Date(2026, 8, 8, 9, 30, 0, 0, time.UTC)
	tr.PutLastWordTime(tm)
	tr.PutLastNotificationTime(tm)

	if got, ok := store.GetSetting("last_word_time"); !ok || got != tm.Format(time.RFC3339) {
		t.Fatalf("last_word_time=%q ok=%v, want %q", got, ok, tm.Format(time.RFC3339))
	}
	if got, ok := store.GetSetting("last_notify_time"); !ok || got != tm.Format(time.RFC3339) {
		t.Fatalf("last_notify_time=%q ok=%v, want %q", got, ok, tm.Format(time.RFC3339))
	}
}

func TestActiveWindowDefaults(t *testing.T) {
	tr, _, _ := newTestTracker(t)

	start, end := tr.ActiveWindow()
	if start != defaultActiveStart || end != defaultActiveEnd {
		t.Fatalf("ActiveWindow on empty DB = (%d, %d), want (%d, %d)",
			start, end, defaultActiveStart, defaultActiveEnd)
	}
}

func TestActiveWindowSparseSignalFallsBackToDefaults(t *testing.T) {
	tr, db, _ := newTestTracker(t)
	// A single engaged hour cannot support the minimum six-hour window.
	seedEngagement(t, db, []struct {
		hour     int
		sent     int
		answered int
		score    float64
	}{{12, 10, 8, 0.9}})

	start, end := tr.ActiveWindow()
	if start != defaultActiveStart || end != defaultActiveEnd {
		t.Fatalf("ActiveWindow with sparse signal = (%d, %d), want defaults (%d, %d)",
			start, end, defaultActiveStart, defaultActiveEnd)
	}
}

func TestActiveWindowUniformAllDay(t *testing.T) {
	tr, db, _ := newTestTracker(t)
	rows := make([]struct {
		hour     int
		sent     int
		answered int
		score    float64
	}, 24)
	for h := 0; h < 24; h++ {
		rows[h] = struct {
			hour     int
			sent     int
			answered int
			score    float64
		}{h, 10, 8, 0.9}
	}
	seedEngagement(t, db, rows)

	start, end := tr.ActiveWindow()
	if start != 0 || end != 24 {
		t.Fatalf("ActiveWindow with all-day engagement = (%d, %d), want (0, 24)", start, end)
	}
}

func TestHourlyScoresEmpty(t *testing.T) {
	tr, _, _ := newTestTracker(t)

	scores := tr.hourlyScores()
	for h := 0; h < 24; h++ {
		if scores[h] != 0 {
			t.Fatalf("hourlyScores[%d] = %v, want 0", h, scores[h])
		}
	}
}

func TestHourlyScoresPrefersNotificationData(t *testing.T) {
	tr, db, _ := newTestTracker(t)
	seedEngagement(t, db, []struct {
		hour     int
		sent     int
		answered int
		score    float64
	}{{10, 10, 8, 0.9}})

	scores := tr.hourlyScores()
	want := float64(8+2) / float64(10+4)
	if scores[10] != want {
		t.Fatalf("hourlyScores[10] = %v, want notification-derived %v (not score column)", scores[10], want)
	}
	for h := 0; h < 24; h++ {
		if h != 10 && scores[h] != 0 {
			t.Fatalf("hourlyScores[%d] = %v, want 0", h, scores[h])
		}
	}
}

func TestHourlyScoresFallsBackToScoreColumn(t *testing.T) {
	tr, db, _ := newTestTracker(t)
	// Score column populated, but no notification activity.
	seedEngagement(t, db, []struct {
		hour     int
		sent     int
		answered int
		score    float64
	}{{10, 0, 0, 0.9}})

	scores := tr.hourlyScores()
	if scores[10] != 0.9 {
		t.Fatalf("hourlyScores[10] = %v, want 0.9 from score column", scores[10])
	}
}

func TestWordsPerDayDefault(t *testing.T) {
	tr, _, store := newTestTracker(t)

	n := tr.WordsPerDay()
	if n != defaultWordsPerDay {
		t.Fatalf("WordsPerDay on empty DB = %d, want %d", n, defaultWordsPerDay)
	}
	if got, ok := store.GetSetting(settingWordsPerDay); !ok || got != "3" {
		t.Fatalf("words_per_day persisted = %q ok=%v, want \"3\"", got, ok)
	}
}

func TestWordsPerDayScalesWithEngagement(t *testing.T) {
	tr, db, _ := newTestTracker(t)
	rows := make([]struct {
		hour     int
		sent     int
		answered int
		score    float64
	}, 24)
	for h := 0; h < 24; h++ {
		rows[h] = struct {
			hour     int
			sent     int
			answered int
			score    float64
		}{h, 10, 8, 0.9}
	}
	seedEngagement(t, db, rows)

	n := tr.WordsPerDay()
	// window score = min(2 * 10/14, 1.5) = 1.4286; 3 * 1.4286 rounds to 4.
	if n != 4 {
		t.Fatalf("WordsPerDay with strong engagement = %d, want 4", n)
	}
}

func TestWordsPerDaySmoothsAgainstPrevious(t *testing.T) {
	tr, db, store := newTestTracker(t)
	if err := store.SetSetting(settingWordsPerDay, "2"); err != nil {
		t.Fatalf("preset words_per_day: %v", err)
	}
	rows := make([]struct {
		hour     int
		sent     int
		answered int
		score    float64
	}, 24)
	for h := 0; h < 24; h++ {
		rows[h] = struct {
			hour     int
			sent     int
			answered int
			score    float64
		}{h, 10, 8, 0.9}
	}
	seedEngagement(t, db, rows)

	// Unsmoothed it would be 4, but it must stay within prev +/- 1 -> 3.
	n := tr.WordsPerDay()
	if n != 3 {
		t.Fatalf("WordsPerDay with previous value 2 = %d, want 3 (smoothed)", n)
	}
	if got, ok := store.GetSetting(settingWordsPerDay); !ok || got != "3" {
		t.Fatalf("words_per_day persisted = %q ok=%v, want \"3\"", got, ok)
	}
}

func TestInterWordMinutes(t *testing.T) {
	tr, _, _ := newTestTracker(t)

	cases := []struct {
		activeHours, wordsPerDay int
		want                     int
	}{
		{14, 3, 180}, // 210 -> clamped to max
		{6, 4, 72},
		{2, 8, 45}, // 13 -> clamped to min
		{8, 3, 120},
	}
	for _, c := range cases {
		got := tr.InterWordMinutes(c.activeHours, c.wordsPerDay)
		if got != c.want {
			t.Errorf("InterWordMinutes(%d, %d) = %d, want %d",
				c.activeHours, c.wordsPerDay, got, c.want)
		}
	}
}

func TestClamp(t *testing.T) {
	if got := clamp(5, 1, 8); got != 5 {
		t.Fatalf("clamp middle = %d, want 5", got)
	}
	if got := clamp(0, 1, 8); got != 1 {
		t.Fatalf("clamp below = %d, want 1", got)
	}
	if got := clamp(9, 1, 8); got != 8 {
		t.Fatalf("clamp above = %d, want 8", got)
	}
}

func TestActiveThreshold(t *testing.T) {
	if got := activeThreshold([24]float64{}); got != 0.5 {
		t.Fatalf("activeThreshold(empty) = %v, want 0.5", got)
	}
	scores := [24]float64{}
	for h := 0; h < 24; h++ {
		scores[h] = 0.5
	}
	if got, want := activeThreshold(scores), 0.2; math.Abs(got-want) > 1e-9 {
		t.Fatalf("activeThreshold(uniform 0.5) = %v, want %v", got, want)
	}
}
