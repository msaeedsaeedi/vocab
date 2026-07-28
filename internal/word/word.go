package word

import (
	"database/sql"
	"fmt"
)

type Word struct {
	ID         int64  `json:"id"`
	Text       string `json:"text"`
	Definition string `json:"definition"`
	Example    string `json:"example"`
	Box        int    `json:"box"`
	NextDue    string `json:"next_due"`
}

func Insert(db *sql.DB, w *Word) error {
	res, err := db.Exec(
		`INSERT INTO words (text, definition, example, box, next_due) VALUES (?, ?, ?, ?, ?)`,
		w.Text, w.Definition, w.Example, w.Box, w.NextDue,
	)
	if err != nil {
		return fmt.Errorf("insert word: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("last insert id: %w", err)
	}
	w.ID = id
	return nil
}

func UpdateFeedback(db *sql.DB, id int64, box int, nextDue string) error {
	res, err := db.Exec(
		`UPDATE words SET box = ?, next_due = ? WHERE id = ?`,
		box, nextDue, id,
	)
	if err != nil {
		return fmt.Errorf("update word: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("word %d not found", id)
	}
	return nil
}

func GetDueWords(db *sql.DB, today string) ([]Word, error) {
	rows, err := db.Query(
		`SELECT id, text, definition, example, box, next_due
		 FROM words WHERE next_due <= ?
		 ORDER BY box ASC, RANDOM()`, today,
	)
	if err != nil {
		return nil, fmt.Errorf("query due words: %w", err)
	}
	defer rows.Close()

	var words []Word
	for rows.Next() {
		var w Word
		if err := rows.Scan(&w.ID, &w.Text, &w.Definition, &w.Example, &w.Box, &w.NextDue); err != nil {
			return nil, fmt.Errorf("scan word: %w", err)
		}
		words = append(words, w)
	}
	return words, rows.Err()
}

func GetAll(db *sql.DB) ([]Word, error) {
	rows, err := db.Query(
		`SELECT id, text, definition, example, box, next_due FROM words ORDER BY id`,
	)
	if err != nil {
		return nil, fmt.Errorf("query all words: %w", err)
	}
	defer rows.Close()

	var words []Word
	for rows.Next() {
		var w Word
		if err := rows.Scan(&w.ID, &w.Text, &w.Definition, &w.Example, &w.Box, &w.NextDue); err != nil {
			return nil, fmt.Errorf("scan word: %w", err)
		}
		words = append(words, w)
	}
	return words, rows.Err()
}

func Count(db *sql.DB) (int, error) {
	var n int
	err := db.QueryRow(`SELECT COUNT(*) FROM words`).Scan(&n)
	return n, err
}

func CountDue(db *sql.DB, today string) (int, error) {
	var n int
	err := db.QueryRow(`SELECT COUNT(*) FROM words WHERE next_due <= ?`, today).Scan(&n)
	return n, err
}
