package scheduler

import (
	"database/sql"
	"time"
)

var boxIntervals = []int{1, 2, 4, 8, 16, 32}

func BoxInterval(box int) int {
	if box < 0 {
		return 1
	}
	if box >= len(boxIntervals) {
		return boxIntervals[len(boxIntervals)-1]
	}
	return boxIntervals[box]
}

func NextDue(box int) string {
	return time.Now().AddDate(0, 0, BoxInterval(box)).Format("2006-01-02")
}

func RecordFeedback(db *sql.DB, wordID int64, knewIt bool) error {
	box := 0
	if knewIt {
		var currentBox int
		if err := db.QueryRow(`SELECT box FROM words WHERE id = ?`, wordID).Scan(&currentBox); err != nil {
			return err
		}
		box = currentBox + 1
		if box > 5 {
			box = 5
		}
	}
	nextDue := NextDue(box)
	_, err := db.Exec(`UPDATE words SET box = ?, next_due = ? WHERE id = ?`, box, nextDue, wordID)
	return err
}
