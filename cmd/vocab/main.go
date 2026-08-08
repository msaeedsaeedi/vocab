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

	"github.com/msaeedsaeedi/vocab/internal/apppaths"
	"github.com/msaeedsaeedi/vocab/internal/autostart"
	"github.com/msaeedsaeedi/vocab/internal/daemon"
	"github.com/msaeedsaeedi/vocab/internal/database"
	"github.com/msaeedsaeedi/vocab/internal/engage"
	"github.com/msaeedsaeedi/vocab/internal/instance"
	"github.com/msaeedsaeedi/vocab/internal/lexicon"
	"github.com/msaeedsaeedi/vocab/internal/notify"
	"github.com/msaeedsaeedi/vocab/internal/report"
	"github.com/msaeedsaeedi/vocab/internal/state"
	"github.com/msaeedsaeedi/vocab/internal/tray"
	"github.com/msaeedsaeedi/vocab/internal/wallpaper"
)

const supportURL = "https://github.com/msaeedsaeedi/vocab#get-help"

func main() {
	resetDB := flag.Bool("reset-db", false, "Delete Vocab learner state and start fresh")
	reviewID := flag.Int64("review", 0, "Word ID to record feedback for")
	rating := flag.Int("rating", 0, "Rating 0=forgot, 1=struggled, 2=knew (used with --review)")
	knewIt := flag.Bool("knew", false, "Shorthand for --rating 2 (deprecated)")
	produceID := flag.Int64("produce", 0, "Word ID to record production feedback for")
	produced := flag.Bool("produced", false, "Whether the user could produce a sentence (used with --produce)")
	register := flag.Bool("register", false, "Register app for Windows notifications")
	reportFlag := flag.Bool("report", false, "Write a local diagnostic bundle for bug reports")
	daemonFlag := flag.Bool("daemon", false, "Run as background daemon")
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

	devFactor := 1.0

	if *learnNow {
		if err := daemon.SendCommand("learn-now"); err != nil {
			log.Fatalf("send daemon command: %v", err)
		}
		return
	}
	if *quit {
		if err := daemon.SendCommand("quit"); err != nil {
			log.Fatalf("send daemon command: %v", err)
		}
		return
	}

	// A direct Windows launch is the normal desktop entry point. Keep the
	// explicit -daemon flag for compatibility, but show the tray for a plain
	// launch as well.
	desktopDaemon := *daemonFlag || runtime.GOOS == "windows"
	if *reportFlag || *register || *reviewID > 0 || *produceID > 0 {
		desktopDaemon = false
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
	if desktopDaemon {
		release, alreadyRunning, err := instance.Acquire()
		if err != nil {
			log.Fatalf("single-instance guard: %v", err)
		}
		if alreadyRunning {
			log.Print("another Vocab instance is already running")
			return
		}
		defer release()
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db := openDB(*resetDB)
	defer db.Close()
	store := state.New(db)

	if *reportFlag {
		path, err := report.Write(db, logPath, report.Info{
			Version: version,
			Commit:  commit,
			Date:    date,
			Go:      runtime.Version(),
			OS:      runtime.GOOS,
			Arch:    runtime.GOARCH,
		})
		if err != nil {
			log.Fatalf("report: %v", err)
		}
		fmt.Println(path)
		return
	}

	if *register {
		doRegister()
		return
	}

	if *reviewID > 0 {
		if err := handleReviewCommand(db, store, *reviewID, *knewIt, *rating); err != nil {
			log.Fatalf("review: %v", err)
		}
		return
	}

	if *produceID > 0 {
		if err := handleProduceCommand(store, *produceID, *produced); err != nil {
			log.Fatalf("produce: %v", err)
		}
		return
	}

	wordList := lexicon.LoadSeed()
	tr := engage.New(db, store)
	d := daemon.New(daemon.Config{
		Context:          ctx,
		DB:               db,
		Store:            store,
		Lexicon:          wordList,
		Tracker:          tr,
		DevFactor:        devFactor,
		RestoreWallpaper: restoreWallpaper,
	})

	if desktopDaemon {
		if !ensureWallpaperConsent(store) {
			log.Print("daemon start cancelled: wallpaper consent was declined")
			return
		}
		enableAutostart()
		registerNotifications()
		notify.SetActivationCallback(func(arguments string) {
			// COM invokes this callback on its own thread. Do database work outside
			// that call so activation can return promptly.
			go dispatchToastActivation(db, store, arguments)
		})
		go tray.Run(ctx, tray.Actions{
			LearnNow:    d.RequestLearnNow,
			PauseResume: d.TogglePaused,
			IsPaused:    d.IsPaused,
			Report: func() {
				path, err := report.Write(db, logPath, report.Info{
					Version: version,
					Commit:  commit,
					Date:    date,
					Go:      runtime.Version(),
					OS:      runtime.GOOS,
					Arch:    runtime.GOARCH,
				})
				if err != nil {
					log.Printf("report: %v", err)
					return
				}
				if err := report.Reveal(path); err != nil {
					log.Printf("report: reveal %s: %v", path, err)
				}
				if err := notify.SendStatusLink("Diagnostic report created. Explorer opened it so you can attach it to a support request.", "Get help", supportURL); err != nil {
					log.Printf("report: confirmation notification: %v", err)
				}
			},
			Quit: stop,
		})
		defer func() {
			if err := restoreWallpaper(); err != nil {
				log.Printf("restore wallpaper: %v", err)
			}
		}()
	}

	d.Run()
}

func openLog() (*os.File, string) {
	dir, err := apppaths.LogDir()
	if err != nil {
		log.Fatalf("cannot find log dir: %v", err)
	}
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

func openDB(reset bool) *database.DB {
	dir, err := apppaths.DataDir()
	if err != nil {
		log.Fatalf("cannot find data dir: %v", err)
	}
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
