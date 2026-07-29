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
	"github.com/msaeed/vocab/internal/notify"
	"github.com/msaeed/vocab/internal/scheduler"
	"github.com/msaeed/vocab/internal/seed"
	"github.com/msaeed/vocab/internal/wallpaper"
	"github.com/msaeed/vocab/internal/word"
)

func main() {
	resetDB := flag.Bool("reset-db", false, "Delete database and re-seed")
	reviewID := flag.Int64("review", 0, "Word ID to record feedback for")
	knewIt := flag.Bool("knew", false, "Whether the user knew the word (used with --review)")
	register := flag.Bool("register", false, "Register app for Windows notifications")
	daemon := flag.Bool("daemon", false, "Run as background daemon")
	flag.Parse()

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
		if err := scheduler.RecordFeedback(db, *reviewID, *knewIt); err != nil {
			log.Fatalf("record feedback: %v", err)
		}
		fmt.Fprintln(logFile, "Feedback recorded.")
		return
	}

	if *daemon {
		enableAutostart()
	}

	runDaemon(db)
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

	for {
		due, err := word.GetDueWords(db, time.Now().Format("2006-01-02"))
		if err != nil {
			log.Printf("get due words: %v", err)
			time.Sleep(30 * time.Minute)
			continue
		}

		if len(due) == 0 {
			nextDue := findNextDue(db)
			if nextDue.IsZero() || nextDue.Before(time.Now()) {
				log.Print("no words due, sleeping 1h")
				time.Sleep(1 * time.Hour)
			} else {
				dur := time.Until(nextDue)
				log.Printf("no words due, next due in %v", dur.Round(time.Minute))
				time.Sleep(dur + time.Minute)
			}
			continue
		}

		w := due[0]
		handleDueWord(db, w)
	}
}

func findNextDue(db *sql.DB) time.Time {
	rows, err := db.Query("SELECT next_due FROM words ORDER BY next_due ASC LIMIT 1")
	if err != nil {
		return time.Time{}
	}
	defer rows.Close()
	if rows.Next() {
		var s string
		if rows.Scan(&s) == nil {
			t, err := time.Parse("2006-01-02", s)
			if err == nil {
				return t
			}
		}
	}
	return time.Time{}
}

func handleDueWord(db *sql.DB, w word.Word) {
	log.Printf("=== showing word: id=%d text=%q", w.ID, w.Text)

	log.Printf("rendering wallpaper for %q", w.Text)
	if err := wallpaper.Render(wallpaper.Word{
		Text:       w.Text,
		Definition: w.Definition,
		Example:    w.Example,
	}, wallpaper.Option{}); err != nil {
		log.Printf("wallpaper ERROR: %v", err)
	} else {
		log.Print("wallpaper set successfully")
	}

	log.Printf("sending notification for word %d", w.ID)
	if err := notify.Send(notify.Word{
		ID:         w.ID,
		Text:       w.Text,
		Definition: w.Definition,
		Example:    w.Example,
	}); err != nil {
		log.Printf("notify ERROR: %v", err)
	} else {
		log.Print("notification sent")
	}

	log.Printf("waiting for review of word %d (polling 5m)", w.ID)

	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		var nextDue string
		err := db.QueryRow("SELECT next_due FROM words WHERE id = ?", w.ID).Scan(&nextDue)
		if err != nil {
			log.Printf("check review ERROR: %v", err)
			return
		}
		log.Printf("word %d next_due=%s", w.ID, nextDue)
		if nextDue > time.Now().Format("2006-01-02") {
			log.Printf("word %d reviewed, moving on", w.ID)
			return
		}
	}
}


