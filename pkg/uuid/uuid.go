// Package uuid generates RFC 4122 version 4 UUIDs from crypto/rand.
package uuid

import (
	"crypto/rand"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
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

// NewV5 returns the RFC 4122 version 5 (SHA-1, name-based) UUID of name in namespace. The same
// namespace and name always give the same UUID, so an idempotency key can name a workflow.
func NewV5(namespace, name string) (string, error) {
	ns, err := Parse(namespace)
	if err != nil {
		return "", err
	}
	raw, err := hex.DecodeString(strings.ReplaceAll(ns, "-", ""))
	if err != nil {
		return "", err
	}
	h := sha1.New()
	h.Write(raw)
	h.Write([]byte(name))
	sum := h.Sum(nil)
	sum[6] = (sum[6] & 0x0f) | 0x50
	sum[8] = (sum[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", sum[0:4], sum[4:6], sum[6:8], sum[8:10], sum[10:16]), nil
}
