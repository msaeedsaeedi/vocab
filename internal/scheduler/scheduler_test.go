package scheduler

import (
	"testing"

	"github.com/msaeed/vocab/internal/word"
)

func TestReviewGood(t *testing.T) {
	s := word.NewStore("")
	s.Add(word.Entry{Word: "ephemeral", Meaning: "short-lived"})
	all := s.All()

	sch := New(s)
	sch.Review(all[0], Good)

	if all[0].Repetitions == 0 {
		t.Fatal("expected repetitions to increase")
	}
}

func TestReviewForgotten(t *testing.T) {
	s := word.NewStore("")
	s.Add(word.Entry{Word: "ephemeral", Meaning: "short-lived"})
	all := s.All()

	sch := New(s)
	sch.Review(all[0], Forgotten)

	if all[0].Repetitions != 0 {
		t.Fatal("expected repetitions to reset")
	}
}

func TestDueCount(t *testing.T) {
	s := word.NewStore("")
	s.Add(word.Entry{Word: "test"})
	sch := New(s)

	if sch.DueCount() != 1 {
		t.Fatalf("expected 1 due, got %d", sch.DueCount())
	}
}
