package display

import (
	"testing"

	"github.com/msaeed/vocab/internal/word"
)

func TestWord(t *testing.T) {
	w := word.Word{
		Text:       "serendipity",
		Definition: "luck",
		Example:    "Pure serendipity",
	}
	Word(w)
}

func TestWordWithEmptyFields(t *testing.T) {
	w := word.Word{Text: "test"}
	Word(w)
}

func TestStats(t *testing.T) {
	Stats(10, 3)
}
