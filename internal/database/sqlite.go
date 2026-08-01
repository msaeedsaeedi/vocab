package database

import (
	"database/sql"
	"fmt"
	"log"

	_ "modernc.org/sqlite"
)

// DB wraps *sql.DB and manages schema migrations.
type DB struct {
	*sql.DB
}

// Open opens (or creates) a SQLite database and runs any pending migrations.
func Open(path string) (*DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}
	d := &DB{DB: db}
	if err := d.migrate(); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return d, nil
}

type migration struct {
	Version int
	SQL     string
	Desc    string
}

var migrations = []migration{
	{1, schemaV1, "Vocab-owned learner state"},
	{2, schemaV2, "split learner state from canonical lexical content"},
}

const schemaV1 = `
CREATE TABLE IF NOT EXISTS learning_items (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    lexeme_id     TEXT    NOT NULL,
    sense_id      TEXT    NOT NULL DEFAULT '',
    dataset_version TEXT  NOT NULL,
    box           INTEGER NOT NULL DEFAULT 0,
    next_due      TEXT    NOT NULL DEFAULT (date('now')),
    stability     REAL    NOT NULL DEFAULT 1.0,
    difficulty    REAL    NOT NULL DEFAULT 0.3,
    bkt_alpha     REAL    NOT NULL DEFAULT 1.0,
    bkt_beta      REAL    NOT NULL DEFAULT 1.0,
    last_reviewed TEXT    NOT NULL DEFAULT '',
    review_count  INTEGER NOT NULL DEFAULT 0,
    lapse_count   INTEGER NOT NULL DEFAULT 0,
    exposure_phase TEXT   NOT NULL DEFAULT ''
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_learning_items_lexeme_dataset ON learning_items(lexeme_id, dataset_version);
CREATE INDEX IF NOT EXISTS idx_learning_items_next_due ON learning_items(next_due);

CREATE TABLE IF NOT EXISTS review_events (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    learning_item_id INTEGER NOT NULL REFERENCES learning_items(id),
    rating        INTEGER NOT NULL,
    elapsed_hours REAL    NOT NULL DEFAULT 0,
    stability     REAL    NOT NULL DEFAULT 1.0,
    timestamp     TEXT    NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_review_events_item ON review_events(learning_item_id);

CREATE TABLE IF NOT EXISTS exposures (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    learning_item_id INTEGER NOT NULL REFERENCES learning_items(id),
    phase            TEXT NOT NULL,
    started_at       TEXT NOT NULL DEFAULT (datetime('now')),
    completed_at     TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_exposures_item ON exposures(learning_item_id);

CREATE TABLE IF NOT EXISTS engagement (
    hour         INTEGER PRIMARY KEY,
    score        REAL    NOT NULL DEFAULT 0.0,
    sample_count INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS settings (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS installed_datasets (
    dataset_version TEXT PRIMARY KEY,
    schema_version  TEXT NOT NULL,
    path            TEXT NOT NULL,
    installed_at    TEXT NOT NULL DEFAULT (datetime('now')),
    active          INTEGER NOT NULL DEFAULT 0 CHECK (active IN (0, 1))
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_installed_datasets_active ON installed_datasets(active) WHERE active = 1;
`

// schemaV2 lets the pre-v0.3 prototype start cleanly with the new state
// model. Its legacy words table is intentionally left untouched: v0.3 does
// not migrate old learner state, and it never reads those rows.
const schemaV2 = schemaV1

func (db *DB) migrate() error {
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_version (
		version    INTEGER PRIMARY KEY,
		applied_at TEXT NOT NULL DEFAULT (datetime('now'))
	)`); err != nil {
		return fmt.Errorf("create schema_version table: %w", err)
	}

	current, err := db.currentVersion()
	if err != nil {
		return fmt.Errorf("read current schema version: %w", err)
	}

	for _, m := range migrations {
		if m.Version > current {
			log.Printf("applying migration %d: %s", m.Version, m.Desc)
			if _, err := db.Exec(m.SQL); err != nil {
				return fmt.Errorf("migration %d (%s): %w", m.Version, m.Desc, err)
			}
			if _, err := db.Exec(`INSERT INTO schema_version (version) VALUES (?)`, m.Version); err != nil {
				return fmt.Errorf("record migration %d: %w", m.Version, err)
			}
			current = m.Version
		}
	}

	if err := db.runAdHocMigrations(current); err != nil {
		return fmt.Errorf("adhoc migrations: %w", err)
	}
	return nil
}

func (db *DB) currentVersion() (int, error) {
	var version int
	err := db.QueryRow(`SELECT COALESCE(MAX(version), 0) FROM schema_version`).Scan(&version)
	return version, err
}

// runAdHocMigrations handles pre-versioned-schema migrations
// for databases created before this migration system existed.
func (db *DB) runAdHocMigrations(current int) error {
	if current >= 1 {
		return nil
	}

	cols, err := existingColumns(db.DB, "words")
	if err != nil {
		return err
	}
	if !hasColumn(cols, "pos") {
		for _, stmt := range []string{
			`ALTER TABLE words ADD COLUMN pos TEXT NOT NULL DEFAULT ''`,
		} {
			if _, err := db.Exec(stmt); err != nil {
				return fmt.Errorf("adhoc add pos: %w", err)
			}
		}
	}
	if !hasColumn(cols, "stability") {
		for _, stmt := range []string{
			`ALTER TABLE words ADD COLUMN stability REAL NOT NULL DEFAULT 1.0`,
			`ALTER TABLE words ADD COLUMN difficulty REAL NOT NULL DEFAULT 0.3`,
			`ALTER TABLE words ADD COLUMN bkt_alpha REAL NOT NULL DEFAULT 1.0`,
			`ALTER TABLE words ADD COLUMN bkt_beta REAL NOT NULL DEFAULT 1.0`,
			`ALTER TABLE words ADD COLUMN last_reviewed TEXT NOT NULL DEFAULT ''`,
			`ALTER TABLE words ADD COLUMN review_count INTEGER NOT NULL DEFAULT 0`,
			`ALTER TABLE words ADD COLUMN lapse_count INTEGER NOT NULL DEFAULT 0`,
			`ALTER TABLE words ADD COLUMN exposure_phase TEXT NOT NULL DEFAULT ''`,
		} {
			if _, err := db.Exec(stmt); err != nil {
				return fmt.Errorf("adhoc add stability: %w", err)
			}
		}
	}

	if _, err := db.Exec(`INSERT OR IGNORE INTO schema_version (version) VALUES (1)`, current); err != nil {
		return fmt.Errorf("record adhoc migration: %w", err)
	}
	return nil
}

func existingColumns(db *sql.DB, table string) (map[string]bool, error) {
	rows, err := db.Query(fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	cols := make(map[string]bool)
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return nil, err
		}
		cols[name] = true
	}
	return cols, rows.Err()
}

func hasColumn(cols map[string]bool, name string) bool {
	return cols[name]
}
