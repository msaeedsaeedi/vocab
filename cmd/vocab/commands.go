package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"strings"

	"github.com/msaeedsaeedi/vocab/internal/database"
	"github.com/msaeedsaeedi/vocab/internal/engage"
	"github.com/msaeedsaeedi/vocab/internal/scheduler"
	"github.com/msaeedsaeedi/vocab/internal/state"
	"github.com/msaeedsaeedi/vocab/internal/word"
)

func handleReviewCommand(db *database.DB, store *state.Store, id int64, knewDeprecated bool, ratingFlag int) error {
	return handleReview(db, store, id, knewDeprecated, ratingFlag, true)
}

func handleReviewActivation(db *database.DB, store *state.Store, id int64, rating int) error {
	return handleReview(db, store, id, false, rating, false)
}

func handleReview(db *database.DB, store *state.Store, id int64, knewDeprecated bool, ratingFlag int, recordEngagement bool) error {
	r := ratingFlag
	if knewDeprecated && ratingFlag == 0 {
		r = 2
	}
	var outcome string
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
	w, err := word.GetWord(db, id)
	if err != nil {
		return fmt.Errorf("get word: %w", err)
	}

	if _, err := scheduler.ScheduleReview(db, w, r, outcome); err != nil {
		return fmt.Errorf("schedule review: %w", err)
	}

	if recordEngagement {
		tr := engage.New(db, store)
		tr.RecordEngagement()
		tr.RecordNotificationAnswered()
	}
	log.Printf("review: recorded id=%d rating=%d", id, r)
	return nil
}

func handleProduceCommand(store *state.Store, id int64, produced bool) error {
	val := "no"
	if produced {
		val = "yes"
	}
	log.Printf("produce: id=%d produced=%s", id, val)
	if err := store.SetProduceResult(id, produced); err != nil {
		return fmt.Errorf("store produce result: %w", err)
	}
	return nil
}

// dispatchToastActivation parses the arguments attached to a Windows toast
// button and applies the same state transition as the corresponding CLI flag.
// Engagement is deliberately left to waitForReview, which observes the state
// transition and records one answer for the notification it sent.
func dispatchToastActivation(db *database.DB, store *state.Store, arguments string) {
	flags := flag.NewFlagSet("toast activation", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	reviewID := flags.Int64("review", 0, "")
	rating := flags.Int("rating", 0, "")
	produceID := flags.Int64("produce", 0, "")
	produced := flags.Bool("produced", false, "")
	if err := flags.Parse(strings.Fields(arguments)); err != nil {
		log.Printf("toast activation ignored: parse %q: %v", arguments, err)
		return
	}
	switch {
	case *reviewID > 0 && *produceID == 0:
		if err := handleReviewActivation(db, store, *reviewID, *rating); err != nil {
			log.Printf("toast review: %v", err)
		}
	case *produceID > 0 && *reviewID == 0:
		if err := handleProduceCommand(store, *produceID, *produced); err != nil {
			log.Printf("toast produce: %v", err)
		}
	default:
		log.Printf("toast activation ignored: unsupported arguments %q", arguments)
	}
}
