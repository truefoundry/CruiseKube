package redaction

import "strings"

const Marker = "[REDACTED]"

func String(message, secret string) string {
	if strings.TrimSpace(secret) == "" {
		return message
	}
	return strings.ReplaceAll(message, secret, Marker)
}
