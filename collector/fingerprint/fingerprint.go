package fingerprint

import (
	"crypto/sha256"
	"fmt"
	"regexp"
	"strings"
)

// Generate creates a fingerprint from error stack and message
func Generate(stack, message string) string {
	if stack != "" {
		return stackFingerprint(stack)
	}
	return messageFingerprint(message)
}

// stackFingerprint extracts function+file from each frame, ignoring line numbers
func stackFingerprint(stack string) string {
	// Match "at functionName (file:line:col)" or "at file:line:col"
	re := regexp.MustCompile(`at\s+(?:\S+\s+)?\(?(.+?):\d+:\d+\)?`)
	matches := re.FindAllStringSubmatch(stack, -1)
	var frames []string
	for _, m := range matches {
		if len(m) > 1 {
			frames = append(frames, m[1])
		}
	}
	if len(frames) == 0 {
		return messageFingerprint(stack)
	}
	h := sha256.Sum256([]byte(strings.Join(frames, "|")))
	return fmt.Sprintf("%x", h[:16]) // First 16 bytes, 32 hex characters
}

// messageFingerprint normalizes dynamic values then hashes
func messageFingerprint(message string) string {
	normalized := normalizeMessage(message)
	h := sha256.Sum256([]byte(normalized))
	return fmt.Sprintf("%x", h[:16])
}

// normalizeMessage replaces dynamic values with placeholders
func normalizeMessage(msg string) string {
	// UUID: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
	re := regexp.MustCompile(`[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`)
	msg = re.ReplaceAllString(msg, "{uuid}")

	// Hex: 0x...
	re = regexp.MustCompile(`0x[0-9a-fA-F]+`)
	msg = re.ReplaceAllString(msg, "{hex}")

	// Numbers (but not in obvious version strings like "1.0.0")
	re = regexp.MustCompile(`\b\d{3,}\b`)
	msg = re.ReplaceAllString(msg, "{n}")

	// URLs with dynamic paths
	re = regexp.MustCompile(`/\d+/`)
	msg = re.ReplaceAllString(msg, "/{id}/")

	return strings.TrimSpace(msg)
}
