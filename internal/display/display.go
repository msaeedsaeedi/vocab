package display

import (
	"fmt"
	"strings"

	"github.com/msaeed/vocab/internal/word"
)

func Word(e *word.Entry, showMeaning, showUsage bool) {
	printHeader()

	fmt.Printf("  %s\n", e.Word)

	if showMeaning && e.Meaning != "" {
		fmt.Printf("  \033[3m%s\033[0m\n", e.Meaning)
	}

	if showUsage && e.Usage != "" {
		fmt.Printf("  \033[90m\"%s\"\033[0m\n", e.Usage)
	}

	fmt.Println()
	fmt.Println(strings.Repeat("─", 40))
	fmt.Println()
}

func Stats(total, due int) {
	fmt.Printf("  Total: %d  |  Due: %d\n", total, due)
	fmt.Println()
}

func printHeader() {
	fmt.Println()
	fmt.Print("  \033[1m")
	fmt.Print("Vocab")
	fmt.Print("\033[0m")
	fmt.Println()
	fmt.Println(strings.Repeat("─", 40))
	fmt.Println()
}
