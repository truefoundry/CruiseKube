package redaction

import (
	"fmt"
	"strings"
)

const Marker = "[REDACTED]"

func String(message, secret string) string {
	if strings.TrimSpace(secret) == "" {
		return message
	}
	return strings.ReplaceAll(message, secret, Marker)
}

func Error(err error, secret string) error {
	if err == nil {
		return nil
	}
	redacted := String(err.Error(), secret)
	if redacted == err.Error() {
		return err
	}
	return fmt.Errorf("%s", redacted)
}
