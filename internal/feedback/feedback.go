package feedback

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
)

var stdin io.Reader = os.Stdin

func Prompt() (bool, error) {
	return readFrom(stdin)
}

func readFrom(r io.Reader) (bool, error) {
	reader := bufio.NewReader(r)
	fmt.Fprint(io.Discard, "Did you know this word? [y/n]: ")
	input, err := reader.ReadString('\n')
	if err != nil {
		return false, fmt.Errorf("read input: %w", err)
	}
	input = strings.TrimSpace(strings.ToLower(input))
	switch input {
	case "y", "yes":
		return true, nil
	case "n", "no":
		return false, nil
	default:
		return false, fmt.Errorf("unrecognized input: %q (answer y or n)", input)
	}
}
