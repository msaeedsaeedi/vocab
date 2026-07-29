package database

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

const schema = `
CREATE TABLE IF NOT EXISTS words (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    text          TEXT    NOT NULL,
    definition    TEXT    NOT NULL DEFAULT '',
    example       TEXT    NOT NULL DEFAULT '',
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
CREATE INDEX IF NOT EXISTS idx_words_next_due ON words(next_due);

CREATE TABLE IF NOT EXISTS review_log (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    word_id        INTEGER NOT NULL,
    rating         INTEGER NOT NULL,
    elapsed_hours  REAL    NOT NULL DEFAULT 0,
    stability      REAL    NOT NULL DEFAULT 1.0,
    timestamp      TEXT    NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY (word_id) REFERENCES words(id)
);
CREATE INDEX IF NOT EXISTS idx_review_log_word ON review_log(word_id);

CREATE TABLE IF NOT EXISTS engagement (
    hour         INTEGER PRIMARY KEY,
    score        REAL    NOT NULL DEFAULT 0.0,
    sample_count INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS daemon_state (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);
`

func Open(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}
	if _, err := db.Exec(schema); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	if err := migrate(db); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return db, nil
}

func migrate(db *sql.DB) error {
	cols, err := existingColumns(db, "words")
	if err != nil {
		return err
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
				return fmt.Errorf("migrate add column: %w", err)
			}
		}
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
