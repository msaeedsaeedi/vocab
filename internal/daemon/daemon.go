// Package daemon runs the ambient learning loop: it presents words through
// wallpaper exposure and recall/production notifications, and observes the
// learner's engagement to adapt the daily schedule.
package daemon

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync/atomic"
	"time"

	"github.com/msaeedsaeedi/vocab/internal/apppaths"
	"github.com/msaeedsaeedi/vocab/internal/database"
	"github.com/msaeedsaeedi/vocab/internal/engage"
	"github.com/msaeedsaeedi/vocab/internal/lexicon"
	"github.com/msaeedsaeedi/vocab/internal/scheduler"
	"github.com/msaeedsaeedi/vocab/internal/state"
	"github.com/msaeedsaeedi/vocab/internal/word"
)

var (
	errDaemonStop     = fmt.Errorf("daemon stop requested")
	errLearningPaused = fmt.Errorf("learning paused")
)

// Config wires the daemon's collaborators. DevFactor scales every timeout and
// is 1.0 in normal operation (1/60 in dev mode).
type Config struct {
	Context          context.Context
	DB               *database.DB
	Store            *state.Store
	Lexicon          *lexicon.List
	Tracker          *engage.Tracker
	DevFactor        float64
	RestoreWallpaper func() error
}

// Daemon owns the ambient learning loop and the in-process control signals
// (learn-now, pause/resume, quit) that the tray and local commands trigger.
type Daemon struct {
	ctx              context.Context
	db               *database.DB
	store            *state.Store
	list             *lexicon.List
	tr               *engage.Tracker
	devFactor        float64
	restoreWallpaper func() error

	learnNowRequests chan struct{}
	learnNowPending  atomic.Bool
	learningPaused   atomic.Bool
	quitPending      atomic.Bool
}

// New returns a Daemon driven by cfg.
func New(cfg Config) *Daemon {
	if cfg.DevFactor <= 0 {
		cfg.DevFactor = 1.0
	}
	return &Daemon{
		ctx:              cfg.Context,
		db:               cfg.DB,
		store:            cfg.Store,
		list:             cfg.Lexicon,
		tr:               cfg.Tracker,
		devFactor:        cfg.DevFactor,
		restoreWallpaper: cfg.RestoreWallpaper,
		learnNowRequests: make(chan struct{}, 1),
	}
}

// Run drives the daemon until the context is cancelled, quit is requested, or
// a fatal session error occurs.
func (d *Daemon) Run() {
	// A command left over from a previous invocation is stale: the installer's
	// pre-replace "quit" must not stop the daemon that starts after it.
	if path, err := apppaths.CommandPath(); err == nil {
		_ = os.Remove(path)
	}

	log.Print("daemon started")
	d.loadPaused()

	if !d.learningPaused.Load() {
		d.resumeAmbientSession()
	}

	for {
		d.drainCommand()
		if d.quitPending.Load() {
			log.Print("shutting down daemon on local command")
			return
		}
		select {
		case <-d.ctx.Done():
			log.Print("shutting down daemon")
			return
		default:
		}
		if d.learningPaused.Load() {
			if !d.waitWhilePaused() {
				return
			}
			continue
		}

		forceNow := d.consumeLearnNowRequest()
		now := time.Now()
		dayStart, dayEnd := d.tr.ActiveWindow()

		if !forceNow && (now.Hour() < dayStart || now.Hour() >= dayEnd) {
			d.sleepUntilNextWindow(dayStart, dayEnd)
			if d.ctx.Err() != nil || d.quitPending.Load() {
				log.Print("shutting down daemon")
				return
			}
			continue
		}

		wordsPerDay := d.tr.WordsPerDay()
		interWordMins := d.tr.InterWordMinutes(dayEnd-dayStart, wordsPerDay)
		log.Printf("adaptive: window=%d-%d words/day=%d gap=%dm",
			dayStart, dayEnd, wordsPerDay, interWordMins)

		dueWords, err := word.GetDueWords(d.db, now.Format(word.TimeLayout))
		if err != nil {
			log.Printf("get due words: %v", err)
			d.sleep(30 * time.Minute)
			continue
		}
		if len(dueWords) == 0 {
			if err := d.introduceItems(wordsPerDay); err != nil {
				log.Printf("introduce learner items: %v", err)
			}
			dueWords, err = word.GetDueWords(d.db, now.Format(word.TimeLayout))
			if err != nil {
				log.Printf("refresh introduced items: %v", err)
				continue
			}
		}

		presented := 0
		for presented < wordsPerDay && len(dueWords) > 0 {
			w := scheduler.SelectNextWord(dueWords)
			if w == nil {
				break
			}

			if err := d.hydrateWord(w); err != nil {
				log.Printf("load lexical content for %s: %v", w.LexemeID, err)
				continue
			}
			if err := d.presentWord(*w); err != nil {
				log.Printf("present word %d: %v", w.ID, err)
				if d.quitPending.Load() || d.ctx.Err() != nil {
					return
				}
				if d.learningPaused.Load() {
					d.store.ClearCurrentWord()
					break
				}
			}

			presented++
			d.tr.PutLastWordTime(time.Now())

			if presented < wordsPerDay {
				waitMins := interWordMins
				remainingDay := dayEnd - time.Now().Hour()
				if remainingDay > 0 && waitMins > remainingDay*60/(wordsPerDay-presented) {
					waitMins = remainingDay * 60 / (wordsPerDay - presented + 1)
				}
				log.Printf("waiting %dm before next word", waitMins)
				d.sleep(time.Duration(waitMins) * time.Minute)
				if d.ctx.Err() != nil || d.quitPending.Load() {
					log.Print("shutting down daemon")
					return
				}
			}

			dueWords = refreshDueWords(dueWords, w.ID)
			if time.Now().Hour() >= dayEnd {
				break
			}
		}

		if presented == 0 {
			if forceNow {
				log.Print("learn now honored but no words are due right now")
			}
			nextDue := d.findNextDue()
			if nextDue.IsZero() || nextDue.Before(time.Now()) {
				log.Print("no words due, sleeping 30m")
				d.sleep(30 * time.Minute)
			} else {
				dur := time.Until(nextDue)
				log.Printf("no words due, next in %v", dur.Round(time.Minute))
				if dur > time.Hour {
					d.sleep(time.Hour)
				} else {
					d.sleep(dur + time.Minute)
				}
			}
			if d.ctx.Err() != nil || d.quitPending.Load() {
				log.Print("shutting down daemon")
				return
			}
		}
	}
}

// RequestLearnNow asks the loop to start the next learning session promptly.
func (d *Daemon) RequestLearnNow() {
	d.requestLearnNow()
}

// IsPaused reports whether the learning loop is paused.
func (d *Daemon) IsPaused() bool {
	return d.learningPaused.Load()
}

// TogglePaused flips the pause state, persisting it and restoring the desktop
// wallpaper when pausing.
func (d *Daemon) TogglePaused() {
	paused := !d.learningPaused.Load()
	if err := d.store.SetPaused(paused); err != nil {
		log.Printf("pause learning: %v", err)
		return
	}
	d.learningPaused.Store(paused)
	if paused {
		log.Print("learning paused")
		if d.restoreWallpaper != nil {
			if err := d.restoreWallpaper(); err != nil {
				log.Printf("restore wallpaper on pause: %v", err)
			}
		}
		return
	}
	log.Print("learning resumed")
}

// devDuration scales d by the dev factor.
func (d *Daemon) devDuration(dur time.Duration) time.Duration {
	return time.Duration(float64(dur) * d.devFactor)
}

func (d *Daemon) loadPaused() {
	paused := d.store.Paused()
	d.learningPaused.Store(paused)
	if paused {
		log.Print("learning is paused")
	}
}

// sleep blocks for dur (scaled by the dev factor) while keeping the daemon
// responsive to local commands. It returns true when the full duration elapsed,
// and false when a learn-now/quit request interrupted the sleep or the context
// was cancelled.
func (d *Daemon) sleep(dur time.Duration) bool {
	scaled := time.Duration(float64(dur) * d.devFactor)
	timer := time.NewTimer(scaled)
	defer timer.Stop()
	poll := time.NewTicker(time.Second)
	defer poll.Stop()
	for {
		select {
		case <-timer.C:
			return true
		case <-d.ctx.Done():
			return false
		case <-d.learnNowRequests:
			return false
		case <-poll.C:
			d.drainCommand()
			if d.quitPending.Load() || d.learnNowPending.Load() || d.learningPaused.Load() {
				return false
			}
		}
	}
}

func (d *Daemon) waitWhilePaused() bool {
	for d.learningPaused.Load() {
		select {
		case <-d.ctx.Done():
			return false
		case <-time.After(500 * time.Millisecond):
			d.drainCommand()
			if d.quitPending.Load() {
				return false
			}
		}
	}
	return true
}

func (d *Daemon) sleepUntilNextWindow(start, end int) {
	now := time.Now()
	tomorrow := time.Date(now.Year(), now.Month(), now.Day()+1, start, 0, 0, 0, now.Location())
	sleepDur := tomorrow.Sub(now)
	if sleepDur > 8*time.Hour {
		sleepDur = 8 * time.Hour
	}
	log.Printf("outside active window (%d-%d), sleeping %v", start, end, sleepDur.Round(time.Minute))
	d.sleep(sleepDur)
}

func (d *Daemon) requestLearnNow() {
	d.learnNowPending.Store(true)
	select {
	case d.learnNowRequests <- struct{}{}:
	default:
	}
	log.Print("learn now requested")
}

func (d *Daemon) consumeLearnNowRequest() bool {
	if !d.learnNowPending.Swap(false) {
		return false
	}
	select {
	case <-d.learnNowRequests:
	default:
	}
	log.Print("learn now honored")
	return true
}
