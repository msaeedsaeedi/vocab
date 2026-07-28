package seed

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"

	"github.com/msaeed/vocab/internal/word"
)

type rawWord struct {
	Word    string `json:"word"`
	Meaning string `json:"meaning"`
	Usage   string `json:"usage"`
}

func FromJSON(db *sql.DB, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read seed file: %w", err)
	}

	var raw []rawWord
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("parse seed file: %w", err)
	}

	for _, r := range raw {
		w := word.Word{
			Text:       r.Word,
			Definition: r.Meaning,
			Example:    r.Usage,
			Box:        0,
			NextDue:    "1970-01-01",
		}
		if err := word.Insert(db, &w); err != nil {
			return fmt.Errorf("seed word %q: %w", r.Word, err)
		}
	}
	return nil
}

func MustFromJSON(db *sql.DB, path string) {
	if err := FromJSON(db, path); err != nil {
		fmt.Fprintf(os.Stderr, "seed: %v\n", err)
		os.Exit(1)
	}
}

func Needed(db *sql.DB) (bool, error) {
	n, err := word.Count(db)
	if err != nil {
		return false, err
	}
	return n == 0, nil
}
