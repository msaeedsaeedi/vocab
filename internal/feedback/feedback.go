package feedback

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

type Result int

const (
	Unknown Result = iota
	Known
	Close
)

func Prompt() (Result, error) {
	reader := bufio.NewReader(os.Stdin)

	fmt.Print("Did you know this word? [y]es / [n]o / [c]lose: ")
	input, err := reader.ReadString('\n')
	if err != nil {
		return Unknown, fmt.Errorf("read input: %w", err)
	}

	input = strings.TrimSpace(strings.ToLower(input))

	switch input {
	case "y", "yes", "yeah":
		return Known, nil
	case "n", "no", "nah":
		return Unknown, nil
	case "c", "close", "kinda":
		return Close, nil
	default:
		return Unknown, fmt.Errorf("unrecognized input: %s", input)
	}
}

func MapToQuality(r Result) int {
	switch r {
	case Known:
		return 5
	case Close:
		return 3
	case Unknown:
		return 0
	default:
		return 0
	}
}
