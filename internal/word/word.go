package word

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"os"
	"sync"
	"time"
)

type Entry struct {
	Word        string   `json:"word"`
	Meaning     string   `json:"meaning"`
	Usage       string   `json:"usage"`
	Interval    int      `json:"interval"`
	NextReview  int64    `json:"next_review"`
	EaseFactor  float64  `json:"ease_factor"`
	Repetitions int      `json:"repetitions"`
	Tags        []string `json:"tags,omitempty"`
}

type Store struct {
	mu       sync.RWMutex
	entries  []*Entry
	filePath string
}

func NewStore(filePath string) *Store {
	return &Store{
		entries:  make([]*Entry, 0),
		filePath: filePath,
	}
}

func (s *Store) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read word file: %w", err)
	}

	if err := json.Unmarshal(data, &s.entries); err != nil {
		return fmt.Errorf("parse word file: %w", err)
	}
	return nil
}

func (s *Store) Save() error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, err := json.MarshalIndent(s.entries, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal words: %w", err)
	}

	if err := os.WriteFile(s.filePath, data, 0644); err != nil {
		return fmt.Errorf("write word file: %w", err)
	}
	return nil
}

func (s *Store) All() []*Entry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*Entry, len(s.entries))
	copy(result, s.entries)
	return result
}

func (s *Store) Due() []*Entry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	now := time.Now().Unix()
	var due []*Entry
	for _, e := range s.entries {
		if e.NextReview <= now {
			due = append(due, e)
		}
	}
	return due
}

func (s *Store) TodaysWord() *Entry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.entries) == 0 {
		return nil
	}

	now := time.Now()
	daySeed := now.YearDay()*10000 + now.Year()
	rng := rand.New(rand.NewSource(int64(daySeed)))

	idx := rng.Intn(len(s.entries))
	return s.entries[idx]
}

func (s *Store) UpdateInterval(e *Entry, quality int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if quality < 3 {
		e.Repetitions = 0
		e.Interval = 0
	} else {
		e.Repetitions++
		switch e.Repetitions {
		case 1:
			e.Interval = 1
		case 2:
			e.Interval = 6
		default:
			e.Interval = int(math.Round(float64(e.Interval) * e.EaseFactor))
		}
	}

	e.EaseFactor = e.EaseFactor + (0.1 - float64(5-quality)*(0.08+float64(5-quality)*0.02))
	if e.EaseFactor < 1.3 {
		e.EaseFactor = 1.3
	}

	e.NextReview = time.Now().AddDate(0, 0, e.Interval).Unix()
}

func (s *Store) Add(entries ...Entry) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range entries {
		entries[i].EaseFactor = 2.5
		entries[i].NextReview = time.Now().Unix()
		s.entries = append(s.entries, &entries[i])
	}
}
