package main

import (
	"context"
	"database/sql"
	"log"
	"os"
	"path/filepath"
	"time"

	_ "embed"

	"github.com/msaeed/vocab/internal/database"
	"github.com/msaeed/vocab/internal/scheduler"
	"github.com/msaeed/vocab/internal/seed"
	"github.com/msaeed/vocab/internal/word"
)

//go:embed data/words.json
var wordsJSON []byte

type WordCard struct {
	ID         int64  `json:"id"`
	Text       string `json:"text"`
	Definition string `json:"definition"`
	Example    string `json:"example"`
	Box        int    `json:"box"`
}

type Stats struct {
	Total    int `json:"total"`
	DueToday int `json:"due_today"`
}

type App struct {
	ctx context.Context
	db  *sql.DB
}

func NewApp() *App {
	return &App{}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Fatalf("cannot find home dir: %v", err)
	}
	dataDir := filepath.Join(homeDir, ".local", "share", "vocab")
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		log.Fatalf("cannot create data dir: %v", err)
	}

	db, err := database.Open(filepath.Join(dataDir, "vocab.db"))
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	a.db = db

	needed, err := seed.Needed(db)
	if err != nil {
		log.Fatalf("check seed: %v", err)
	}
	if needed {
		seed.MustFromJSONBytes(db, wordsJSON)
	}
}

func (a *App) GetDueWord() *WordCard {
	today := time.Now().Format("2006-01-02")
	due, err := word.GetDueWords(a.db, today)
	if err != nil || len(due) == 0 {
		return nil
	}
	w := due[0]
	return &WordCard{
		ID:         w.ID,
		Text:       w.Text,
		Definition: w.Definition,
		Example:    w.Example,
		Box:        w.Box,
	}
}

func (a *App) RecordFeedback(wordID int64, knewIt bool) error {
	return scheduler.RecordFeedback(a.db, wordID, knewIt)
}

func (a *App) GetStats() Stats {
	today := time.Now().Format("2006-01-02")
	total, err := word.Count(a.db)
	if err != nil {
		log.Printf("count: %v", err)
	}
	dueCount, err := word.CountDue(a.db, today)
	if err != nil {
		log.Printf("count due: %v", err)
	}
	return Stats{Total: total, DueToday: dueCount}
}
