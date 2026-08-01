package lexicon

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func fixture(t *testing.T, schema, version string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "lexicon.sqlite")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	for _, statement := range []string{
		"CREATE TABLE metadata (key TEXT PRIMARY KEY, value TEXT NOT NULL)",
		"CREATE TABLE languages (id TEXT PRIMARY KEY)", "CREATE TABLE sources (id TEXT PRIMARY KEY)",
		"CREATE TABLE lexemes (id TEXT PRIMARY KEY, lemma TEXT, part_of_speech TEXT, source_id TEXT)",
		"CREATE TABLE forms (id TEXT PRIMARY KEY)",
		"CREATE TABLE senses (id TEXT PRIMARY KEY, lexeme_id TEXT, source_sense_key TEXT)",
		"CREATE TABLE definitions (id TEXT PRIMARY KEY, sense_id TEXT, text TEXT)",
		"CREATE TABLE examples (id TEXT PRIMARY KEY, sense_id TEXT, text TEXT)",
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.Exec("INSERT INTO metadata VALUES ('schema_version', ?), ('dataset_version', ?)", schema, version); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO lexemes VALUES ('l1', 'record', 'noun', 'oewn'), ('l2', 'plain', 'adjective', 'oewn')"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO senses VALUES ('s2', 'l1', 'sense-2'), ('s1', 'l1', 'sense-1'), ('s3', 'l2', 'sense-3')"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO definitions VALUES ('d2', 's2', 'second'), ('d1', 's1', 'first'), ('d3', 's3', 'without example')"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO examples VALUES ('e2', 's2', 'second example'), ('e1', 's1', 'first example')"); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestEntryIsDeterministicAndAllowsMissingExamples(t *testing.T) {
	db, err := Open(fixture(t, SupportedSchemaVersion, "0.4.0"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	entry, err := db.Entry("l1")
	if err != nil {
		t.Fatal(err)
	}
	if entry.Definition != "first" || entry.Example != "first example" || entry.SourceSenseKey != "sense-1" {
		t.Fatalf("unexpected entry: %+v", entry)
	}
	plain, err := db.Entry("l2")
	if err != nil {
		t.Fatal(err)
	}
	if plain.Example != "" {
		t.Fatalf("expected empty optional example, got %q", plain.Example)
	}
}

func TestOpenRejectsUnsupportedSchema(t *testing.T) {
	if _, err := Open(fixture(t, "0.3.0", "0.4.0")); err == nil {
		t.Fatal("expected unsupported schema error")
	}
}

func TestDatasetIsQueryOnly(t *testing.T) {
	db, err := Open(fixture(t, SupportedSchemaVersion, "0.4.0"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec("INSERT INTO metadata VALUES ('write', 'no')"); err == nil {
		t.Fatal("read-only dataset accepted a write")
	}
}

func TestDatasetVersionsAreSemverButNotSchemaVersions(t *testing.T) {
	if _, err := ParseDatasetVersion("0.4.1"); err != nil {
		t.Fatal(err)
	}
	if _, err := ParseDatasetVersion("0.4"); err == nil {
		t.Fatal("expected invalid dataset SemVer")
	}
	comparison, err := CompareDatasetVersions("0.4.1", "0.4.0")
	if err != nil || comparison <= 0 {
		t.Fatalf("comparison=%d err=%v", comparison, err)
	}
	db, err := Open(fixture(t, SupportedSchemaVersion, "0.4.1"))
	if err != nil {
		t.Fatal(err)
	}
	db.Close()
}

func TestVerifyBundleRejectsChecksumMismatch(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), []byte(`{"dataset_version":"0.4.0","schema_version":"0.2.0","artifacts":[{"filename":"lexicon.sqlite","sha256":"0000000000000000000000000000000000000000000000000000000000000000"}]}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "release-manifest.json"), []byte(`{"files":[]}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "lexicon.sqlite"), []byte("not sqlite"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyBundle(dir); err == nil {
		t.Fatal("expected checksum mismatch")
	}
}
