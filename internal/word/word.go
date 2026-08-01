package word

import (
	"database/sql"
	"fmt"

	"github.com/msaeedsaeedi/vocab/internal/database"
)

type Word struct {
	ID             int64  `json:"id"`
	LexemeID       string `json:"lexeme_id"`
	SenseID        string `json:"sense_id,omitempty"`
	DatasetVersion string `json:"dataset_version"`
	// Text, Definition, Example, and Pos are presentation fields hydrated from
	// the read-only Lexicon dataset. They are never persisted in vocab.db.
	Text          string  `json:"text"`
	Definition    string  `json:"definition"`
	Example       string  `json:"example"`
	Pos           string  `json:"pos,omitempty"`
	Box           int     `json:"box"`
	NextDue       string  `json:"next_due"`
	Stability     float64 `json:"stability"`
	Difficulty    float64 `json:"difficulty"`
	BktAlpha      float64 `json:"bkt_alpha"`
	BktBeta       float64 `json:"bkt_beta"`
	LastReviewed  string  `json:"last_reviewed"`
	ReviewCount   int     `json:"review_count"`
	LapseCount    int     `json:"lapse_count"`
	ExposurePhase string  `json:"exposure_phase"`
}

type ReviewLog struct {
	ID           int64   `json:"id"`
	WordID       int64   `json:"learning_item_id"`
	Rating       int     `json:"rating"`
	ElapsedHours float64 `json:"elapsed_hours"`
	Stability    float64 `json:"stability"`
	Timestamp    string  `json:"timestamp"`
}

func Insert(db *database.DB, w *Word) error {
	res, err := db.Exec(
		`INSERT INTO learning_items (lexeme_id, sense_id, dataset_version, box, next_due)
		 VALUES (?, ?, ?, ?, ?)`,
		w.LexemeID, w.SenseID, w.DatasetVersion, w.Box, w.NextDue,
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

func UpdateFeedback(db *database.DB, id int64, box int, nextDue string) error {
	res, err := db.Exec(
		`UPDATE learning_items SET box = ?, next_due = ? WHERE id = ?`,
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

func UpdateAdaptive(db *database.DB, id int64, stability, difficulty, bktAlpha, bktBeta float64, reviewCount, lapseCount int, lastReviewed, nextDue, phase string) error {
	_, err := db.Exec(
		`UPDATE learning_items SET
			stability = ?, difficulty = ?, bkt_alpha = ?, bkt_beta = ?,
			review_count = ?, lapse_count = ?, last_reviewed = ?,
			next_due = ?, exposure_phase = ?
		 WHERE id = ?`,
		stability, difficulty, bktAlpha, bktBeta,
		reviewCount, lapseCount, lastReviewed,
		nextDue, phase, id,
	)
	if err != nil {
		return fmt.Errorf("update adaptive word: %w", err)
	}
	return nil
}

func UpdatePhase(db *database.DB, id int64, phase string) error {
	_, err := db.Exec(`UPDATE learning_items SET exposure_phase = ? WHERE id = ?`, phase, id)
	return err
}

func InsertReviewLog(db *database.DB, log *ReviewLog) error {
	_, err := db.Exec(
		`INSERT INTO review_events (learning_item_id, rating, elapsed_hours, stability, timestamp)
		 VALUES (?, ?, ?, ?, datetime('now'))`,
		log.WordID, log.Rating, log.ElapsedHours, log.Stability,
	)
	return err
}

func GetWord(db *database.DB, id int64) (*Word, error) {
	w := &Word{}
	err := db.QueryRow(
		`SELECT id, lexeme_id, sense_id, dataset_version, box, next_due,
		        stability, difficulty, bkt_alpha, bkt_beta,
		        last_reviewed, review_count, lapse_count, exposure_phase
		 FROM learning_items WHERE id = ?`, id,
	).Scan(&w.ID, &w.LexemeID, &w.SenseID, &w.DatasetVersion,
		&w.Box, &w.NextDue,
		&w.Stability, &w.Difficulty, &w.BktAlpha, &w.BktBeta,
		&w.LastReviewed, &w.ReviewCount, &w.LapseCount, &w.ExposurePhase)
	if err != nil {
		return nil, err
	}
	return w, nil
}

func GetDueWords(db *database.DB, today string) ([]Word, error) {
	rows, err := db.Query(
		`SELECT id, lexeme_id, sense_id, dataset_version, box, next_due,
		        stability, difficulty, bkt_alpha, bkt_beta,
		        last_reviewed, review_count, lapse_count, exposure_phase
		 FROM learning_items WHERE next_due <= ?
		 ORDER BY box ASC, RANDOM()`, today,
	)
	if err != nil {
		return nil, fmt.Errorf("query due words: %w", err)
	}
	defer rows.Close()
	return scanWords(rows)
}

func GetDueWordCount(db *database.DB, today string) (int, error) {
	var n int
	err := db.QueryRow(`SELECT COUNT(*) FROM learning_items WHERE next_due <= ?`, today).Scan(&n)
	return n, err
}

func GetNextDue(db *database.DB) (string, error) {
	var nextDue string
	err := db.QueryRow(
		`SELECT next_due FROM learning_items ORDER BY next_due ASC LIMIT 1`,
	).Scan(&nextDue)
	return nextDue, err
}

func GetAll(db *database.DB) ([]Word, error) {
	rows, err := db.Query(
		`SELECT id, lexeme_id, sense_id, dataset_version, box, next_due,
		        stability, difficulty, bkt_alpha, bkt_beta,
		        last_reviewed, review_count, lapse_count, exposure_phase
		 FROM learning_items ORDER BY id`,
	)
	if err != nil {
		return nil, fmt.Errorf("query all words: %w", err)
	}
	defer rows.Close()
	return scanWords(rows)
}

func Count(db *database.DB) (int, error) {
	var n int
	err := db.QueryRow(`SELECT COUNT(*) FROM learning_items`).Scan(&n)
	return n, err
}

func CountDue(db *database.DB, today string) (int, error) {
	var n int
	err := db.QueryRow(`SELECT COUNT(*) FROM learning_items WHERE next_due <= ?`, today).Scan(&n)
	return n, err
}

func scanWords(rows *sql.Rows) ([]Word, error) {
	var words []Word
	for rows.Next() {
		var w Word
		if err := rows.Scan(&w.ID, &w.LexemeID, &w.SenseID, &w.DatasetVersion,
			&w.Box, &w.NextDue,
			&w.Stability, &w.Difficulty, &w.BktAlpha, &w.BktBeta,
			&w.LastReviewed, &w.ReviewCount, &w.LapseCount, &w.ExposurePhase); err != nil {
			return nil, fmt.Errorf("scan word: %w", err)
		}
		words = append(words, w)
	}
	return words, rows.Err()
}
