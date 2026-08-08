// Package engage adapts the learning cadence to when the user actually answers.
// It observes engagement per hour of the day, and derives the active window and
// the number of words to present per day.
package engage

import (
	"log"
	"math"
	"strconv"
	"time"

	"github.com/msaeedsaeedi/vocab/internal/database"
	"github.com/msaeedsaeedi/vocab/internal/state"
)

const (
	defaultActiveStart   = 8
	defaultActiveEnd     = 22
	defaultWordsPerDay   = 3
	defaultInterWordMins = 180
	minInterWordMins     = 45
	maxWordsPerDay       = 8
	minWordsPerDay       = 1
)

// Setting keys persisted in the settings table.
const (
	settingWordsPerDay = "words_per_day"
)

// Tracker records engagement observations and derives adaptive settings.
type Tracker struct {
	db    *database.DB
	store *state.Store
}

// New returns a Tracker that records into db and persists adaptive settings
// through store.
func New(db *database.DB, store *state.Store) *Tracker {
	return &Tracker{db: db, store: store}
}

// RecordEngagement counts one answered prompt in the current hour.
func (t *Tracker) RecordEngagement() {
	hour := time.Now().Hour()
	s, n := t.hourStats(hour)
	n++
	s = (s*float64(n-1) + 1.0) / float64(n)
	t.setHourStats(hour, s, n)
}

// RecordNotificationSent counts one notification sent in the current hour.
func (t *Tracker) RecordNotificationSent() {
	hour := time.Now().Hour()
	sent, answered := t.notificationStats(hour)
	sent++
	t.setNotificationStats(hour, sent, answered)
}

// RecordNotificationAnswered counts one notification answered in the current hour.
func (t *Tracker) RecordNotificationAnswered() {
	hour := time.Now().Hour()
	sent, answered := t.notificationStats(hour)
	answered++
	t.setNotificationStats(hour, sent, answered)
}

// PutLastWordTime records when the last word was presented.
func (t *Tracker) PutLastWordTime(tm time.Time) {
	if err := t.store.SetSetting("last_word_time", tm.Format(time.RFC3339)); err != nil {
		log.Printf("engage: PutLastWordTime: %v", err)
	}
}

// PutLastNotificationTime records when the last notification was sent.
func (t *Tracker) PutLastNotificationTime(tm time.Time) {
	if err := t.store.SetSetting("last_notify_time", tm.Format(time.RFC3339)); err != nil {
		log.Printf("engage: PutLastNotificationTime: %v", err)
	}
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

func (t *Tracker) notificationStats(hour int) (sent, answered int) {
	err := t.db.QueryRow(
		`SELECT notifications_sent, notifications_answered FROM engagement WHERE hour = ?`, hour,
	).Scan(&sent, &answered)
	if err != nil {
		return 0, 0
	}
	return sent, answered
}

func (t *Tracker) setNotificationStats(hour int, sent, answered int) {
	if _, err := t.db.Exec(
		`INSERT INTO engagement (hour, score, sample_count, notifications_sent, notifications_answered)
		 VALUES (?, 0, 0, ?, ?)
		 ON CONFLICT(hour) DO UPDATE SET notifications_sent = ?, notifications_answered = ?`,
		hour, sent, answered, sent, answered,
	); err != nil {
		log.Printf("engage: setNotificationStats: %v", err)
	}
}

// ActiveWindow returns the [start, end) hours of day when the user is most
// engaged, falling back to the defaults when there is not enough signal.
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

// WordsPerDay returns how many words to present each day, clamped to the
// supported range and smoothed against the previously persisted value.
func (t *Tracker) WordsPerDay() int {
	windowScore := t.windowEngagementScore()

	n := int(math.Round(float64(defaultWordsPerDay) * windowScore))
	n = clamp(n, minWordsPerDay, maxWordsPerDay)

	prev, ok := t.getSetting(settingWordsPerDay)
	if ok {
		n = clamp(n, prev-1, prev+1)
	}

	t.putSetting(settingWordsPerDay, n)
	return n
}

// InterWordMinutes returns how long to wait between word presentations within
// the active window.
func (t *Tracker) InterWordMinutes(activeHours int, wordsPerDay int) int {
	gap := (activeHours * 60) / (wordsPerDay + 1)
	return clamp(gap, minInterWordMins, defaultInterWordMins)
}

func (t *Tracker) hourlyScores() [24]float64 {
	var scores [24]float64

	rows, err := t.db.Query(`SELECT hour, notifications_sent, notifications_answered FROM engagement`)
	if err != nil {
		return scores
	}
	defer rows.Close()

	hasNotifData := false
	for rows.Next() {
		var h, sent, answered int
		if err := rows.Scan(&h, &sent, &answered); err != nil || h < 0 || h >= 24 {
			continue
		}
		total := sent + answered
		if total > 0 {
			hasNotifData = true
			scores[h] = float64(answered+2) / float64(sent+4)
		}
	}

	if hasNotifData {
		return scores
	}

	rows2, err := t.db.Query(`SELECT hour, score FROM engagement`)
	if err != nil {
		return scores
	}
	defer rows2.Close()

	for rows2.Next() {
		var h int
		var s float64
		if rows2.Scan(&h, &s) == nil && h >= 0 && h < 24 {
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

func (t *Tracker) getSetting(key string) (int, bool) {
	val, ok := t.store.GetSetting(key)
	if !ok || val == "" {
		return 0, false
	}
	v, err := strconv.Atoi(val)
	if err != nil || v == 0 {
		return 0, false
	}
	return v, true
}

func (t *Tracker) putSetting(key string, val int) {
	if err := t.store.SetSetting(key, strconv.Itoa(val)); err != nil {
		log.Printf("engage: putSetting %s: %v", key, err)
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
