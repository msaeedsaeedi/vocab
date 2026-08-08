package daemon

import (
	"fmt"
	"sort"
	"time"

	"github.com/msaeedsaeedi/vocab/internal/word"
)

// introduceItems inserts up to count unlearned lexicon entries into the
// learner's queue, ordered by word frequency.
func (d *Daemon) introduceItems(count int) error {
	existing, err := word.GetAll(d.db)
	if err != nil {
		return err
	}
	known := make(map[string]bool, len(existing))
	for _, item := range existing {
		known[item.LexemeID] = true
	}

	type candidate struct {
		id   string
		rank int
	}
	candidates := make([]candidate, 0, len(d.list.IDs()))
	for _, id := range d.list.IDs() {
		if known[id] {
			continue
		}
		entry, ok := d.list.Lookup(id)
		if !ok {
			continue
		}
		rank := entry.FrequencyRank
		if rank == 0 {
			rank = 10000
		}
		candidates = append(candidates, candidate{id: id, rank: rank})
	}
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].rank < candidates[j].rank
	})

	introduced := 0
	for _, c := range candidates {
		entry, ok := d.list.Lookup(c.id)
		if !ok {
			continue
		}
		item := word.Word{LexemeID: entry.LexemeID, NextDue: "1970-01-01 00:00:00"}
		if err := word.Insert(d.db, &item); err != nil {
			return err
		}
		introduced++
		if introduced == count {
			break
		}
	}
	if introduced == 0 {
		return fmt.Errorf("no unlearned words remain in seed")
	}
	return nil
}

// findNextDue returns the earliest next_due across all learning items.
func (d *Daemon) findNextDue() time.Time {
	nextDue, err := word.GetNextDue(d.db)
	if err != nil || nextDue == "" {
		return time.Time{}
	}
	t, err := time.Parse(word.TimeLayout, nextDue)
	if err != nil {
		t, err = time.Parse("2006-01-02", nextDue)
		if err != nil {
			return time.Time{}
		}
	}
	return t
}

// refreshDueWords drops the just-presented word from the pending slice.
func refreshDueWords(current []word.Word, excludeID int64) []word.Word {
	filtered := make([]word.Word, 0, len(current)-1)
	for _, w := range current {
		if w.ID != excludeID {
			filtered = append(filtered, w)
		}
	}
	return filtered
}
