package buildssrht

import "git.sr.ht/~emersion/hut/termfmt"

func (status JobStatus) TermString() string {
	var color termfmt.Style
	switch status {
	case JobStatusSuccess:
		color = termfmt.Green
	case JobStatusFailed:
		color = termfmt.Red
	case JobStatusRunning:
		color = termfmt.Blue
	case JobStatusCancelled:
		color = termfmt.Yellow
	}

	return termfmt.String(string(status), color)
}
