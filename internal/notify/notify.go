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

// SetActivationCallback receives the argument string from a toast action while
// the process that displayed the toast is still running.
func SetActivationCallback(callback func(arguments string)) {
	setActivationCallback(callback)
}

// SendStatus confirms a user-requested local action without collecting or
// transmitting any data.
func SendStatus(message string) error {
	return sendStatus(message)
}
