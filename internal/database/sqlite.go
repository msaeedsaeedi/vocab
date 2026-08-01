package database

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

type DB struct {
	*sql.DB
}

func Open(path string) (*DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}
	d := &DB{DB: db}
	if _, err := d.Exec(schema); err != nil {
		return nil, fmt.Errorf("create schema: %w", err)
	}
	return d, nil
}

const schema = `
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

CREATE TABLE IF NOT EXISTS daemon_state (
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
