package main

import (
	"context"
	"database/sql"
	_ "embed"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/msaeed/vocab/internal/autostart"
	"github.com/msaeed/vocab/internal/config"
	"github.com/msaeed/vocab/internal/database"
	"github.com/msaeed/vocab/internal/scheduler"
	"github.com/msaeed/vocab/internal/seed"
	"github.com/msaeed/vocab/internal/word"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed data/words.json
var wordsJSON []byte

//go:embed build/appicon.png
var appIconBytes []byte

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

type WidgetConfig struct {
	WindowX     int  `json:"window_x"`
	WindowY     int  `json:"window_y"`
	AlwaysOnTop bool `json:"always_on_top"`
	AutoStart   bool `json:"auto_start"`
}

type App struct {
	ctx      context.Context
	db       *sql.DB
	cfg      *config.Manager
	quitting bool
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

	need, err := seed.Needed(db)
	if err != nil {
		log.Fatalf("check seed: %v", err)
	}
	if need {
		seed.MustFromJSONBytes(db, wordsJSON)
	}

	mgr, err := config.NewManager(dataDir)
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	a.cfg = mgr

	a.startSystray()

	cfg := a.cfg.Get()
	runtime.WindowSetAlwaysOnTop(ctx, cfg.AlwaysOnTop)
	if cfg.WindowX >= 0 && cfg.WindowY >= 0 {
		runtime.WindowSetPosition(ctx, cfg.WindowX, cfg.WindowY)
	}
}

func (a *App) shutdown(ctx context.Context) {
	a.cfg.Save()
}

func (a *App) onBeforeClose(ctx context.Context) (prevent bool) {
	if a.quitting {
		return false
	}
	runtime.Hide(ctx)
	return true
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

func (a *App) SaveWindowPosition(x, y int) {
	a.cfg.SetWindowPosition(x, y)
	a.cfg.Save()
}

func (a *App) HideToTray() {
	runtime.Hide(a.ctx)
}

func (a *App) ToggleWindow() {
	if runtime.WindowIsNormal(a.ctx) {
		runtime.Hide(a.ctx)
	} else {
		runtime.Show(a.ctx)
	}
}

func (a *App) GetConfig() WidgetConfig {
	c := a.cfg.Get()
	return WidgetConfig{
		WindowX:     c.WindowX,
		WindowY:     c.WindowY,
		AlwaysOnTop: c.AlwaysOnTop,
		AutoStart:   c.AutoStart,
	}
}

func (a *App) SetAutoStart(enabled bool) error {
	execPath, err := os.Executable()
	if err != nil {
		return err
	}
	if err := autostart.SetEnabled("vocab", execPath, enabled); err != nil {
		return err
	}
	a.cfg.SetAutoStart(enabled)
	a.cfg.Save()
	return nil
}

func (a *App) IsAutoStart() bool {
	return autostart.Enabled("vocab")
}
