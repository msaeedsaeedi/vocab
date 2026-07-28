package scheduler

import (
	"time"

	"github.com/msaeed/vocab/internal/word"
)

type Quality int

const (
	Forgotten Quality = 0
	Hard      Quality = 3
	Good      Quality = 4
	Easy      Quality = 5
)

type Scheduler struct {
	store *word.Store
}

func New(store *word.Store) *Scheduler {
	return &Scheduler{store: store}
}

func (s *Scheduler) DueCount() int {
	return len(s.store.Due())
}

func (s *Scheduler) Review(entry *word.Entry, quality Quality) {
	s.store.UpdateInterval(entry, int(quality))
}

func (s *Scheduler) ShowMeaningToday() bool {
	now := time.Now()
	return now.Day()%3 == 0
}

func (s *Scheduler) ShowUsageToday() bool {
	now := time.Now()
	return now.Day()%7 == 0
}
