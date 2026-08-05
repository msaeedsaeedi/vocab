package database

import (
	"database/sql"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

const currentSchemaVersion = 1

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
	if err := d.migrate(path); err != nil {
		_ = d.Close()
		return nil, err
	}
	return d, nil
}

// migrate brings the database to currentSchemaVersion. Existing v0.x databases
// report user_version = 0 but already contain some tables; re-applying the
// idempotent schema is safe for them. The migration runs in a transaction and a
// byte-for-byte backup is taken first so a failure can never destroy learner
// state. Databases already on the current version skip the backup and schema
// work entirely on normal startups.
func (d *DB) migrate(path string) error {
	var integrity string
	if err := d.QueryRow("PRAGMA integrity_check").Scan(&integrity); err != nil {
		return fmt.Errorf("integrity check: %w", err)
	}
	if integrity != "ok" {
		return fmt.Errorf("integrity check failed: %s", integrity)
	}

	var version int
	if err := d.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		return fmt.Errorf("read schema version: %w", err)
	}
	if version > currentSchemaVersion {
		return fmt.Errorf("database schema %d is newer than supported schema %d", version, currentSchemaVersion)
	}
	if version == currentSchemaVersion {
		return nil
	}

	backup := ""
	if path != ":memory:" {
		var err error
		backup, err = backupDatabase(path)
		if err != nil {
			return err
		}
	}

	tx, err := d.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(schema); err != nil {
		migrateErr := fmt.Errorf("apply schema: %w", err)
		if backup != "" {
			if restoreErr := restoreBackup(path, backup); restoreErr != nil {
				return fmt.Errorf("%w (backup restore also failed: %v)", migrateErr, restoreErr)
			}
		}
		return migrateErr
	}
	if _, err := tx.Exec(fmt.Sprintf("PRAGMA user_version = %d", currentSchemaVersion)); err != nil {
		return fmt.Errorf("write schema version: %w", err)
	}
	return tx.Commit()
}

// backupDatabase preserves a byte-for-byte diagnostic copy of the database right
// before the versioned migration runs.
func backupDatabase(path string) (string, error) {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("stat database: %w", err)
	}
	if info.Size() == 0 {
		return "", nil
	}
	backup := filepath.Join(filepath.Dir(path), fmt.Sprintf("vocab.db.pre-migration-%s.bak", time.Now().UTC().Format("20060102T150405Z")))
	if err := copyFile(path, backup); err != nil {
		return "", fmt.Errorf("backup database: %w", err)
	}
	return backup, nil
}

func restoreBackup(path, backup string) error {
	failed := path + ".failed-" + time.Now().UTC().Format("20060102T150405Z")
	if err := os.Rename(path, failed); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("preserve failed database: %w", err)
	}
	return copyFile(backup, path)
}

func copyFile(source, destination string) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
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
`
