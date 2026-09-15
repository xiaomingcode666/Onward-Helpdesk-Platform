// Package logprivacy minimizes untrusted data before it reaches operational logs.
package logprivacy

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/textproto"
	"strings"
)

const Redacted = "[redacted]"

// Value deliberately omits the entire value. A blacklist cannot safely identify
// arbitrary names, addresses, encoded credentials or free-form message bodies.
func Value(value string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	return Redacted
}

// Error retains only a bounded, typed failure category, never remote error text.
func Error(err error) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, context.Canceled) {
		return "operation cancelled"
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "operation timed out"
	}
	var network net.Error
	if errors.As(err, &network) && network.Timeout() {
		return "network timeout"
	}
	var smtp *textproto.Error
	if errors.As(err, &smtp) && smtp.Code >= 400 && smtp.Code <= 599 {
		return fmt.Sprintf("SMTP %d (details redacted)", smtp.Code)
	}
	return "operation failed (details redacted)"
}
