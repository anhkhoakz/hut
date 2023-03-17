package listssrht

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

func (status PatchsetStatus) TermString() string {
	var style termfmt.Style

	switch status {
	case PatchsetStatusUnknown:
	case PatchsetStatusProposed:
		style = termfmt.Blue
	case PatchsetStatusNeedsRevision:
		style = termfmt.Yellow
		status = "needs revision"
	case PatchsetStatusSuperseded:
		style = termfmt.Dim
	case PatchsetStatusApproved:
		style = termfmt.Green
	case PatchsetStatusRejected:
		style = termfmt.Red
	case PatchsetStatusApplied:
		style = termfmt.Bold
	default:
		panic(fmt.Sprintf("unknown status: %q", status))
	}

	return style.String(strings.ToLower(string(status)))
}

func ParsePatchsetStatus(s string) (PatchsetStatus, error) {
	switch strings.ToLower(s) {
	case "unknown":
		return PatchsetStatusUnknown, nil
	case "proposed":
		return PatchsetStatusProposed, nil
	case "needs_revision":
		return PatchsetStatusNeedsRevision, nil
	case "superseded":
		return PatchsetStatusSuperseded, nil
	case "approved":
		return PatchsetStatusApproved, nil
	case "rejected":
		return PatchsetStatusRejected, nil
	case "applied":
		return PatchsetStatusApplied, nil
	default:
		return "", fmt.Errorf("invalid patchset status: %s", s)
	}
}

func (acl GeneralACL) TermString() string {
	return fmt.Sprintf("%s browse  %s reply  %s post  %s moderate",
		PermissionIcon(acl.Browse), PermissionIcon(acl.Reply), PermissionIcon(acl.Post), PermissionIcon(acl.Moderate))
}

func PermissionIcon(permission bool) string {
	if permission {
		return termfmt.Green.Sprint("✔")
	}
	return termfmt.Red.Sprint("✗")
}

func ParseUserEvents(events []string) ([]WebhookEvent, error) {
	var whEvents []WebhookEvent
	for _, event := range events {
		switch strings.ToLower(event) {
		case "list_created":
			whEvents = append(whEvents, WebhookEventListCreated)
		case "list_updated":
			whEvents = append(whEvents, WebhookEventListUpdated)
		case "list_deleted":
			whEvents = append(whEvents, WebhookEventListDeleted)
		case "email_received":
			whEvents = append(whEvents, WebhookEventEmailReceived)
		case "patchset_received":
			whEvents = append(whEvents, WebhookEventPatchsetReceived)
		default:
			return whEvents, fmt.Errorf("invalid event: %q", event)
		}
	}

	return whEvents, nil
}

func ParseMailingListWebhookEvents(events []string) ([]WebhookEvent, error) {
	var whEvents []WebhookEvent
	for _, event := range events {
		switch strings.ToLower(event) {
		case "list_updated":
			whEvents = append(whEvents, WebhookEventListUpdated)
		case "list_deleted":
			whEvents = append(whEvents, WebhookEventListDeleted)
		case "email_received":
			whEvents = append(whEvents, WebhookEventEmailReceived)
		case "patchset_received":
			whEvents = append(whEvents, WebhookEventPatchsetReceived)
		default:
			return whEvents, fmt.Errorf("invalid event: %q", event)
		}
	}

	return whEvents, nil
}
