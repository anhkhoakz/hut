package pastesrht

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

func ParseVisibility(s string) (Visibility, error) {
	switch strings.ToLower(s) {
	case "unlisted":
		return VisibilityUnlisted, nil
	case "private":
		return VisibilityPrivate, nil
	case "public":
		return VisibilityPublic, nil
	default:
		return "", fmt.Errorf("invalid visibility: %s", s)
	}
}

func ParseEvents(events []string) ([]WebhookEvent, error) {
	var whEvents []WebhookEvent
	for _, event := range events {
		switch strings.ToLower(event) {
		case "paste_created":
			whEvents = append(whEvents, WebhookEventPasteCreated)
		case "paste_updated":
			whEvents = append(whEvents, WebhookEventPasteUpdated)
		case "paste_deleted":
			whEvents = append(whEvents, WebhookEventPasteDeleted)
		default:
			return whEvents, fmt.Errorf("invalid event: %q", event)
		}
	}

	return whEvents, nil
}
