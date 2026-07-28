package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/msaeed/vocab/internal/database"
	"github.com/msaeed/vocab/internal/display"
	"github.com/msaeed/vocab/internal/feedback"
	"github.com/msaeed/vocab/internal/scheduler"
	"github.com/msaeed/vocab/internal/seed"
	"github.com/msaeed/vocab/internal/word"
)

func main() {
	dataDir := os.Getenv("VOCAB_DATA_DIR")
	if dataDir == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			log.Fatalf("cannot find home dir: %v", err)
		}
		dataDir = filepath.Join(homeDir, ".local", "share", "vocab")
	}
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		log.Fatalf("cannot create data dir: %v", err)
	}

	db, err := database.Open(filepath.Join(dataDir, "vocab.db"))
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer db.Close()

	needed, err := seed.Needed(db)
	if err != nil {
		log.Fatalf("check seed: %v", err)
	}
	if needed {
		seedPath := "data/words.json"
		if _, err := os.Stat(seedPath); err == nil {
			seed.MustFromJSON(db, seedPath)
		}
	}

	today := time.Now().Format("2006-01-02")
	due, err := word.GetDueWords(db, today)
	if err != nil {
		log.Fatalf("get due words: %v", err)
	}
	if len(due) == 0 {
		fmt.Println("No words due today. Come back tomorrow!")
		return
	}

	w := due[0]
	display.Word(w)

	total, err := word.Count(db)
	if err != nil {
		log.Printf("count: %v", err)
	}
	dueCount, err := word.CountDue(db, today)
	if err != nil {
		log.Printf("count due: %v", err)
	}
	display.Stats(total, dueCount)

	knewIt, err := feedback.Prompt()
	if err != nil {
		log.Fatalf("feedback: %v", err)
	}

	if err := scheduler.RecordFeedback(db, w.ID, knewIt); err != nil {
		log.Fatalf("record feedback: %v", err)
	}

	fmt.Println("Got it! Word reviewed.")
}

func init() {
	log.SetFlags(0)
}
