package daemon

import (
	"fmt"
	"log"
	"math/rand/v2"
	"time"

	"github.com/msaeedsaeedi/vocab/internal/notify"
	"github.com/msaeedsaeedi/vocab/internal/scheduler"
	"github.com/msaeedsaeedi/vocab/internal/wallpaper"
	"github.com/msaeedsaeedi/vocab/internal/word"
)

// presentWord runs the exposure, recall, and (when warranted) production phases
// for a single word.
func (d *Daemon) presentWord(w word.Word) error {
	log.Printf("=== presenting word: id=%d text=%q ===", w.ID, w.Text)

	if err := d.phraseExpose(w); err != nil {
		return fmt.Errorf("expose phase: %w", err)
	}

	if err := d.phaseRecall(w); err != nil {
		return fmt.Errorf("recall phase: %w", err)
	}

	if d.shouldProduce(w) {
		if err := d.phaseProduce(w); err != nil {
			log.Printf("produce phase: %v", err)
		}
	}

	if err := word.UpdatePhase(d.db, w.ID, ""); err != nil {
		log.Printf("update phase done: %v", err)
	}
	return nil
}

func (d *Daemon) phraseExpose(w word.Word) error {
	log.Printf("expose: setting wallpaper for %q", w.Text)
	if err := word.UpdatePhase(d.db, w.ID, "expose"); err != nil {
		log.Printf("update phase expose: %v", err)
	}
	d.store.SetCurrentWord(w.ID, "expose")

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
	if !d.sleep(absorptionTime) {
		if d.quitPending.Load() {
			return errDaemonStop
		}
		if d.learningPaused.Load() {
			d.store.ClearCurrentWord()
			return errLearningPaused
		}
		return d.ctx.Err()
	}

	return nil
}

func (d *Daemon) phaseRecall(w word.Word) error {
	log.Printf("recall: notifying for word %q (id=%d)", w.Text, w.ID)
	if err := word.UpdatePhase(d.db, w.ID, "recall"); err != nil {
		log.Printf("update phase recall: %v", err)
	}
	d.store.SetCurrentWord(w.ID, "recall")

	if err := notify.Send(notify.Word{
		ID:   w.ID,
		Text: w.Text,
	}); err != nil {
		log.Printf("notify ERROR: %v", err)
		return err
	}
	log.Print("recall notification sent")
	d.tr.PutLastNotificationTime(time.Now())
	d.tr.RecordNotificationSent()

	return d.waitForReview(w)
}

func (d *Daemon) phaseProduce(w word.Word) error {
	log.Printf("produce: asking for sentence with %q (id=%d)", w.Text, w.ID)
	d.store.SetCurrentWord(w.ID, "produce")

	if err := notify.SendProduction(notify.Word{ID: w.ID, Text: w.Text}); err != nil {
		log.Printf("produce notify ERROR: %v", err)
		d.store.ClearCurrentWord()
		return err
	}
	log.Print("produce notification sent")
	d.tr.PutLastNotificationTime(time.Now())
	d.tr.RecordNotificationSent()

	return d.waitForProduce(w)
}

func (d *Daemon) waitForReview(w word.Word) error {
	log.Printf("waiting for review of word %d (polling 5s, timeout 2h)", w.ID)

	deadline := time.Now().Add(d.devDuration(2 * time.Hour))
	pollInterval := d.devDuration(2 * time.Minute)
	if pollInterval > 5*time.Second {
		pollInterval = 5 * time.Second
	}

	for {
		select {
		case <-d.ctx.Done():
			log.Print("shutdown during review wait")
			d.store.ClearCurrentWord()
			return nil

		case <-time.After(pollInterval):
			d.drainCommand()
			if d.quitPending.Load() {
				log.Print("shutting down during review wait")
				d.store.ClearCurrentWord()
				return errDaemonStop
			}
			if d.learningPaused.Load() {
				log.Print("learning paused during review wait")
				d.store.ClearCurrentWord()
				return errLearningPaused
			}
			if d.learnNowPending.Load() {
				log.Printf("learn now queued; it will run after word %d resolves", w.ID)
			}
			current, err := word.GetWord(d.db, w.ID)
			if err != nil {
				log.Printf("review poll ERROR: %v", err)
				continue
			}

			if current.NextDue > time.Now().Format(word.TimeLayout) {
				log.Printf("word %d reviewed (next_due=%s)", w.ID, current.NextDue)
				d.tr.RecordEngagement()
				d.tr.RecordNotificationAnswered()
				d.store.ClearCurrentWord()
				return nil
			}

			if time.Now().After(deadline) {
				log.Printf("word %d review timeout — marking as missed", w.ID)
				if _, err := scheduler.ScheduleReview(d.db, &w, 0, "missed"); err != nil {
					return fmt.Errorf("auto-lapse: %w", err)
				}
				if err := word.UpdatePhase(d.db, w.ID, ""); err != nil {
					log.Printf("update phase: %v", err)
				}
				d.store.ClearCurrentWord()
				return nil
			}
		}
	}
}

func (d *Daemon) waitForProduce(w word.Word) error {
	log.Printf("waiting for production response (word %d, polling 5s, timeout 30m)", w.ID)

	deadline := time.Now().Add(d.devDuration(30 * time.Minute))
	pollInterval := d.devDuration(2 * time.Minute)
	if pollInterval > 5*time.Second {
		pollInterval = 5 * time.Second
	}

	for {
		select {
		case <-d.ctx.Done():
			log.Print("shutdown during produce wait")
			d.store.ClearCurrentWord()
			return nil

		case <-time.After(pollInterval):
			d.drainCommand()
			if d.quitPending.Load() {
				log.Print("shutting down during produce wait")
				d.store.ClearCurrentWord()
				return errDaemonStop
			}
			if d.learningPaused.Load() {
				log.Print("learning paused during production wait")
				d.store.ClearCurrentWord()
				return errLearningPaused
			}

			if val, ok := d.store.TakeProduceResult(w.ID); ok {
				log.Printf("produce: word %d result=%s", w.ID, val)
				d.store.ClearCurrentWord()
				return nil
			}

			if time.Now().After(deadline) {
				log.Printf("produce: timeout for word %d", w.ID)
				d.store.ClearCurrentWord()
				return nil
			}
		}
	}
}

// resumeAmbientSession re-enters the in-flight word session left behind by a
// previous run, matching the phase that was recorded.
func (d *Daemon) resumeAmbientSession() {
	wordID, phase := d.store.CurrentWord()
	if wordID == 0 {
		return
	}

	log.Printf("resume: word=%d phase=%s", wordID, phase)

	w, err := word.GetWord(d.db, wordID)
	if err != nil {
		log.Printf("resume: word %d not found: %v", wordID, err)
		d.store.ClearCurrentWord()
		return
	}
	if err := d.hydrateWord(w); err != nil {
		log.Printf("resume: load lexical content for %s: %v", w.LexemeID, err)
		d.store.ClearCurrentWord()
		return
	}

	switch phase {
	case "expose":
		log.Printf("resume: re-entering recall phase for word %q", w.Text)
		if err := d.phaseRecall(*w); err != nil {
			log.Printf("resume recall ERROR: %v", err)
		}
	case "recall":
		log.Printf("resume: continuing review wait for word %q", w.Text)
		if err := d.waitForReview(*w); err != nil {
			log.Printf("resume review ERROR: %v", err)
		}
	case "produce":
		log.Printf("resume: continuing produce wait for word %q", w.Text)
		if err := d.waitForProduce(*w); err != nil {
			log.Printf("resume produce ERROR: %v", err)
		}
	}
}

// shouldProduce decides whether to ask the learner to produce a sentence with
// the word, based on review history, stability, and randomness.
func (d *Daemon) shouldProduce(w word.Word) bool {
	updated, err := word.GetWord(d.db, w.ID)
	if err != nil {
		return false
	}
	if updated.ReviewCount < 2 {
		return false
	}
	now := time.Now()
	elapsed := 0.0
	if updated.LastReviewed != "" {
		t, err := time.Parse(word.TimeLayout, updated.LastReviewed)
		if err == nil {
			elapsed = now.Sub(t).Hours()
		}
	}
	if scheduler.RecallProbability(updated.Stability, elapsed) < 0.75 {
		return false
	}
	return rand.Float64() < 0.15
}

// hydrateWord copies the presentation fields from the embedded lexicon into w.
func (d *Daemon) hydrateWord(w *word.Word) error {
	entry, ok := d.list.Lookup(w.LexemeID)
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
