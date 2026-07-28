package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/msaeed/vocab/internal/display"
	"github.com/msaeed/vocab/internal/feedback"
	"github.com/msaeed/vocab/internal/scheduler"
	"github.com/msaeed/vocab/internal/word"
)

func main() {
	dataDir := os.Getenv("VOCAB_DATA_DIR")
	if dataDir == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			log.Fatalf("cannot find home dir: %v", err)
		}
		dataDir = filepath.Join(homeDir, ".config", "vocab")
	}
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		log.Fatalf("cannot create data dir: %v", err)
	}

	store := word.NewStore(filepath.Join(dataDir, "words.json"))

	builtinWords, err := os.ReadFile("data/words.json")
	if err == nil {
		tmpStore := word.NewStore("")
		if err := tmpStore.Load(); err == nil {
			_ = os.WriteFile(filepath.Join(dataDir, "words.json"), builtinWords, 0644)
		}
	}

	if err := store.Load(); err != nil {
		log.Printf("warning: no existing word data, using defaults")
	}

	all := store.All()
	if len(all) == 0 {
		store.Add(word.Entry{Word: "hello", Meaning: "a greeting", Usage: "Hello, world!"},
			word.Entry{Word: "vocab", Meaning: "a vocabulary widget", Usage: "Vocab helps you learn words."})
		_ = store.Save()
	}

	sch := scheduler.New(store)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	todayWord := store.TodaysWord()
	if todayWord == nil {
		fmt.Println("No words available.")
		return
	}

	showMeaning := sch.ShowMeaningToday()
	showUsage := sch.ShowUsageToday()

	display.Word(todayWord, showMeaning, showUsage)

	all = store.All()
	display.Stats(len(all), sch.DueCount())

	result, err := feedback.Prompt()
	if err != nil {
		log.Printf("feedback error: %v", err)
		result = feedback.Unknown
	}

	quality := feedback.MapToQuality(result)
	sch.Review(todayWord, quality)

	_ = store.Save()

	fmt.Println()
	fmt.Println("  Word reviewed! See you tomorrow.")
}

func init() {
	log.SetFlags(0)
	log.SetPrefix("")
}
