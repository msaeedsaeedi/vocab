// Package state is a small typed key-value store over the daemon_state and
// settings tables. It owns every piece of ephemeral learner/daemon state so
// callers never run raw SQL against those tables.
package state

import (
	"database/sql"
	"fmt"
	"log"
	"strconv"

	"github.com/msaeedsaeedi/vocab/internal/database"
)

const (
	// KeyLearningPaused marks whether the ambient learning loop is paused.
	KeyLearningPaused = "learning_paused"
	// KeyCurrentWordID and KeyCurrentPhase record the in-flight word session.
	KeyCurrentWordID = "current_word_id"
	KeyCurrentPhase  = "current_phase"
	// KeyWallpaperConsent records whether the user accepted wallpaper exposure.
	KeyWallpaperConsent = "wallpaper_consent"
	// ValueConsentAccepted is the accepted value for KeyWallpaperConsent.
	ValueConsentAccepted = "accepted"
)

// Store reads and writes the daemon_state and settings tables.
type Store struct {
	db *database.DB
}

// New wraps db with a typed state store.
func New(db *database.DB) *Store {
	return &Store{db: db}
}

// GetState reads a value from the daemon_state table.
func (s *Store) GetState(key string) (string, bool) {
	var value string
	err := s.db.QueryRow(`SELECT value FROM daemon_state WHERE key = ?`, key).Scan(&value)
	if err != nil {
		return "", false
	}
	return value, true
}

// SetState upserts a value into the daemon_state table.
func (s *Store) SetState(key, value string) error {
	_, err := s.db.Exec(
		`INSERT INTO daemon_state (key, value) VALUES (?, ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
		key, value,
	)
	return err
}

// DeleteState removes the given keys from the daemon_state table.
func (s *Store) DeleteState(keys ...string) {
	for _, key := range keys {
		if _, err := s.db.Exec(`DELETE FROM daemon_state WHERE key = ?`, key); err != nil {
			log.Printf("state: delete %s: %v", key, err)
		}
	}
}

// GetSetting reads a value from the settings table.
func (s *Store) GetSetting(key string) (string, bool) {
	var value string
	err := s.db.QueryRow(`SELECT value FROM settings WHERE key = ?`, key).Scan(&value)
	if err != nil {
		return "", false
	}
	return value, true
}

// SetSetting upserts a value into the settings table.
func (s *Store) SetSetting(key, value string) error {
	_, err := s.db.Exec(
		`INSERT INTO settings (key, value) VALUES (?, ?)
		 ON CONFLICT(key) DO UPDATE SET value = ?`,
		key, value, value,
	)
	return err
}

// Paused reports whether ambient learning is paused.
func (s *Store) Paused() bool {
	value, ok := s.GetState(KeyLearningPaused)
	return ok && value == "1"
}

// SetPaused records whether ambient learning is paused.
func (s *Store) SetPaused(paused bool) error {
	value := "0"
	if paused {
		value = "1"
	}
	return s.SetState(KeyLearningPaused, value)
}

// CurrentWord returns the in-flight word ID and phase, or zero when idle.
func (s *Store) CurrentWord() (id int64, phase string) {
	idStr, ok := s.GetState(KeyCurrentWordID)
	if !ok {
		return 0, ""
	}
	var phaseOK bool
	phase, phaseOK = s.GetState(KeyCurrentPhase)
	if !phaseOK || !validPhase(phase) {
		return 0, ""
	}
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		return 0, ""
	}
	return id, phase
}

// SetCurrentWord records the in-flight word session. Invalid input is logged
// and ignored so new writes cannot create an unusable session.
func (s *Store) SetCurrentWord(id int64, phase string) {
	if id <= 0 || !validPhase(phase) {
		log.Printf("state: refusing invalid current word id=%d phase=%q", id, phase)
		return
	}
	if err := s.setCurrentWord(id, phase); err != nil {
		log.Printf("state: set current word: %v", err)
	}
}

func (s *Store) setCurrentWord(id int64, phase string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err != sql.ErrTxDone {
			log.Printf("state: rollback current word: %v", err)
		}
	}()

	if _, err := tx.Exec(
		`INSERT INTO daemon_state (key, value) VALUES (?, ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
		KeyCurrentWordID, strconv.FormatInt(id, 10),
	); err != nil {
		return err
	}
	if _, err := tx.Exec(
		`INSERT INTO daemon_state (key, value) VALUES (?, ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
		KeyCurrentPhase, phase,
	); err != nil {
		return err
	}
	return tx.Commit()
}

func validPhase(phase string) bool {
	switch phase {
	case "expose", "recall", "produce":
		return true
	default:
		return false
	}
}

// ClearCurrentWord forgets the in-flight word session.
func (s *Store) ClearCurrentWord() {
	s.DeleteState(KeyCurrentWordID, KeyCurrentPhase)
}

// ProduceResultKey is the daemon_state key holding the user's answer to a
// production prompt for the given word.
func ProduceResultKey(wordID int64) string {
	return "produce_" + strconv.FormatInt(wordID, 10)
}

// SetProduceResult records whether the user could produce wordID in a sentence.
func (s *Store) SetProduceResult(wordID int64, produced bool) error {
	value := "no"
	if produced {
		value = "yes"
	}
	return s.SetState(ProduceResultKey(wordID), value)
}

// TakeProduceResult reads and removes the production answer for wordID,
// reporting whether one had been recorded.
func (s *Store) TakeProduceResult(wordID int64) (string, bool) {
	key := ProduceResultKey(wordID)
	value, ok := s.GetState(key)
	if !ok {
		return "", false
	}
	if _, err := s.db.Exec(`DELETE FROM daemon_state WHERE key = ?`, key); err != nil {
		log.Printf("state: delete produce result: %v", err)
	}
	return value, true
}

// WallpaperConsentAccepted reports whether the user accepted wallpaper exposure.
func (s *Store) WallpaperConsentAccepted() bool {
	value, ok := s.GetState(KeyWallpaperConsent)
	return ok && value == ValueConsentAccepted
}

// AcceptWallpaperConsent records the user's consent to wallpaper exposure.
func (s *Store) AcceptWallpaperConsent() error {
	if err := s.SetState(KeyWallpaperConsent, ValueConsentAccepted); err != nil {
		return fmt.Errorf("save wallpaper consent: %w", err)
	}
	return nil
}
