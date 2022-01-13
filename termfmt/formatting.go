package termfmt

import (
	"fmt"
	"os"

	"golang.org/x/term"
)

var isTerminal = term.IsTerminal(int(os.Stdout.Fd()))

type Style string

const (
	Bold Style = "bold"
	Dim  Style = "dim"

	Red    Style = "red"
	Green  Style = "green"
	Yellow Style = "yellow"
	Blue   Style = "blue"

	DarkYellow Style = "dark-yellow"
)

func (style Style) String(s string) string {
	if !isTerminal {
		return s
	}

	switch style {
	case Bold:
		return fmt.Sprintf("\033[1m%s\033[0m", s)
	case Dim:
		return fmt.Sprintf("\033[2m%s\033[0m", s)
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
	default:
		return s
	}
}

func (style Style) Sprint(args ...interface{}) string {
	return style.String(fmt.Sprint(args...))
}

func (style Style) Sprintf(format string, args ...interface{}) string {
	return style.String(fmt.Sprintf(format, args...))
}
