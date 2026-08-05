package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"github.com/msaeedsaeedi/vocab/internal/autostart"
	"github.com/msaeedsaeedi/vocab/internal/database"
	"github.com/msaeedsaeedi/vocab/internal/engage"
	"github.com/msaeedsaeedi/vocab/internal/notify"
	"github.com/msaeedsaeedi/vocab/internal/scheduler"
	"github.com/msaeedsaeedi/vocab/internal/wallpaper"
	"github.com/msaeedsaeedi/vocab/internal/word"
	"github.com/msaeedsaeedi/vocab/internal/words"
)

var (
	devFactor = 1.0

	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	resetDB := flag.Bool("reset-db", false, "Delete Vocab learner state and start fresh")
	reviewID := flag.Int64("review", 0, "Word ID to record feedback for")
	knewIt := flag.Bool("knew", false, "Whether the user knew the word (used with --review)")
	register := flag.Bool("register", false, "Register app for Windows notifications")
	daemon := flag.Bool("daemon", false, "Run as background daemon")
	dev := flag.Bool("dev", false, "Dev mode: 60x faster timeouts for debugging")
	preview := flag.Bool("preview", false, "Generate wallpaper preview to preview.jpg without setting it")
	showVersion := flag.Bool("version", false, "Print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Printf("vocab %s (commit=%s, built=%s, go=%s, os=%s/%s)\n",
			version, commit, date, runtime.Version(), runtime.GOOS, runtime.GOARCH)
		return
	}

	if *preview {
		w := wallpaper.Word{
			Text:       "Fortuitous",
			Definition: "happening by accident",
			Example:    "Their meeting was entirely fortuitous.",
			Pos:        "adj.",
		}
		if err := wallpaper.RenderPreview(w, "preview.jpg", wallpaper.Option{Width: 1920, Height: 1080}); err != nil {
			log.Fatalf("preview: %v", err)
		}
		fmt.Println("preview.jpg written")
		return
	}

	if *dev {
		devFactor = 1.0 / 60.0
		log.Print("=== DEV MODE: timeouts divided by 60 ===")
	}

	logFile := openLog()
	defer logFile.Close()
	log.SetOutput(logFile)
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	log.Printf("=== vocab started (pid=%d, os=%s, version=%s) ===", os.Getpid(), runtime.GOOS, version)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db := openDB(*resetDB)
	defer db.Close()

	if *register {
		doRegister()
		return
	}

	if *reviewID > 0 {
		if err := handleReviewCommand(db, *reviewID, *knewIt); err != nil {
			log.Fatalf("review: %v", err)
		}
		return
	}

	if *daemon {
		enableAutostart()
	}

	wordList := words.LoadSeed()
	runDaemon(ctx, db, wordList)
}

func handleReviewCommand(db *database.DB, id int64, knew bool) error {
	log.Printf("review: id=%d knew=%v", id, knew)
	tr := engage.New(db)

	w, err := word.GetWord(db, id)
	if err != nil {
		return fmt.Errorf("get word: %w", err)
	}

	rating := 0
	if knew {
		rating = 2
	}
	if _, err := scheduler.ScheduleReview(db, w, rating); err != nil {
		return fmt.Errorf("schedule review: %w", err)
	}

	tr.RecordEngagement()
	log.Printf("review: recorded id=%d rating=%d", id, rating)
	return nil
}

func openLog() *os.File {
	path := filepath.Join(os.TempDir(), "vocab.log")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		log.Fatalf("cannot open log %s: %v", path, err)
	}
	return f
}

func dataDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		log.Fatalf("cannot find home dir: %v", err)
	}
	if runtime.GOOS == "windows" {
		return filepath.Join(home, "AppData", "Roaming", "vocab")
	}
	return filepath.Join(home, ".local", "share", "vocab")
}

func openDB(reset bool) *database.DB {
	dir := dataDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		log.Fatalf("cannot create data dir %s: %v", dir, err)
	}
	log.Printf("data dir: %s", dir)

	dbPath := filepath.Join(dir, "vocab.db")
	if reset {
		if err := os.Remove(dbPath); err != nil && !os.IsNotExist(err) {
			log.Fatalf("remove db: %v", err)
		}
		log.Print("database reset")
	}

	db, err := database.Open(dbPath)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	log.Printf("database opened: %s", dbPath)
	return db
}

func devDuration(d time.Duration) time.Duration { return time.Duration(float64(d) * devFactor) }

func sleepDevContext(ctx context.Context, d time.Duration) bool {
	scaled := time.Duration(float64(d) * devFactor)
	select {
	case <-time.After(scaled):
		return true
	case <-ctx.Done():
		return false
	}
}

func enableAutostart() {
	execPath, err := os.Executable()
	if err != nil {
		log.Printf("autostart: resolve executable: %v", err)
		return
	}
	if err := autostart.SetEnabled("vocab", execPath, true); err != nil {
		log.Printf("autostart: enable: %v", err)
	} else {
		log.Print("autostart enabled")
	}
}

func doRegister() {
	exe, err := os.Executable()
	if err != nil {
		log.Fatalf("resolve executable: %v", err)
	}
	if err := notify.RegisterApp(exe); err != nil {
		log.Fatalf("register app: %v", err)
	}
	log.Print("notification app registered")
}

func runDaemon(ctx context.Context, db *database.DB, wordList *words.List) {
	log.Print("daemon started")
	tr := engage.New(db)

	resumeAmbientSession(ctx, db, wordList, tr)

	for {
		select {
		case <-ctx.Done():
			log.Print("shutting down daemon")
			return
		default:
		}

		now := time.Now()
		dayStart, dayEnd := tr.ActiveWindow()

		if now.Hour() < dayStart || now.Hour() >= dayEnd {
			sleepUntilNextWindow(ctx, dayStart, dayEnd)
			if ctx.Err() != nil {
				log.Print("shutting down daemon")
				return
			}
			continue
		}

		wordsPerDay := tr.WordsPerDay()
		interWordMins := tr.InterWordMinutes(dayEnd-dayStart, wordsPerDay)
		log.Printf("adaptive: window=%d-%d words/day=%d gap=%dm",
			dayStart, dayEnd, wordsPerDay, interWordMins)

		dueWords, err := word.GetDueWords(db, now.Format("2006-01-02"))
		if err != nil {
			log.Printf("get due words: %v", err)
			sleepDevContext(ctx, 30*time.Minute)
			continue
		}
		if len(dueWords) == 0 {
			if err := introduceItems(db, wordList, wordsPerDay); err != nil {
				log.Printf("introduce learner items: %v", err)
			}
			dueWords, err = word.GetDueWords(db, now.Format("2006-01-02"))
			if err != nil {
				log.Printf("refresh introduced items: %v", err)
				continue
			}
		}

		presented := 0
		for presented < wordsPerDay && len(dueWords) > 0 {
			w := scheduler.SelectNextWord(db, dueWords)
			if w == nil {
				break
			}

			if err := hydrateWord(wordList, w); err != nil {
				log.Printf("load lexical content for %s: %v", w.LexemeID, err)
				continue
			}
			if err := presentWord(ctx, db, tr, *w); err != nil {
				log.Printf("present word %d: %v", w.ID, err)
			}

			presented++
			tr.PutLastWordTime(time.Now())

			if presented < wordsPerDay {
				waitMins := interWordMins
				remainingDay := dayEnd - time.Now().Hour()
				if remainingDay > 0 && waitMins > remainingDay*60/(wordsPerDay-presented) {
					waitMins = remainingDay * 60 / (wordsPerDay - presented + 1)
				}
				log.Printf("waiting %dm before next word", waitMins)
				sleepDevContext(ctx, time.Duration(waitMins)*time.Minute)
				if ctx.Err() != nil {
					log.Print("shutting down daemon")
					return
				}
			}

			dueWords = refreshDueWords(db, dueWords, w.ID)
			if time.Now().Hour() >= dayEnd {
				break
			}
		}

		if presented == 0 {
			nextDue := findNextDue(db)
			if nextDue.IsZero() || nextDue.Before(time.Now()) {
				log.Print("no words due, sleeping 30m")
				sleepDevContext(ctx, 30*time.Minute)
			} else {
				dur := time.Until(nextDue)
				log.Printf("no words due, next in %v", dur.Round(time.Minute))
				if dur > time.Hour {
					sleepDevContext(ctx, time.Hour)
				} else {
					sleepDevContext(ctx, dur+time.Minute)
				}
			}
			if ctx.Err() != nil {
				log.Print("shutting down daemon")
				return
			}
		}
	}
}

func presentWord(ctx context.Context, db *database.DB, tr *engage.Tracker, w word.Word) error {
	log.Printf("=== presenting word: id=%d text=%q", w.ID, w.Text)

	if err := phraseExpose(ctx, db, tr, w); err != nil {
		return fmt.Errorf("expose phase: %w", err)
	}

	if err := phaseRecall(ctx, db, tr, w); err != nil {
		return fmt.Errorf("recall phase: %w", err)
	}

	if err := word.UpdatePhase(db, w.ID, ""); err != nil {
		log.Printf("update phase done: %v", err)
	}
	return nil
}

func phraseExpose(ctx context.Context, db *database.DB, tr *engage.Tracker, w word.Word) error {
	log.Printf("expose: setting wallpaper for %q", w.Text)
	if err := word.UpdatePhase(db, w.ID, "expose"); err != nil {
		log.Printf("update phase expose: %v", err)
	}
	tr.PutCurrentWord(w.ID, "expose")

	if err := wallpaper.Render(wallpaper.Word{
		Text:       w.Text,
		Definition: w.Definition,
		Example:    w.Example,
		Pos:        w.Pos,
	}, wallpaper.Option{}); err != nil {
		log.Printf("wallpaper ERROR: %v", err)
		return err
	}
	log.Print("wallpaper set (expose phase)")

	absorptionTime := 30 * time.Minute
	log.Printf("expose: absorption period %v", absorptionTime)
	if !sleepDevContext(ctx, absorptionTime) {
		return ctx.Err()
	}

	return nil
}

func phaseRecall(ctx context.Context, db *database.DB, tr *engage.Tracker, w word.Word) error {
	log.Printf("recall: notifying for word %q (id=%d)", w.Text, w.ID)
	if err := word.UpdatePhase(db, w.ID, "recall"); err != nil {
		log.Printf("update phase recall: %v", err)
	}
	tr.PutCurrentWord(w.ID, "recall")

	if err := notify.Send(notify.Word{
		ID:   w.ID,
		Text: w.Text,
	}); err != nil {
		log.Printf("notify ERROR: %v", err)
		return err
	}
	log.Print("recall notification sent")
	tr.PutLastNotificationTime(time.Now())

	return waitForReview(ctx, db, tr, w)
}

func waitForReview(ctx context.Context, db *database.DB, tr *engage.Tracker, w word.Word) error {
	log.Printf("waiting for review of word %d (polling 2m, timeout 2h)", w.ID)

	deadline := time.Now().Add(devDuration(2 * time.Hour))
	pollInterval := devDuration(2 * time.Minute)

	for {
		select {
		case <-ctx.Done():
			log.Print("shutdown during review wait")
			tr.ClearCurrentWord()
			return nil

		case <-time.After(pollInterval):
			current, err := word.GetWord(db, w.ID)
			if err != nil {
				log.Printf("review poll ERROR: %v", err)
				continue
			}

			if current.NextDue > time.Now().Format("2006-01-02") {
				log.Printf("word %d reviewed (next_due=%s)", w.ID, current.NextDue)
				tr.RecordEngagement()
				tr.ClearCurrentWord()
				return nil
			}

			if time.Now().After(deadline) {
				log.Printf("word %d review timeout — auto-lapsing", w.ID)
				if _, err := scheduler.ScheduleReview(db, &w, 0); err != nil {
					return fmt.Errorf("auto-lapse: %w", err)
				}
				if err := word.UpdatePhase(db, w.ID, ""); err != nil {
					log.Printf("update phase: %v", err)
				}
				tr.ClearCurrentWord()
				return nil
			}
		}
	}
}

func resumeAmbientSession(ctx context.Context, db *database.DB, wordList *words.List, tr *engage.Tracker) {
	wordID, phase := tr.GetCurrentWord()
	if wordID == 0 {
		return
	}

	log.Printf("resume: word=%d phase=%s", wordID, phase)

	w, err := word.GetWord(db, wordID)
	if err != nil {
		log.Printf("resume: word %d not found: %v", wordID, err)
		tr.ClearCurrentWord()
		return
	}
	if err := hydrateWord(wordList, w); err != nil {
		log.Printf("resume: load lexical content for %s: %v", w.LexemeID, err)
		tr.ClearCurrentWord()
		return
	}

	switch phase {
	case "expose":
		log.Printf("resume: re-entering recall phase for word %q", w.Text)
		if err := phaseRecall(ctx, db, tr, *w); err != nil {
			log.Printf("resume recall ERROR: %v", err)
		}
	case "recall":
		log.Printf("resume: continuing review wait for word %q", w.Text)
		if err := waitForReview(ctx, db, tr, *w); err != nil {
			log.Printf("resume review ERROR: %v", err)
		}
	}
}

func hydrateWord(wordList *words.List, w *word.Word) error {
	entry, ok := wordList.Lookup(w.LexemeID)
	if !ok {
		return fmt.Errorf("word %q not found in seed", w.LexemeID)
	}
	w.Text = entry.Text
	w.Definition = entry.Definition
	w.Example = entry.Example
	w.Pos = entry.Pos
	return nil
}

func introduceItems(db *database.DB, wordList *words.List, count int) error {
	existing, err := word.GetAll(db)
	if err != nil {
		return err
	}
	known := make(map[string]bool, len(existing))
	for _, item := range existing {
		known[item.LexemeID] = true
	}
	ids := wordList.IDs()
	introduced := 0
	for _, id := range ids {
		if known[id] {
			continue
		}
		entry, ok := wordList.Lookup(id)
		if !ok {
			continue
		}
		item := word.Word{LexemeID: entry.LexemeID, NextDue: "1970-01-01"}
		if err := word.Insert(db, &item); err != nil {
			return err
		}
		introduced++
		if introduced == count {
			break
		}
	}
	if introduced == 0 {
		return fmt.Errorf("no unlearned words remain in seed")
	}
	return nil
}

func findNextDue(db *database.DB) time.Time {
	nextDue, err := word.GetNextDue(db)
	if err != nil || nextDue == "" {
		return time.Time{}
	}
	t, err := time.Parse("2006-01-02", nextDue)
	if err != nil {
		return time.Time{}
	}
	return t
}

func refreshDueWords(db *database.DB, current []word.Word, excludeID int64) []word.Word {
	filtered := make([]word.Word, 0, len(current)-1)
	for _, w := range current {
		if w.ID != excludeID {
			filtered = append(filtered, w)
		}
	}
	return filtered
}

func sleepUntilNextWindow(ctx context.Context, start, end int) {
	now := time.Now()
	tomorrow := time.Date(now.Year(), now.Month(), now.Day()+1, start, 0, 0, 0, now.Location())
	sleepDur := tomorrow.Sub(now)
	if sleepDur > 8*time.Hour {
		sleepDur = 8 * time.Hour
	}
	log.Printf("outside active window (%d-%d), sleeping %v", start, end, sleepDur.Round(time.Minute))
	sleepDevContext(ctx, sleepDur)
}
