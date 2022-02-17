package todosrht

import (
	"fmt"
	"strings"

	"git.sr.ht/~emersion/hut/termfmt"
)

func (visibility Visibility) TermString() string {
	var style termfmt.Style

	switch visibility {
	case VisibilityPublic:
	case VisibilityUnlisted:
		style = termfmt.Blue
	case VisibilityPrivate:
		style = termfmt.Red
	default:
		panic(fmt.Sprintf("unknown visibility: %q", visibility))
	}

	return style.String(strings.ToLower(string(visibility)))
}

func (status TicketStatus) TermString() string {
	var style termfmt.Style
	var s string

	switch status {
	case TicketStatusReported, TicketStatusConfirmed, TicketStatusInProgress, TicketStatusPending:
		s = "open"
		style = termfmt.Red
	case TicketStatusResolved:
		s = "closed"
		style = termfmt.Green
	default:
		panic(fmt.Sprintf("unknown status: %q", status))
	}

	return style.String(s)
}

func (label Label) TermString() string {
	return termfmt.HexString(label.Name, label.ForegroundColor, label.BackgroundColor)
}

func ParseTicketStatus(s string) (TicketStatus, error) {
	switch strings.ToLower(s) {
	case "reported":
		return TicketStatusReported, nil
	case "confirmed":
		return TicketStatusConfirmed, nil
	case "in_progress":
		return TicketStatusInProgress, nil
	case "pending":
		return TicketStatusPending, nil
	case "resolved":
		return TicketStatusResolved, nil
	default:
		return "", fmt.Errorf("invalid ticket status: %s", s)
	}
}

func ParseTicketResolution(s string) (TicketResolution, error) {
	switch strings.ToLower(s) {
	case "unresolved":
		return TicketResolutionUnresolved, nil
	case "fixed":
		return TicketResolutionFixed, nil
	case "implemented":
		return TicketResolutionImplemented, nil
	case "wont_fix":
		return TicketResolutionWontFix, nil
	case "by_design":
		return TicketResolutionByDesign, nil
	case "invalid":
		return TicketResolutionInvalid, nil
	case "duplicate":
		return TicketResolutionDuplicate, nil
	case "not_out_bug":
		return TicketResolutionNotOurBug, nil
	default:
		return "", fmt.Errorf("invalid ticket resolution: %s", s)
	}
}
