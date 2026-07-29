package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/msaeed/vocab/internal/autostart"
	"github.com/msaeed/vocab/internal/database"
	"github.com/msaeed/vocab/internal/engage"
	"github.com/msaeed/vocab/internal/notify"
	"github.com/msaeed/vocab/internal/scheduler"
	"github.com/msaeed/vocab/internal/seed"
	"github.com/msaeed/vocab/internal/wallpaper"
	"github.com/msaeed/vocab/internal/word"
)

var devFactor = 1.0

func main() {
	resetDB := flag.Bool("reset-db", false, "Delete database and re-seed")
	reviewID := flag.Int64("review", 0, "Word ID to record feedback for")
	knewIt := flag.Bool("knew", false, "Whether the user knew the word (used with --review)")
	register := flag.Bool("register", false, "Register app for Windows notifications")
	daemon := flag.Bool("daemon", false, "Run as background daemon")
	dev := flag.Bool("dev", false, "Dev mode: 60x faster timeouts for debugging")
	preview := flag.Bool("preview", false, "Generate wallpaper preview to preview.jpg without setting it")
	flag.Parse()

	if *preview {
		if err := wallpaper.RenderPreview(wallpaper.Word{
			Text:       "ambiguous",
			Definition: "Not clear or exact; open to more than one interpretation.",
			Example:    "Her reply was deliberately ambiguous, leaving room for both readings.",
			Pos:        "adjective",
			Phonetic:   "/æmˈbɪɡ.juəs/",
		}, "preview.jpg", wallpaper.Option{Width: 1920, Height: 1080}); err != nil {
			log.Fatalf("preview: %v", err)
		}
		fmt.Println("preview.jpg written — open it with: wslview preview.jpg")
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
	log.Printf("=== vocab started (pid=%d, os=%s) ===", os.Getpid(), runtime.GOOS)

	db := openDB(*resetDB)
	defer db.Close()

	seedIfNeeded(db)

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

	runDaemon(db)
}

func handleReviewCommand(db *sql.DB, id int64, knew bool) error {
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

func openDB(reset bool) *sql.DB {
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

func seedIfNeeded(db *sql.DB) {
	needed, err := seed.Needed(db)
	if err != nil {
		log.Fatalf("check seed: %v", err)
	}
	if !needed {
		return
	}
	seedPath := findSeedFile()
	if seedPath != "" {
		seed.MustFromJSON(db, seedPath)
		log.Printf("seeded from %s", seedPath)
	} else {
		log.Print("no seed file found, will retry on next run")
	}
}

func findSeedFile() string {
	candidates := []string{"data/words.json"}
	if exe, err := os.Executable(); err == nil {
		candidates = append(candidates,
			filepath.Join(filepath.Dir(exe), "data", "words.json"),
			filepath.Join(filepath.Dir(exe), "words.json"),
		)
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	log.Printf("seed file not found (tried: %v)", candidates)
	return ""
}

func sleepDev(d time.Duration)                  { time.Sleep(time.Duration(float64(d) * devFactor)) }
func devDuration(d time.Duration) time.Duration { return time.Duration(float64(d) * devFactor) }

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

func runDaemon(db *sql.DB) {
	log.Print("daemon started")
	tr := engage.New(db)

	resumeAmbientSession(db, tr)

	for {
		now := time.Now()
		dayStart, dayEnd := tr.ActiveWindow()

		if now.Hour() < dayStart || now.Hour() >= dayEnd {
			sleepUntilNextWindow(dayStart, dayEnd)
			continue
		}

		wordsPerDay := tr.WordsPerDay()
		interWordMins := tr.InterWordMinutes(dayEnd-dayStart, wordsPerDay)
		log.Printf("adaptive: window=%d-%d words/day=%d gap=%dm",
			dayStart, dayEnd, wordsPerDay, interWordMins)

		dueWords, err := word.GetDueWords(db, now.Format("2006-01-02"))
		if err != nil {
			log.Printf("get due words: %v", err)
			sleepDev(30 * time.Minute)
			continue
		}

		presented := 0
		for presented < wordsPerDay && len(dueWords) > 0 {
			w := scheduler.SelectNextWord(db, dueWords)
			if w == nil {
				break
			}

			if err := presentWord(db, tr, *w); err != nil {
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
				sleepDev(time.Duration(waitMins) * time.Minute)
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
				sleepDev(30 * time.Minute)
			} else {
				dur := time.Until(nextDue)
				log.Printf("no words due, next in %v", dur.Round(time.Minute))
				if dur > time.Hour {
					sleepDev(time.Hour)
				} else {
					sleepDev(dur + time.Minute)
				}
			}
		}
	}
}

func presentWord(db *sql.DB, tr *engage.Tracker, w word.Word) error {
	log.Printf("=== presenting word: id=%d text=%q", w.ID, w.Text)

	if err := phraseExpose(db, tr, w); err != nil {
		return fmt.Errorf("expose phase: %w", err)
	}

	if err := phaseRecall(db, tr, w); err != nil {
		return fmt.Errorf("recall phase: %w", err)
	}

	if err := word.UpdatePhase(db, w.ID, ""); err != nil {
		log.Printf("update phase done: %v", err)
	}
	return nil
}

func phraseExpose(db *sql.DB, tr *engage.Tracker, w word.Word) error {
	log.Printf("expose: setting wallpaper for %q", w.Text)
	if err := word.UpdatePhase(db, w.ID, "expose"); err != nil {
		log.Printf("update phase expose: %v", err)
	}
	tr.PutCurrentWord(w.ID, "expose")

	if err := wallpaper.Render(wallpaper.Word{
		Text:       w.Text,
		Definition: w.Definition,
		Example:    w.Example,
	}, wallpaper.Option{}); err != nil {
		log.Printf("wallpaper ERROR: %v", err)
		return err
	}
	log.Print("wallpaper set (expose phase)")

	absorptionTime := 30 * time.Minute
	log.Printf("expose: absorption period %v", absorptionTime)
	sleepDev(absorptionTime)

	return nil
}

func phaseRecall(db *sql.DB, tr *engage.Tracker, w word.Word) error {
	log.Printf("recall: notifying for word %q (id=%d)", w.Text, w.ID)
	if err := word.UpdatePhase(db, w.ID, "recall"); err != nil {
		log.Printf("update phase recall: %v", err)
	}
	tr.PutCurrentWord(w.ID, "recall")

	if err := notify.Send(notify.Word{
		ID:         w.ID,
		Text:       w.Text,
		Definition: w.Definition,
		Example:    w.Example,
	}); err != nil {
		log.Printf("notify ERROR: %v", err)
		return err
	}
	log.Print("recall notification sent")
	tr.PutLastNotificationTime(time.Now())

	return waitForReview(db, tr, w)
}

func waitForReview(db *sql.DB, tr *engage.Tracker, w word.Word) error {
	log.Printf("waiting for review of word %d (polling 2m, timeout 2h)", w.ID)

	deadline := time.Now().Add(devDuration(2 * time.Hour))
	ticker := time.NewTicker(devDuration(2 * time.Minute))
	defer ticker.Stop()

	for range ticker.C {
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

	return nil
}

func resumeAmbientSession(db *sql.DB, tr *engage.Tracker) {
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

	switch phase {
	case "expose":
		log.Printf("resume: re-entering recall phase for word %q", w.Text)
		if err := phaseRecall(db, tr, *w); err != nil {
			log.Printf("resume recall ERROR: %v", err)
		}
	case "recall":
		log.Printf("resume: continuing review wait for word %q", w.Text)
		if err := waitForReview(db, tr, *w); err != nil {
			log.Printf("resume review ERROR: %v", err)
		}
	}
}

func findNextDue(db *sql.DB) time.Time {
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

func refreshDueWords(db *sql.DB, current []word.Word, excludeID int64) []word.Word {
	filtered := make([]word.Word, 0, len(current)-1)
	for _, w := range current {
		if w.ID != excludeID {
			filtered = append(filtered, w)
		}
	}
	return filtered
}

func sleepUntilNextWindow(start, end int) {
	now := time.Now()
	tomorrow := time.Date(now.Year(), now.Month(), now.Day()+1, start, 0, 0, 0, now.Location())
	sleepDur := tomorrow.Sub(now)
	if sleepDur > 8*time.Hour {
		sleepDur = 8 * time.Hour
	}
	log.Printf("outside active window (%d-%d), sleeping %v", start, end, sleepDur.Round(time.Minute))
	sleepDev(sleepDur)
}
