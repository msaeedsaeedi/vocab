package word

import (
	"os"
	"testing"
)

func TestStoreAddAndSave(t *testing.T) {
	s := NewStore("/tmp/test_vocab_words.json")
	defer os.Remove("/tmp/test_vocab_words.json")

	s.Add(Entry{Word: "serendipity", Meaning: "luck in unexpected discoveries"})
	if err := s.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	s2 := NewStore("/tmp/test_vocab_words.json")
	if err := s2.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}

	all := s2.All()
	if len(all) != 1 || all[0].Word != "serendipity" {
		t.Fatalf("got %+v", all)
	}
}

func TestTodaysWordDeterministic(t *testing.T) {
	s := NewStore("/tmp/test_vocab_today.json")
	defer os.Remove("/tmp/test_vocab_today.json")

	s.Add(
		Entry{Word: "alpha"},
		Entry{Word: "beta"},
		Entry{Word: "gamma"},
	)

	w1 := s.TodaysWord()
	w2 := s.TodaysWord()
	if w1.Word != w2.Word {
		t.Fatal("todays word should be deterministic per day")
	}
}

func TestUpdateIntervalNew(t *testing.T) {
	e := &Entry{Word: "test", EaseFactor: 2.5}
	s := NewStore("")
	s.UpdateInterval(e, 5)

	if e.Repetitions != 1 || e.Interval != 1 {
		t.Fatalf("expected reps=1 interval=1, got reps=%d interval=%d", e.Repetitions, e.Interval)
	}
}

func TestUpdateIntervalResetOnBad(t *testing.T) {
	e := &Entry{Word: "test", EaseFactor: 2.5, Repetitions: 5, Interval: 30}
	s := NewStore("")
	s.UpdateInterval(e, 0)

	if e.Repetitions != 0 || e.Interval != 0 {
		t.Fatalf("expected reps=0 interval=0, got reps=%d interval=%d", e.Repetitions, e.Interval)
	}
}
