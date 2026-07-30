package seed

import (
	"database/sql"
	"encoding/json"
	_ "embed"
	"fmt"
	"os"

	"github.com/msaeedsaeedi/vocab/internal/word"
)

//go:embed words.json
var wordsJSON []byte

type rawWord struct {
	Word     string `json:"word"`
	Meaning  string `json:"meaning"`
	Usage    string `json:"usage"`
	Pos      string `json:"pos,omitempty"`
}

func FromJSON(db *sql.DB, path string) error {
	return FromJSONReader(db, func() ([]byte, error) {
		return os.ReadFile(path)
	})
}

func FromJSONBytes(db *sql.DB, data []byte) error {
	return FromJSONReader(db, func() ([]byte, error) {
		return data, nil
	})
}

func FromJSONReader(db *sql.DB, readFn func() ([]byte, error)) error {
	data, err := readFn()
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
			Pos:        r.Pos,
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

func MustFromJSONBytes(db *sql.DB, data []byte) {
	if err := FromJSONBytes(db, data); err != nil {
		fmt.Fprintf(os.Stderr, "seed: %v\n", err)
		os.Exit(1)
	}
}

func MustFromEmbedded(db *sql.DB) {
	MustFromJSONBytes(db, wordsJSON)
}

func Needed(db *sql.DB) (bool, error) {
	n, err := word.Count(db)
	if err != nil {
		return false, err
	}
	return n == 0, nil
}
