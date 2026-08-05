package words

import (
	"bufio"
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"slices"
)

//go:embed seed.jsonl
var seedBytes []byte

// Entry is a single curated word with presentation fields.
type Entry struct {
	LexemeID   string `json:"lexeme_id"`
	Text       string `json:"text"`
	Definition string `json:"definition"`
	Example    string `json:"example"`
	Pos        string `json:"pos"`
}

// List holds the in-memory curated word catalog.
type List struct {
	entries map[string]Entry
	ids     []string
}

// Load reads the embedded seed JSONL and returns an immutable List.
func Load(r io.Reader) (*List, error) {
	l := &List{entries: map[string]Entry{}}
	s := bufio.NewScanner(r)
	for s.Scan() {
		line := s.Text()
		if line == "" {
			continue
		}
		var e Entry
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			return nil, fmt.Errorf("parse seed entry: %w", err)
		}
		if e.LexemeID == "" {
			return nil, fmt.Errorf("seed entry missing lexeme_id")
		}
		if _, exists := l.entries[e.LexemeID]; exists {
			return nil, fmt.Errorf("duplicate lexeme_id %q in seed", e.LexemeID)
		}
		l.entries[e.LexemeID] = e
		l.ids = append(l.ids, e.LexemeID)
	}
	if err := s.Err(); err != nil {
		return nil, fmt.Errorf("read seed: %w", err)
	}
	if len(l.entries) == 0 {
		return nil, fmt.Errorf("seed list is empty")
	}
	return l, nil
}

// Lookup returns the entry for lexemeID and true, or a zero Entry and false.
func (l *List) Lookup(lexemeID string) (Entry, bool) {
	e, ok := l.entries[lexemeID]
	return e, ok
}

// IDs returns the lexeme IDs in stable file order.
func (l *List) IDs() []string {
	return slices.Clone(l.ids)
}

// Len returns the number of entries.
func (l *List) Len() int { return len(l.entries) }

// LoadSeed returns the built-in curated word list.
func LoadSeed() *List {
	l, err := Load(bytes.NewReader(seedBytes))
	if err != nil {
		panic(fmt.Sprintf("internal/words: embedded seed is corrupt: %v", err))
	}
	return l
}