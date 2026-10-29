// Package uuid generates RFC 4122 version 4 UUIDs from crypto/rand.
package uuid

import (
	"crypto/rand"
	"fmt"
	"regexp"
)

// NewV4 returns a random UUID in canonical 8-4-4-4-12 form.
func NewV4() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("uuid: %w", err)
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}

var canonical = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// Parse accepts only the canonical lower-case 8-4-4-4-12 form.
func Parse(s string) (string, error) {
	if !canonical.MatchString(s) {
		return "", fmt.Errorf("uuid: not canonical: %q", s)
	}
	return s, nil
}
