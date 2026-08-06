package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"math/rand/v2"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"sort"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/msaeedsaeedi/vocab/internal/autostart"
	"github.com/msaeedsaeedi/vocab/internal/database"
	"github.com/msaeedsaeedi/vocab/internal/engage"
	"github.com/msaeedsaeedi/vocab/internal/notify"
	"github.com/msaeedsaeedi/vocab/internal/scheduler"
	"github.com/msaeedsaeedi/vocab/internal/tray"
	"github.com/msaeedsaeedi/vocab/internal/wallpaper"
	"github.com/msaeedsaeedi/vocab/internal/word"
	"github.com/msaeedsaeedi/vocab/internal/words"
)

var (
	devFactor = 1.0

	learnNowRequests = make(chan struct{}, 1)
	learnNowPending  atomic.Bool
	quitPending      atomic.Bool

	version = "dev"
	commit  = "none"
	date    = "unknown"
)

var errDaemonStop = fmt.Errorf("daemon stop requested")

func main() {
	resetDB := flag.Bool("reset-db", false, "Delete Vocab learner state and start fresh")
	reviewID := flag.Int64("review", 0, "Word ID to record feedback for")
	rating := flag.Int("rating", 0, "Rating 0=forgot, 1=struggled, 2=knew (used with --review)")
	knewIt := flag.Bool("knew", false, "Shorthand for --rating 2 (deprecated)")
	produceID := flag.Int64("produce", 0, "Word ID to record production feedback for")
	produced := flag.Bool("produced", false, "Whether the user could produce a sentence (used with --produce)")
	register := flag.Bool("register", false, "Register app for Windows notifications")
	daemon := flag.Bool("daemon", false, "Run as background daemon")
	dev := flag.Bool("dev", false, "Dev mode: 60x faster timeouts for debugging")
	preview := flag.Bool("preview", false, "Generate wallpaper preview to preview.jpg without setting it")
	learnNow := flag.Bool("learn-now", false, "Ask a running daemon to start the next learning session")
	quit := flag.Bool("quit", false, "Ask a running daemon to shut down cleanly")
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

	if *learnNow {
		if err := sendDaemonCommand("learn-now"); err != nil {
			log.Fatalf("send daemon command: %v", err)
		}
		return
	}
	if *quit {
		if err := sendDaemonCommand("quit"); err != nil {
			log.Fatalf("send daemon command: %v", err)
		}
		return
	}

	if *dev {
		devFactor = 1.0 / 60.0
		log.Print("=== DEV MODE: timeouts divided by 60 ===")
	}

	logFile, logPath := openLog()
	defer logFile.Close()
	log.SetOutput(logFile)
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	log.Printf("=== vocab started (pid=%d, os=%s, version=%s) ===", os.Getpid(), runtime.GOOS, version)
	log.Printf("log file: %s", logPath)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db := openDB(*resetDB)
	defer db.Close()

	if *register {
		doRegister()
		return
	}

	if *reviewID > 0 {
		if err := handleReviewCommand(db, *reviewID, *knewIt, *rating); err != nil {
			log.Fatalf("review: %v", err)
		}
		return
	}

	if *produceID > 0 {
		if err := handleProduceCommand(db, *produceID, *produced); err != nil {
			log.Fatalf("produce: %v", err)
		}
		return
	}

	if *daemon {
		enableAutostart()
		registerNotifications()
		go tray.Run(ctx, tray.Actions{LearnNow: requestLearnNow, Quit: stop})
	}

	wordList := words.LoadSeed()
	runDaemon(ctx, db, wordList)
}

func handleReviewCommand(db *database.DB, id int64, knewDeprecated bool, ratingFlag int) error {
	r := ratingFlag
	outcome := "failure"
	if knewDeprecated && ratingFlag == 0 {
		r = 2
	}
	switch r {
	case 0:
		outcome = "failure"
	case 1:
		outcome = "struggle"
	case 2:
		outcome = "success"
	default:
		return fmt.Errorf("invalid rating %d (use 0=forgot, 1=struggled, 2=knew)", r)
	}
	log.Printf("review: id=%d rating=%d outcome=%s", id, r, outcome)
	tr := engage.New(db)

	w, err := word.GetWord(db, id)
	if err != nil {
		return fmt.Errorf("get word: %w", err)
	}

	if _, err := scheduler.ScheduleReview(db, w, r, outcome); err != nil {
		return fmt.Errorf("schedule review: %w", err)
	}

	tr.RecordEngagement()
	tr.RecordNotificationAnswered()
	log.Printf("review: recorded id=%d rating=%d", id, r)
	return nil
}

func openLog() (*os.File, string) {
	dir := filepath.Join(dataDir(), "logs")
	if err := os.MkdirAll(dir, 0755); err != nil {
		log.Fatalf("cannot create log directory %s: %v", dir, err)
	}
	path := filepath.Join(dir, "vocab.log")
	if info, err := os.Stat(path); err == nil && info.Size() >= 2*1024*1024 {
		rotated := filepath.Join(dir, "vocab.1.log")
		_ = os.Remove(rotated)
		if err := os.Rename(path, rotated); err != nil {
			log.Printf("cannot rotate log: %v", err)
		}
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		log.Fatalf("cannot open log %s: %v", path, err)
	}
	return f, path
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

// sleepDevContext sleeps for d (scaled by dev mode) while keeping the daemon
// responsive to local commands. It returns true when the full duration elapsed,
// and false when a learn-now/quit request interrupted the sleep or the context
// was cancelled.
func sleepDevContext(ctx context.Context, d time.Duration) bool {
	scaled := time.Duration(float64(d) * devFactor)
	timer := time.NewTimer(scaled)
	defer timer.Stop()
	poll := time.NewTicker(time.Second)
	defer poll.Stop()
	for {
		select {
		case <-timer.C:
			return true
		case <-ctx.Done():
			return false
		case <-learnNowRequests:
			return false
		case <-poll.C:
			drainDaemonCommand()
			if quitPending.Load() || learnNowPending.Load() {
				return false
			}
		}
	}
}

func daemonCommandPath() string { return filepath.Join(dataDir(), "daemon-command") }

func sendDaemonCommand(command string) error {
	path := daemonCommandPath()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	temporary := path + ".tmp"
	if err := os.WriteFile(temporary, []byte(command), 0600); err != nil {
		return err
	}
	return os.Rename(temporary, path)
}

func drainDaemonCommand() {
	path := daemonCommandPath()
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	if err := os.Remove(path); err != nil {
		log.Printf("daemon command: remove mailbox: %v", err)
	}
	switch string(data) {
	case "learn-now":
		requestLearnNow()
	case "quit":
		quitPending.Store(true)
		log.Print("quit requested by local command")
	default:
		log.Printf("daemon command: ignored unknown command %q", string(data))
	}
}

func requestLearnNow() {
	learnNowPending.Store(true)
	select {
	case learnNowRequests <- struct{}{}:
	default:
	}
	log.Print("learn now requested")
}

func consumeLearnNowRequest() bool {
	if !learnNowPending.Swap(false) {
		return false
	}
	select {
	case <-learnNowRequests:
	default:
	}
	log.Print("learn now honored")
	return true
}

func enableAutostart() {
	execPath, err := os.Executable()
	if err != nil {
		log.Printf("autostart: resolve executable: %v", err)
		return
	}
	if err := autostart.SetEnabled("VocabDaemon", execPath, true); err != nil {
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

func registerNotifications() {
	if runtime.GOOS != "windows" {
		return
	}
	exe, err := os.Executable()
	if err != nil {
		log.Printf("notification registration: resolve executable: %v", err)
		return
	}
	if err := notify.RegisterApp(exe); err != nil {
		log.Printf("notification registration: %v", err)
		return
	}
	log.Print("notification app registered")
}

func runDaemon(ctx context.Context, db *database.DB, wordList *words.List) {
	// A command left over from a previous invocation is stale: the installer's
	// pre-replace "quit" must not stop the daemon that starts after it.
	_ = os.Remove(daemonCommandPath())

	log.Print("daemon started")
	tr := engage.New(db)

	resumeAmbientSession(ctx, db, wordList, tr)

	for {
		drainDaemonCommand()
		if quitPending.Load() {
			log.Print("shutting down daemon on local command")
			return
		}
		select {
		case <-ctx.Done():
			log.Print("shutting down daemon")
			return
		default:
		}

		forceNow := consumeLearnNowRequest()
		now := time.Now()
		dayStart, dayEnd := tr.ActiveWindow()

		if !forceNow && (now.Hour() < dayStart || now.Hour() >= dayEnd) {
			sleepUntilNextWindow(ctx, dayStart, dayEnd)
			if ctx.Err() != nil || quitPending.Load() {
				log.Print("shutting down daemon")
				return
			}
			continue
		}

		wordsPerDay := tr.WordsPerDay()
		interWordMins := tr.InterWordMinutes(dayEnd-dayStart, wordsPerDay)
		log.Printf("adaptive: window=%d-%d words/day=%d gap=%dm",
			dayStart, dayEnd, wordsPerDay, interWordMins)

		dueWords, err := word.GetDueWords(db, now.Format("2006-01-02 15:04:05"))
		if err != nil {
			log.Printf("get due words: %v", err)
			sleepDevContext(ctx, 30*time.Minute)
			continue
		}
		if len(dueWords) == 0 {
			if err := introduceItems(db, wordList, wordsPerDay); err != nil {
				log.Printf("introduce learner items: %v", err)
			}
			dueWords, err = word.GetDueWords(db, now.Format("2006-01-02 15:04:05"))
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
				if quitPending.Load() || ctx.Err() != nil {
					return
				}
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
				if ctx.Err() != nil || quitPending.Load() {
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
			if forceNow {
				log.Print("learn now honored but no words are due right now")
			}
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
			if ctx.Err() != nil || quitPending.Load() {
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

	if shouldProduce(db, w) {
		_ = phaseProduce(ctx, db, tr, w)
	}

	if err := word.UpdatePhase(db, w.ID, ""); err != nil {
		log.Printf("update phase done: %v", err)
	}
	return nil
}

func shouldProduce(db *database.DB, w word.Word) bool {
	updated, err := word.GetWord(db, w.ID)
	if err != nil {
		return false
	}
	if updated.ReviewCount < 2 {
		return false
	}
	now := time.Now()
	elapsed := 0.0
	if updated.LastReviewed != "" {
		t, err := time.Parse("2006-01-02 15:04:05", updated.LastReviewed)
		if err == nil {
			elapsed = now.Sub(t).Hours()
		}
	}
	if scheduler.RecallProbability(updated.Stability, elapsed) < 0.75 {
		return false
	}
	return rand.Float64() < 0.15
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
		if quitPending.Load() {
			return errDaemonStop
		}
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
	tr.RecordNotificationSent()

	return waitForReview(ctx, db, tr, w)
}

func waitForReview(ctx context.Context, db *database.DB, tr *engage.Tracker, w word.Word) error {
	log.Printf("waiting for review of word %d (polling 5s, timeout 2h)", w.ID)

	deadline := time.Now().Add(devDuration(2 * time.Hour))
	pollInterval := devDuration(2 * time.Minute)
	if pollInterval > 5*time.Second {
		pollInterval = 5 * time.Second
	}

	for {
		select {
		case <-ctx.Done():
			log.Print("shutdown during review wait")
			tr.ClearCurrentWord()
			return nil

		case <-time.After(pollInterval):
			drainDaemonCommand()
			if quitPending.Load() {
				log.Print("shutting down during review wait")
				tr.ClearCurrentWord()
				return errDaemonStop
			}
			if learnNowPending.Load() {
				log.Printf("learn now queued; it will run after word %d resolves", w.ID)
			}
			current, err := word.GetWord(db, w.ID)
			if err != nil {
				log.Printf("review poll ERROR: %v", err)
				continue
			}

			if current.NextDue > time.Now().Format("2006-01-02 15:04:05") {
				log.Printf("word %d reviewed (next_due=%s)", w.ID, current.NextDue)
				tr.RecordEngagement()
				tr.RecordNotificationAnswered()
				tr.ClearCurrentWord()
				return nil
			}

			if time.Now().After(deadline) {
				log.Printf("word %d review timeout — marking as missed", w.ID)
				if _, err := scheduler.ScheduleReview(db, &w, 0, "missed"); err != nil {
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
	case "produce":
		log.Printf("resume: continuing produce wait for word %q", w.Text)
		if err := waitForProduce(ctx, db, tr, *w); err != nil {
			log.Printf("resume produce ERROR: %v", err)
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
	w.Collocation = entry.Collocation
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

	type candidate struct {
		id   string
		rank int
	}
	var candidates []candidate
	for _, id := range wordList.IDs() {
		if known[id] {
			continue
		}
		entry, ok := wordList.Lookup(id)
		if !ok {
			continue
		}
		rank := entry.FrequencyRank
		if rank == 0 {
			rank = 10000
		}
		candidates = append(candidates, candidate{id: id, rank: rank})
	}
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].rank < candidates[j].rank
	})

	introduced := 0
	for _, c := range candidates {
		entry, ok := wordList.Lookup(c.id)
		if !ok {
			continue
		}
		item := word.Word{LexemeID: entry.LexemeID, NextDue: "1970-01-01 00:00:00"}
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
	t, err := time.Parse("2006-01-02 15:04:05", nextDue)
	if err != nil {
		t, err = time.Parse("2006-01-02", nextDue)
		if err != nil {
			return time.Time{}
		}
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

func handleProduceCommand(db *database.DB, id int64, produced bool) error {
	val := "no"
	if produced {
		val = "yes"
	}
	log.Printf("produce: id=%d produced=%s", id, val)
	if _, err := db.Exec(
		`INSERT INTO daemon_state (key, value) VALUES (?, ?)
		 ON CONFLICT(key) DO UPDATE SET value = ?`,
		fmt.Sprintf("produce_%d", id), val, val,
	); err != nil {
		return fmt.Errorf("store produce result: %w", err)
	}
	return nil
}

func phaseProduce(ctx context.Context, db *database.DB, tr *engage.Tracker, w word.Word) error {
	log.Printf("produce: asking for sentence with %q (id=%d)", w.Text, w.ID)
	tr.PutCurrentWord(w.ID, "produce")

	if err := notify.SendProduction(notify.Word{ID: w.ID, Text: w.Text}); err != nil {
		log.Printf("produce notify ERROR: %v", err)
		tr.ClearCurrentWord()
		return err
	}
	log.Print("produce notification sent")
	tr.PutLastNotificationTime(time.Now())
	tr.RecordNotificationSent()

	return waitForProduce(ctx, db, tr, w)
}

func waitForProduce(ctx context.Context, db *database.DB, tr *engage.Tracker, w word.Word) error {
	key := fmt.Sprintf("produce_%d", w.ID)
	log.Printf("waiting for production response (word %d, polling 5s, timeout 30m)", w.ID)

	deadline := time.Now().Add(devDuration(30 * time.Minute))
	pollInterval := devDuration(2 * time.Minute)
	if pollInterval > 5*time.Second {
		pollInterval = 5 * time.Second
	}

	for {
		select {
		case <-ctx.Done():
			log.Print("shutdown during produce wait")
			tr.ClearCurrentWord()
			return nil

		case <-time.After(pollInterval):
			drainDaemonCommand()
			if quitPending.Load() {
				log.Print("shutting down during produce wait")
				tr.ClearCurrentWord()
				return errDaemonStop
			}

			var val string
			err := db.QueryRow(`SELECT value FROM daemon_state WHERE key = ?`, key).Scan(&val)
			if err == nil {
				if _, delErr := db.Exec(`DELETE FROM daemon_state WHERE key = ?`, key); delErr != nil {
					log.Printf("produce: cleanup key: %v", delErr)
				}
				log.Printf("produce: word %d result=%s", w.ID, val)
				tr.ClearCurrentWord()
				return nil
			}

			if time.Now().After(deadline) {
				log.Printf("produce: timeout for word %d", w.ID)
				tr.ClearCurrentWord()
				return nil
			}
		}
	}
}
