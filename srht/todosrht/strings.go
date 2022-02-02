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
