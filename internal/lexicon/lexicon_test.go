package lexicon

import (
	"slices"
	"strings"
	"testing"
)

func TestLoad(t *testing.T) {
	in := strings.NewReader(`{"lexeme_id":"a","text":"apple","definition":"a fruit","example":"I ate an apple.","pos":"noun"}
{"lexeme_id":"b","text":"ball","definition":"a round object","example":"He threw the ball.","pos":"noun"}`)
	l, err := Load(in)
	if err != nil {
		t.Fatal(err)
	}
	if l.Len() != 2 {
		t.Fatalf("len=%d want 2", l.Len())
	}

	e, ok := l.Lookup("a")
	if !ok || e.Text != "apple" {
		t.Fatalf("lookup a: %+v", e)
	}
	e, ok = l.Lookup("b")
	if !ok || e.Text != "ball" {
		t.Fatalf("lookup b: %+v", e)
	}
	_, ok = l.Lookup("nonexistent")
	if ok {
		t.Fatal("lookup nonexistent should return false")
	}

	ids := l.IDs()
	if !slices.Equal(ids, []string{"a", "b"}) {
		t.Fatalf("ids=%v", ids)
	}
}

func TestLoadDuplicate(t *testing.T) {
	in := strings.NewReader(`{"lexeme_id":"a","text":"apple","definition":"a fruit","example":"I ate an apple.","pos":"noun"}
{"lexeme_id":"a","text":"ant","definition":"an insect","example":"Ants are small.","pos":"noun"}`)
	_, err := Load(in)
	if err == nil {
		t.Fatal("expected error for duplicate lexeme_id")
	}
}

func TestLoadEmpty(t *testing.T) {
	_, err := Load(strings.NewReader(""))
	if err == nil {
		t.Fatal("expected error for empty input")
	}
}

func TestLoadMissingLexemeID(t *testing.T) {
	in := strings.NewReader(`{"text":"apple","definition":"a fruit"}`)
	_, err := Load(in)
	if err == nil {
		t.Fatal("expected error for missing lexeme_id")
	}
}

func TestLoadInvalidJSON(t *testing.T) {
	in := strings.NewReader(`not json`)
	_, err := Load(in)
	if err == nil {
		t.Fatal("expected parse error")
	}
}
