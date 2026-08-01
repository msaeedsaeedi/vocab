package engage

import (
	"log"
	"math"
	"time"

	"github.com/msaeedsaeedi/vocab/internal/database"
)

const (
	defaultActiveStart   = 8
	defaultActiveEnd     = 22
	defaultWordsPerDay   = 3
	defaultInterWordMins = 180
	minInterWordMins     = 45
	maxWordsPerDay       = 8
	minWordsPerDay       = 1
	responseWindowHours  = 4
)

type Tracker struct {
	db *database.DB
}

func New(db *database.DB) *Tracker {
	return &Tracker{db: db}
}

func (t *Tracker) RecordEngagement() {
	hour := time.Now().Hour()
	s, n := t.hourStats(hour)
	n++
	s = (s*float64(n-1) + 1.0) / float64(n)
	t.setHourStats(hour, s, n)
}

func (t *Tracker) hourStats(hour int) (float64, int) {
	var score float64
	var count int
	err := t.db.QueryRow(
		`SELECT score, sample_count FROM engagement WHERE hour = ?`, hour,
	).Scan(&score, &count)
	if err != nil {
		return 0, 0
	}
	return score, count
}

func (t *Tracker) setHourStats(hour int, score float64, count int) {
	if _, err := t.db.Exec(
		`INSERT INTO engagement (hour, score, sample_count) VALUES (?, ?, ?)
		 ON CONFLICT(hour) DO UPDATE SET score = ?, sample_count = ?`,
		hour, score, count, score, count,
	); err != nil {
		log.Printf("engage: setHourStats: %v", err)
	}
}

func (t *Tracker) ActiveWindow() (start, end int) {
	scores := t.hourlyScores()

	start, end = defaultActiveStart, defaultActiveEnd

	threshold := activeThreshold(scores)

	bestStart, bestEnd, bestSpan := 0, 0, 0
	for s := 0; s < 24; s++ {
		for e := s + 6; e <= 24; e++ {
			sum := 0.0
			for h := s; h < e; h++ {
				sum += scores[h]
			}
			span := e - s
			if sum > threshold*float64(span) && span > bestSpan {
				bestStart, bestEnd, bestSpan = s, e, span
			}
		}
	}

	if bestSpan >= 8 {
		start, end = bestStart, bestEnd
	}

	if start < 0 {
		start = 0
	}
	if end > 24 {
		end = 24
	}
	return start, end
}

func (t *Tracker) WordsPerDay() int {
	windowScore := t.windowEngagementScore()

	n := int(math.Round(float64(defaultWordsPerDay) * windowScore))
	n = clamp(n, minWordsPerDay, maxWordsPerDay)

	daemonVal := t.getDaemonState("words_per_day")
	if daemonVal != nil {
		n = clamp(n, *daemonVal-1, *daemonVal+1)
	}

	t.putDaemonState("words_per_day", n)
	return n
}

func (t *Tracker) InterWordMinutes(activeHours int, wordsPerDay int) int {
	gap := (activeHours * 60) / (wordsPerDay + 1)
	return clamp(gap, minInterWordMins, defaultInterWordMins)
}

func (t *Tracker) BestNotificationHour(wordPhase string) int {
	scores := t.hourlyScores()
	now := time.Now().Hour()

	var offset int
	switch wordPhase {
	case "recall":
		offset = 2
	case "produce":
		offset = 5
	default:
		offset = 0
	}

	target := now + offset
	if target >= 24 {
		target -= 24
	}

	best, bestScore := target, scores[target]
	for h := max(0, target-2); h <= min(23, target+2); h++ {
		if scores[h] > bestScore {
			best, bestScore = h, scores[h]
		}
	}

	return best
}

func (t *Tracker) hourlyScores() [24]float64 {
	var scores [24]float64

	rows, err := t.db.Query(`SELECT hour, score FROM engagement`)
	if err != nil {
		return scores
	}
	defer rows.Close()

	for rows.Next() {
		var h int
		var s float64
		if rows.Scan(&h, &s) == nil && h >= 0 && h < 24 {
			scores[h] = s
		}
	}

	if allZero(scores) {
		for i := defaultActiveStart; i <= defaultActiveEnd; i++ {
			scores[i] = 0.5
		}
	}

	return scores
}

func (t *Tracker) windowEngagementScore() float64 {
	s, e := t.ActiveWindow()
	if e <= s {
		return 1.0
	}
	scores := t.hourlyScores()
	sum := 0.0
	for h := s; h < e; h++ {
		sum += scores[h]
	}
	avg := sum / float64(e-s)
	if avg < 0.1 {
		return 1.0
	}
	return math.Min(avg*2.0, 1.5)
}

func (t *Tracker) getDaemonState(key string) *int {
	var val string
	err := t.db.QueryRow(`SELECT value FROM settings WHERE key = ?`, key).Scan(&val)
	if err != nil || val == "" {
		return nil
	}
	v := 0
	for _, c := range val {
		if c >= '0' && c <= '9' {
			v = v*10 + int(c-'0')
		}
	}
	if v == 0 {
		return nil
	}
	return &v
}

func (t *Tracker) putDaemonState(key string, val int) {
	valStr := intToStr(val)
	if _, err := t.db.Exec(
		`INSERT INTO settings (key, value) VALUES (?, ?)
		 ON CONFLICT(key) DO UPDATE SET value = ?`,
		key, valStr, valStr,
	); err != nil {
		log.Printf("engage: putDaemonState %s: %v", key, err)
	}
}

func (t *Tracker) PutLastWordTime(tm time.Time) {
	val := tm.Format(time.RFC3339)
	if _, err := t.db.Exec(
		`INSERT INTO settings (key, value) VALUES ('last_word_time', ?)
		 ON CONFLICT(key) DO UPDATE SET value = ?`, val, val,
	); err != nil {
		log.Printf("engage: PutLastWordTime: %v", err)
	}
}

func (t *Tracker) GetLastWordTime() (time.Time, bool) {
	var val string
	err := t.db.QueryRow(`SELECT value FROM settings WHERE key = 'last_word_time'`).Scan(&val)
	if err != nil || val == "" {
		return time.Time{}, false
	}
	tm, err := time.Parse(time.RFC3339, val)
	if err != nil {
		return time.Time{}, false
	}
	return tm, true
}

func (t *Tracker) PutLastNotificationTime(tm time.Time) {
	val := tm.Format(time.RFC3339)
	if _, err := t.db.Exec(
		`INSERT INTO settings (key, value) VALUES ('last_notify_time', ?)
		 ON CONFLICT(key) DO UPDATE SET value = ?`, val, val,
	); err != nil {
		log.Printf("engage: PutLastNotificationTime: %v", err)
	}
}

func (t *Tracker) GetLastNotificationTime() (time.Time, bool) {
	var val string
	err := t.db.QueryRow(`SELECT value FROM settings WHERE key = 'last_notify_time'`).Scan(&val)
	if err != nil || val == "" {
		return time.Time{}, false
	}
	tm, err := time.Parse(time.RFC3339, val)
	if err != nil {
		return time.Time{}, false
	}
	return tm, true
}

func (t *Tracker) PutCurrentWord(id int64, phase string) {
	if _, err := t.db.Exec(
		`INSERT INTO daemon_state (key, value) VALUES ('current_word_id', ?)
		 ON CONFLICT(key) DO UPDATE SET value = ?`,
		intToStr(int(id)), intToStr(int(id)),
	); err != nil {
		log.Printf("engage: PutCurrentWord id: %v", err)
	}
	if _, err := t.db.Exec(
		`INSERT INTO daemon_state (key, value) VALUES ('current_phase', ?)
		 ON CONFLICT(key) DO UPDATE SET value = ?`,
		phase, phase,
	); err != nil {
		log.Printf("engage: PutCurrentWord phase: %v", err)
	}
}

func (t *Tracker) GetCurrentWord() (int64, string) {
	var idStr, phase string
	err1 := t.db.QueryRow(`SELECT value FROM daemon_state WHERE key = 'current_word_id'`).Scan(&idStr)
	err2 := t.db.QueryRow(`SELECT value FROM daemon_state WHERE key = 'current_phase'`).Scan(&phase)
	if err1 != nil || err2 != nil {
		return 0, ""
	}
	id := 0
	for _, c := range idStr {
		if c >= '0' && c <= '9' {
			id = id*10 + int(c-'0')
		}
	}
	return int64(id), phase
}

func (t *Tracker) ClearCurrentWord() {
	if _, err := t.db.Exec(`DELETE FROM daemon_state WHERE key IN ('current_word_id', 'current_phase')`); err != nil {
		log.Printf("engage: ClearCurrentWord: %v", err)
	}
}

func activeThreshold(scores [24]float64) float64 {
	sum := 0.0
	count := 0
	for _, s := range scores {
		if s > 0 {
			sum += s
			count++
		}
	}
	if count == 0 {
		return 0.5
	}
	return (sum / float64(count)) * 0.4
}

func allZero(scores [24]float64) bool {
	for _, s := range scores {
		if s > 0 {
			return false
		}
	}
	return true
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func intToStr(v int) string {
	if v == 0 {
		return "0"
	}
	neg := false
	if v < 0 {
		neg = true
		v = -v
	}
	buf := make([]byte, 0, 12)
	for v > 0 {
		buf = append([]byte{byte('0' + v%10)}, buf...)
		v /= 10
	}
	if neg {
		buf = append([]byte{'-'}, buf...)
	}
	return string(buf)
}
