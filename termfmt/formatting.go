package termfmt

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"golang.org/x/term"
)

var isTerminal = term.IsTerminal(int(os.Stdout.Fd()))

type Style string

type RGB struct {
	Red, Green, Blue uint8
}

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

	// All formatting strings have to be the same length for tabwriter to work
	switch style {
	case Bold:
		return fmt.Sprintf("\033[01m%s\033[0m", s)
	case Dim:
		return fmt.Sprintf("\033[02m%s\033[0m", s)
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

func HexString(s string, fg string, bg string) string {
	if !isTerminal {
		return s
	}

	return RGBString(s, HexToRGB(fg), HexToRGB(bg))
}

func RGBString(s string, fg, bg RGB) string {
	if !isTerminal {
		return s
	}

	return fmt.Sprintf("\033[38;2;%d;%d;%dm\033[48;2;%d;%d;%dm%s\033[0m",
		fg.Red, fg.Green, fg.Blue, bg.Red, bg.Green, bg.Blue, s)
}

func (style Style) Sprint(args ...interface{}) string {
	return style.String(fmt.Sprint(args...))
}

func (style Style) Sprintf(format string, args ...interface{}) string {
	return style.String(fmt.Sprintf(format, args...))
}

func HexToRGB(hex string) RGB {
	var rgb RGB
	hex = strings.TrimPrefix(hex, "#")
	if len(hex) != 6 {
		log.Fatalf("not a valid hex color %q", hex)
	}

	for i := 0; i < 3; i++ {
		v, err := strconv.ParseUint(hex[i*2:i*2+2], 16, 8)
		if err != nil {
			log.Fatal(err)
		}

		switch i {
		case 0:
			rgb.Red = uint8(v)
		case 1:
			rgb.Green = uint8(v)
		case 2:
			rgb.Blue = uint8(v)
		}
	}
	return rgb
}

func ReplaceLine() string {
	if !isTerminal {
		return "\n"
	}
	return "\x1b[1K\r"
}
