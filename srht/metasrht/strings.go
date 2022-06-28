package metasrht

import (
	"fmt"
	"strings"
)

func ParseUserEvents(events []string) ([]WebhookEvent, error) {
	var whEvents []WebhookEvent
	for _, event := range events {
		switch strings.ToLower(event) {
		case "profile_update":
			whEvents = append(whEvents, WebhookEventProfileUpdate)
		case "pgp_key_added":
			whEvents = append(whEvents, WebhookEventPgpKeyAdded)
		case "pgp_key_removed":
			whEvents = append(whEvents, WebhookEventPgpKeyRemoved)
		case "ssh_key_added":
			whEvents = append(whEvents, WebhookEventSshKeyAdded)
		case "ssh_key_removed":
			whEvents = append(whEvents, WebhookEventSshKeyRemoved)
		default:
			return whEvents, fmt.Errorf("invalid event: %q", event)
		}
	}

	return whEvents, nil
}
