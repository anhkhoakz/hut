package gitsrht

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

func ParseAccessMode(s string) (AccessMode, error) {
	switch strings.ToLower(s) {
	case "ro":
		return AccessModeRo, nil
	case "rw":
		return AccessModeRw, nil
	default:
		return "", fmt.Errorf("invalid access mode: %s", s)
	}
}

func ParseEvents(events []string) ([]WebhookEvent, error) {
	var whEvents []WebhookEvent
	for _, event := range events {
		switch strings.ToLower(event) {
		case "repo_created":
			whEvents = append(whEvents, WebhookEventRepoCreated)
		case "repo_update":
			whEvents = append(whEvents, WebhookEventRepoUpdate)
		case "repo_deleted":
			whEvents = append(whEvents, WebhookEventRepoDeleted)
		default:
			return whEvents, fmt.Errorf("invalid event: %q", event)
		}
	}

	return whEvents, nil
}

func ParseGitWebhookEvents(events []string) ([]WebhookEvent, error) {
	var whEvents []WebhookEvent
	for _, event := range events {
		switch strings.ToLower(event) {
		case "git_pre_receive":
			whEvents = append(whEvents, WebhookEventGitPreReceive)
		case "git_post_receive":
			whEvents = append(whEvents, WebhookEventGitPostReceive)
		default:
			return whEvents, fmt.Errorf("invalid event: %q", event)
		}
	}

	return whEvents, nil
}
