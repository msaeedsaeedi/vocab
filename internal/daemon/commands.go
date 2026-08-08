package daemon

import (
	"log"
	"os"
	"path/filepath"

	"github.com/msaeedsaeedi/vocab/internal/apppaths"
)

// SendCommand writes a command to the daemon mailbox. It is used by short-lived
// CLI invocations (--learn-now, --quit) to signal a running daemon.
func SendCommand(command string) error {
	path, err := apppaths.CommandPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	temporary := path + ".tmp"
	if err := os.WriteFile(temporary, []byte(command), 0600); err != nil {
		return err
	}
	return os.Rename(temporary, path)
}

// drainCommand consumes and applies any pending mailbox command.
func (d *Daemon) drainCommand() {
	path, err := apppaths.CommandPath()
	if err != nil {
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	if err := os.Remove(path); err != nil {
		log.Printf("daemon command: remove mailbox: %v", err)
	}
	switch string(data) {
	case "learn-now":
		d.requestLearnNow()
	case "quit":
		d.quitPending.Store(true)
		log.Print("quit requested by local command")
	default:
		log.Printf("daemon command: ignored unknown command %q", string(data))
	}
}
