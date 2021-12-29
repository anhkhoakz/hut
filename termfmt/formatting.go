package termfmt

import (
	"fmt"
	"os"

	"golang.org/x/term"
)

var isTerminal = term.IsTerminal(int(os.Stdout.Fd()))

type Color string

const (
	Red    Color = "red"
	Green  Color = "green"
	Yellow Color = "yellow"
	Blue   Color = "blue"

	DarkYellow Color = "dark-yellow"
)

func String(s string, color Color) string {
	if !isTerminal {
		return s
	}

	switch color {
	case Red:
		return fmt.Sprintf("\033[91m%s\033[0m", s)
	case Green:
		return fmt.Sprintf("\033[92m%s\033[0m", s)
	case Yellow:
		return fmt.Sprintf("\033[93m%s\033[0m", s)
	case Blue:
		return fmt.Sprintf("\033[94m%s\033[0m", s)
	case DarkYellow:
		return fmt.Sprintf("\033[33m%s\033[0m", s)
	}

	return s
}
