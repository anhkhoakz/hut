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
