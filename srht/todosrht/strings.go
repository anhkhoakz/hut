package todosrht

import (
	"fmt"
	"strings"

	"git.sr.ht/~xenrox/hut/termfmt"
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
	return termfmt.HexString(fmt.Sprintf(" %s ", label.Name), label.ForegroundColor, label.BackgroundColor)
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
	case "closed":
		return TicketResolutionClosed, nil
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

func (acl DefaultACL) TermString() string {
	return fmt.Sprintf("%s browse  %s submit  %s comment  %s edit %s triage",
		PermissionIcon(acl.Browse), PermissionIcon(acl.Submit), PermissionIcon(acl.Comment),
		PermissionIcon(acl.Edit), PermissionIcon(acl.Triage))
}

func PermissionIcon(permission bool) string {
	if permission {
		return termfmt.Green.Sprint("✔")
	}
	return termfmt.Red.Sprint("✗")
}

func ParseTicketWebhookEvents(events []string) ([]WebhookEvent, error) {
	var whEvents []WebhookEvent
	for _, event := range events {
		switch strings.ToLower(event) {
		case "ticket_update":
			whEvents = append(whEvents, WebhookEventTicketUpdate)
		case "ticket_deleted":
			whEvents = append(whEvents, WebhookEventTicketDeleted)
		case "event_created":
			whEvents = append(whEvents, WebhookEventEventCreated)
		default:
			return whEvents, fmt.Errorf("invalid event: %q", event)
		}
	}

	return whEvents, nil
}

func ParseUserEvents(events []string) ([]WebhookEvent, error) {
	var whEvents []WebhookEvent
	for _, event := range events {
		switch strings.ToLower(event) {
		case "tracker_created":
			whEvents = append(whEvents, WebhookEventTrackerCreated)
		case "tracker_update":
			whEvents = append(whEvents, WebhookEventTrackerUpdate)
		case "tracker_deleted":
			whEvents = append(whEvents, WebhookEventTrackerDeleted)
		case "ticket_created":
			whEvents = append(whEvents, WebhookEventTicketCreated)
		default:
			return whEvents, fmt.Errorf("invalid event: %q", event)
		}
	}

	return whEvents, nil
}

func ParseTrackerWebhookEvents(events []string) ([]WebhookEvent, error) {
	var whEvents []WebhookEvent
	for _, event := range events {
		switch strings.ToLower(event) {
		case "tracker_update":
			whEvents = append(whEvents, WebhookEventTrackerUpdate)
		case "tracker_deleted":
			whEvents = append(whEvents, WebhookEventTrackerDeleted)
		case "label_created":
			whEvents = append(whEvents, WebhookEventLabelCreated)
		case "label_update":
			whEvents = append(whEvents, WebhookEventLabelUpdate)
		case "label_deleted":
			whEvents = append(whEvents, WebhookEventLabelDeleted)
		case "ticket_created":
			whEvents = append(whEvents, WebhookEventTicketCreated)
		case "ticket_update":
			whEvents = append(whEvents, WebhookEventTicketUpdate)
		case "ticket_deleted":
			whEvents = append(whEvents, WebhookEventTicketDeleted)
		case "event_created":
			whEvents = append(whEvents, WebhookEventEventCreated)
		default:
			return whEvents, fmt.Errorf("invalid event: %q", event)
		}
	}

	return whEvents, nil
}

func (t Ticket) IsOpen() bool {
	return t.Status != TicketStatusResolved
}
