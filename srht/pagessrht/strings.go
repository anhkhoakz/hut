package pagessrht

import (
	"fmt"
	"strings"
)

func ParseProtocol(s string) (Protocol, error) {
	switch strings.ToLower(s) {
	case "https":
		return ProtocolHttps, nil
	case "gemini":
		return ProtocolGemini, nil
	default:
		return "", fmt.Errorf("invalid protocol: %s", s)
	}
}

func ParseEvents(events []string) ([]WebhookEvent, error) {
	var whEvents []WebhookEvent
	for _, event := range events {
		switch strings.ToLower(event) {
		case "site_published":
			whEvents = append(whEvents, WebhookEventSitePublished)
		case "site_unpublished":
			whEvents = append(whEvents, WebhookEventSiteUnpublished)
		default:
			return whEvents, fmt.Errorf("invalid event: %q", event)
		}
	}

	return whEvents, nil
}
