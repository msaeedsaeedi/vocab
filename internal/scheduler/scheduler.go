package scheduler

import (
	"database/sql"
	"math"
	"time"

	"github.com/msaeedsaeedi/vocab/internal/word"
)

const hoursPerDay = 24.0

func ScheduleReview(db *sql.DB, w *word.Word, rating int) (float64, error) {
	elapsed := elapsedHours(w.LastReviewed)
	logEntry := &word.ReviewLog{
		WordID:       w.ID,
		Rating:       rating,
		ElapsedHours: elapsed,
		Stability:    w.Stability,
	}
	if err := word.InsertReviewLog(db, logEntry); err != nil {
		return 0, err
	}

	state := memoryState{
		stability:  w.Stability,
		difficulty: w.Difficulty,
		alpha:      w.BktAlpha,
		beta:       w.BktBeta,
	}

	state.update(rating, elapsed/hoursPerDay)

	nextDueDays := state.nextInterval(rating)
	nextDue := time.Now().Add(time.Duration(nextDueDays*24) * time.Hour).Format("2006-01-02")

	lapseCount := w.LapseCount
	if rating < 2 {
		lapseCount++
	}

	if err := word.UpdateAdaptive(db, w.ID,
		state.stability, state.difficulty, state.alpha, state.beta,
		w.ReviewCount+1, lapseCount,
		time.Now().Format("2006-01-02 15:04:05"),
		nextDue, "",
	); err != nil {
		return 0, err
	}

	return nextDueDays, nil
}

type memoryState struct {
	stability  float64
	difficulty float64
	alpha      float64
	beta       float64
}

func (m *memoryState) update(rating int, elapsedDays float64) {
	if rating >= 2 {
		m.alpha += 1.0
	} else {
		m.beta += 1.0
	}

	switch rating {
	case 0:
		m.difficulty = math.Min(1.0, m.difficulty+0.20)
	case 1:
		m.difficulty = math.Min(1.0, m.difficulty+0.10)
	case 2:
	case 3:
		m.difficulty = math.Max(0.0, m.difficulty-0.15)
	}

	if rating == 0 {
		m.stability = 0.5
	} else {
		factors := []float64{0, 1.3, 2.0, 2.8}
		factor := factors[rating]
		m.stability = math.Max(0.5, m.stability*factor*(1.0-m.difficulty*0.5))
	}
}

func (m *memoryState) nextInterval(rating int) float64 {
	if rating == 0 {
		return 0.25
	}
	factors := []float64{0, 0.25, 1.0, 1.3}
	return math.Max(0.25, m.stability*factors[rating])
}

func RecallProbability(stability float64, elapsedHours float64) float64 {
	if stability <= 0 || elapsedHours <= 0 {
		return 1.0
	}
	return math.Exp(-elapsedHours / (stability * hoursPerDay))
}

func BktSkill(alpha, beta float64) float64 {
	if alpha+beta == 0 {
		return 0.5
	}
	return alpha / (alpha + beta)
}

func Priority(w *word.Word, overdueHours float64) float64 {
	recall := RecallProbability(w.Stability, overdueHours)
	skill := BktSkill(w.BktAlpha, w.BktBeta)
	overdueFactor := math.Min(overdueHours/(72.0), 1.0)

	difficultyWeight := 0.3 + w.Difficulty*0.2

	return (1.0-recall)*0.35 + (1.0-skill)*0.25 + overdueFactor*0.15 + difficultyWeight*0.25
}

func SelectNextWord(db *sql.DB, dueWords []word.Word) *word.Word {
	if len(dueWords) == 0 {
		return nil
	}
	if len(dueWords) == 1 {
		return &dueWords[0]
	}

	now := time.Now()
	bestIdx := 0
	bestScore := -1.0

	for i, w := range dueWords {
		overdue := now.Sub(parseDueDate(w.NextDue)).Hours()
		if overdue < 0 {
			overdue = 0
		}
		score := Priority(&dueWords[i], overdue)
		if score > bestScore {
			bestScore = score
			bestIdx = i
		}
	}

	return &dueWords[bestIdx]
}

func parseDueDate(nextDue string) time.Time {
	t, err := time.Parse("2006-01-02", nextDue)
	if err != nil {
		return time.Now()
	}
	return t
}

func elapsedHours(lastReviewed string) float64 {
	if lastReviewed == "" {
		return 0
	}
	t, err := time.Parse("2006-01-02 15:04:05", lastReviewed)
	if err != nil {
		t, err = time.Parse("2006-01-02", lastReviewed)
		if err != nil {
			return 0
		}
	}
	return time.Since(t).Hours()
}
