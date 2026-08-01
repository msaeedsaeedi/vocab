package dataset

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/msaeedsaeedi/vocab/internal/database"
)

func TestInstallBundledActivatesWithoutWritingVocabDBOver(t *testing.T) {
	if os.Getenv("VOCAB_LEXICON_INTEGRATION") != "1" {
		t.Skip("set VOCAB_LEXICON_INTEGRATION=1 to run against the published Lexicon bundle")
	}
	// The checked-in Lexicon release is the integration fixture for the v0.3 contract.
	source := "/home/msaeed/Projects/lexicon/dist/lexicon-en-oewn-0.4.0"
	if _, err := os.Stat(source); err != nil {
		t.Skip("local Lexicon integration fixture unavailable")
	}
	db, err := database.Open(filepath.Join(t.TempDir(), "vocab.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	root := filepath.Join(t.TempDir(), "datasets")
	installed, err := InstallBundled(db, filepath.Dir(source), root)
	if err != nil {
		t.Fatal(err)
	}
	defer installed.Close()
	if installed.DatasetVersion != "0.4.0" {
		t.Fatalf("version=%q", installed.DatasetVersion)
	}
	active, err := Active(db)
	if err != nil {
		t.Fatal(err)
	}
	defer active.Close()
	if active.Path != installed.Path {
		t.Fatalf("active=%q installed=%q", active.Path, installed.Path)
	}
	var learnerTables int
	if err := db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='learning_items'").Scan(&learnerTables); err != nil {
		t.Fatal(err)
	}
	if learnerTables != 1 {
		t.Fatal("vocab learner database was not preserved")
	}
}
