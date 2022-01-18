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
