// Package lexicon reads a verified Lexicon SQLite dataset without ever
// modifying its canonical content.
package lexicon

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"golang.org/x/mod/semver"
	_ "modernc.org/sqlite"
)

// SupportedSchemaVersion is the Lexicon data-shape contract consumed by
// Vocab v0.3. Dataset releases evolve independently under SemVer.
const SupportedSchemaVersion = "0.2.0"

var requiredTables = []string{
	"metadata", "languages", "sources", "lexemes", "forms", "senses", "definitions", "examples",
}

// Dataset is an open read-only Lexicon SQLite artifact.
type Dataset struct {
	*sql.DB
	Path           string
	DatasetVersion string
	SchemaVersion  string
}

// Entry is the presentation-ready projection of one canonical lexeme. IDs and
// source provenance remain available to Vocab learner state and audit views.
type Entry struct {
	LexemeID       string
	Lemma          string
	PartOfSpeech   string
	SourceID       string
	SenseID        string
	SourceSenseKey string
	Definition     string
	Example        string
}

// Open validates and opens path in SQLite read-only/query-only mode.
func Open(path string) (*Dataset, error) {
	log.Printf("lexicon.Open: opening %q", path)

	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve lexicon path: %w", err)
	}

	p := filepath.ToSlash(abs)

	// SQLite URI filenames represent an absolute Windows drive path as
	// /C:/path/to/database.
	if runtime.GOOS == "windows" &&
		len(p) >= 2 &&
		p[1] == ':' {
		p = "/" + p
	}

	u := &url.URL{
		Scheme: "file",
		Path:   p,
	}

	q := u.Query()
	q.Set("mode", "ro")
	q.Set("immutable", "1")
	u.RawQuery = q.Encode()

	dsn := u.String()

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open lexicon: %w", err)
	}

	db.SetMaxOpenConns(1)

	closeOnError := func(err error) (*Dataset, error) {
		log.Printf("lexicon.Open: closing connection after error: %v", err)
		_ = db.Close()
		return nil, err
	}

	if err := db.Ping(); err != nil {
		return closeOnError(fmt.Errorf("ping lexicon: %w", err))
	}

	if _, err := db.Exec(
		"PRAGMA foreign_keys = ON; PRAGMA query_only = ON",
	); err != nil {
		return closeOnError(
			fmt.Errorf("configure read-only lexicon: %w", err),
		)
	}

	if err := validateSurface(db); err != nil {
		return closeOnError(err)
	}

	metadata, err := readMetadata(db)
	if err != nil {
		return closeOnError(err)
	}

	if metadata["schema_version"] != SupportedSchemaVersion {
		err := fmt.Errorf(
			"unsupported Lexicon schema %q (want %q)",
			metadata["schema_version"],
			SupportedSchemaVersion,
		)
		return closeOnError(err)
	}

	if _, err := ParseDatasetVersion(metadata["dataset_version"]); err != nil {
		return closeOnError(err)
	}

	log.Printf("lexicon.Open: opened %q", path)

	return &Dataset{
		DB:             db,
		Path:           abs,
		DatasetVersion: metadata["dataset_version"],
		SchemaVersion:  metadata["schema_version"],
	}, nil
}

// ParseDatasetVersion validates the manifest's unprefixed SemVer version and
// returns its canonical form with a leading v for semver.Compare.
func ParseDatasetVersion(version string) (string, error) {
	bare := strings.TrimPrefix(version, "v")
	core := strings.SplitN(strings.SplitN(bare, "+", 2)[0], "-", 2)[0]
	if len(strings.Split(core, ".")) != 3 {
		return "", fmt.Errorf("invalid Lexicon dataset version %q", version)
	}
	canonical := version
	if !strings.HasPrefix(canonical, "v") {
		canonical = "v" + canonical
	}
	if !semver.IsValid(canonical) {
		return "", fmt.Errorf("invalid Lexicon dataset version %q", version)
	}
	return canonical, nil
}

// CompareDatasetVersions compares manifest dataset versions after validation.
func CompareDatasetVersions(a, b string) (int, error) {
	left, err := ParseDatasetVersion(a)
	if err != nil {
		return 0, err
	}
	right, err := ParseDatasetVersion(b)
	if err != nil {
		return 0, err
	}
	return semver.Compare(left, right), nil
}

// Entry returns the deterministic first sense, definition, and optional
// example for lexemeID. Ordering by canonical IDs keeps a release reproducible.
func (d *Dataset) Entry(lexemeID string) (*Entry, error) {
	const query = `
SELECT l.id, l.lemma, l.part_of_speech, l.source_id,
       s.id, COALESCE(s.source_sense_key, ''), def.text,
       COALESCE(ex.text, '')
FROM lexemes l
JOIN senses s ON s.id = (
  SELECT id FROM senses WHERE lexeme_id = l.id ORDER BY id LIMIT 1
)
JOIN definitions def ON def.id = (
  SELECT id FROM definitions WHERE sense_id = s.id ORDER BY id LIMIT 1
)
LEFT JOIN examples ex ON ex.id = (
  SELECT id FROM examples WHERE sense_id = s.id ORDER BY id LIMIT 1
)
WHERE l.id = ?`
	entry := &Entry{}
	err := d.QueryRow(query, lexemeID).Scan(
		&entry.LexemeID, &entry.Lemma, &entry.PartOfSpeech, &entry.SourceID,
		&entry.SenseID, &entry.SourceSenseKey, &entry.Definition, &entry.Example,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("lexeme %q not found or not displayable", lexemeID)
	}
	if err != nil {
		return nil, fmt.Errorf("query lexeme %q: %w", lexemeID, err)
	}
	return entry, nil
}

// DisplayableLexemeIDs returns canonical IDs in stable order. The caller owns
// learner selection and can exclude IDs already represented by learning items.
func (d *Dataset) DisplayableLexemeIDs(limit int) ([]string, error) {
	rows, err := d.Query(`SELECT l.id FROM lexemes l WHERE EXISTS (SELECT 1 FROM senses s JOIN definitions def ON def.sense_id = s.id WHERE s.lexeme_id = l.id) ORDER BY l.id LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("list displayable lexemes: %w", err)
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func validateSurface(db *sql.DB) error {
	var integrity string
	if err := db.QueryRow("PRAGMA integrity_check").Scan(&integrity); err != nil {
		return fmt.Errorf("check Lexicon integrity: %w", err)
	}
	if integrity != "ok" {
		return fmt.Errorf("Lexicon integrity check failed: %s", integrity)
	}
	rows, err := db.Query("SELECT name FROM sqlite_master WHERE type = 'table'")
	if err != nil {
		return fmt.Errorf("list Lexicon tables: %w", err)
	}
	defer rows.Close()
	tables := map[string]bool{}
	for rows.Next() {
		var table string
		if err := rows.Scan(&table); err != nil {
			return err
		}
		tables[table] = true
	}
	for _, table := range requiredTables {
		if !tables[table] {
			return fmt.Errorf("Lexicon missing required table %q", table)
		}
	}
	return rows.Err()
}

func readMetadata(db *sql.DB) (map[string]string, error) {
	rows, err := db.Query("SELECT key, value FROM metadata WHERE key IN ('schema_version', 'dataset_version')")
	if err != nil {
		return nil, fmt.Errorf("read Lexicon metadata: %w", err)
	}
	defer rows.Close()
	values := map[string]string{}
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return nil, err
		}
		values[key] = value
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if values["schema_version"] == "" || values["dataset_version"] == "" {
		return nil, fmt.Errorf("Lexicon metadata is missing schema_version or dataset_version")
	}
	return values, nil
}

type manifest struct {
	DatasetVersion string         `json:"dataset_version"`
	SchemaVersion  string         `json:"schema_version"`
	Artifacts      []checksumFile `json:"artifacts"`
}
type releaseManifest struct {
	Files []checksumFile `json:"files"`
}
type checksumFile struct {
	Filename string `json:"filename"`
	SHA256   string `json:"sha256"`
}

// VerifyBundle checks release and artifact manifests before the SQLite file is
// installed. It deliberately does not inspect vocab-compat JSON projections.
func VerifyBundle(dir string) (string, error) {
	readJSON := func(name string, target any) error {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return err
		}
		return json.Unmarshal(data, target)
	}
	var release releaseManifest
	if err := readJSON("release-manifest.json", &release); err != nil {
		return "", fmt.Errorf("read release manifest: %w", err)
	}
	var canonical manifest
	if err := readJSON("manifest.json", &canonical); err != nil {
		return "", fmt.Errorf("read manifest: %w", err)
	}
	if canonical.SchemaVersion != SupportedSchemaVersion {
		return "", fmt.Errorf("unsupported Lexicon bundle schema=%q dataset=%q", canonical.SchemaVersion, canonical.DatasetVersion)
	}
	if _, err := ParseDatasetVersion(canonical.DatasetVersion); err != nil {
		return "", err
	}
	files := append(append([]checksumFile{}, release.Files...), canonical.Artifacts...)
	sort.Slice(files, func(i, j int) bool { return files[i].Filename < files[j].Filename })
	seen := map[string]string{}
	for _, file := range files {
		if prior, ok := seen[file.Filename]; ok && prior != file.SHA256 {
			return "", fmt.Errorf("conflicting checksum for %q", file.Filename)
		}
		seen[file.Filename] = file.SHA256
	}
	if _, ok := seen["manifest.json"]; !ok {
		return "", fmt.Errorf("release manifest does not checksum manifest.json")
	}
	var sqliteName string
	for name, digest := range seen {
		if err := verifySHA256(filepath.Join(dir, name), digest); err != nil {
			return "", err
		}
		if filepath.Ext(name) == ".sqlite" {
			sqliteName = name
		}
	}
	if sqliteName == "" {
		return "", fmt.Errorf("bundle does not contain a SQLite artifact")
	}
	return filepath.Join(dir, sqliteName), nil
}

func verifySHA256(path, want string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", filepath.Base(path), err)
	}
	got := sha256.Sum256(data)
	if hex.EncodeToString(got[:]) != want {
		return fmt.Errorf("checksum mismatch for %s", filepath.Base(path))
	}
	return nil
}
