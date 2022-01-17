package listssrht

import (
	"fmt"
	"strings"

	"git.sr.ht/~emersion/hut/termfmt"
)

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
