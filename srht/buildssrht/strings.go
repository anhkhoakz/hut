package buildssrht

import (
	"fmt"
	"strings"

	"git.sr.ht/~emersion/hut/termfmt"
)

func (status JobStatus) Icon() string {
	switch status {
	case JobStatusPending, JobStatusQueued:
		return "○"
	case JobStatusRunning:
		return "●"
	case JobStatusSuccess:
		return "✔"
	case JobStatusFailed:
		return "✗"
	case JobStatusTimeout:
		return "⏱️"
	case JobStatusCancelled:
		return "🛑"
	default:
		panic(fmt.Sprintf("unknown job status: %q", status))
	}
}

func (status JobStatus) TermStyle() termfmt.Style {
	switch status {
	case JobStatusPending, JobStatusQueued, JobStatusRunning:
		return termfmt.Blue
	case JobStatusSuccess:
		return termfmt.Green
	case JobStatusFailed, JobStatusTimeout:
		return termfmt.Red
	case JobStatusCancelled:
		return termfmt.Yellow
	default:
		panic(fmt.Sprintf("unknown job status: %q", status))
	}
}

func (status JobStatus) TermIcon() string {
	return status.TermStyle().String(status.Icon())
}

func (status JobStatus) TermString() string {
	return status.TermStyle().Sprintf("%s %s", status.Icon(), string(status))
}

func (status TaskStatus) Icon() string {
	switch status {
	case TaskStatusPending:
		return "○"
	case TaskStatusRunning:
		return "●"
	case TaskStatusSuccess:
		return "✔"
	case TaskStatusFailed:
		return "✗"
	case TaskStatusSkipped:
		return "⏩"
	default:
		panic(fmt.Sprintf("unknown task status: %q", status))
	}
}

func (status TaskStatus) TermStyle() termfmt.Style {
	switch status {
	case TaskStatusPending, TaskStatusRunning:
		return termfmt.Blue
	case TaskStatusSuccess:
		return termfmt.Green
	case TaskStatusFailed:
		return termfmt.Red
	case TaskStatusSkipped:
		return termfmt.Yellow
	default:
		panic(fmt.Sprintf("unknown task status: %q", status))
	}
}

func (status TaskStatus) TermIcon() string {
	return status.TermStyle().String(status.Icon())
}

func ParseUserEvents(events []string) ([]WebhookEvent, error) {
	var whEvents []WebhookEvent
	for _, event := range events {
		switch strings.ToLower(event) {
		case "job_created":
			whEvents = append(whEvents, WebhookEventJobCreated)
		default:
			return whEvents, fmt.Errorf("invalid event: %q", event)
		}
	}

	return whEvents, nil
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
