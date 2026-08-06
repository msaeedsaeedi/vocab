package notify

// Word is the minimal recall prompt shown in a notification. The recall phase
// must NOT reveal the definition or example — that would defeat the active
// recall test. Those fields only appear on the wallpaper during exposure.
type Word struct {
	ID   int64
	Text string
}

func SendProduction(w Word) error {
	return sendProduction(w)
}
