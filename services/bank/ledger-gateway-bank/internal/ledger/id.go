// Package ledger holds the ledger domain: accounts, transfers and a Store with
// TigerBeetle semantics. It has no dependency on the TigerBeetle client, so the
// rules can be tested in-process.
package ledger

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
)

// ID is a 128-bit identifier. Its string form is 32 lowercase hex characters.
type ID [16]byte

// ErrBadID is returned when an ID string is not 32 hex characters.
var ErrBadID = errors.New("ledger: bad id")

// ZeroID is the unset ID.
var ZeroID ID

// NewID returns a time-ordered ID: 48 bits of Unix milliseconds followed by 80 random bits.
// TigerBeetle recommends time-ordered IDs for write performance (ARCHITECTURE.md 7.6, VERIFY-TB).
func NewID(now time.Time) (ID, error) {
	var id ID
	ms := uint64(now.UnixMilli())
	for i := 0; i < 6; i++ {
		id[i] = byte(ms >> (8 * (5 - i)))
	}
	if _, err := rand.Read(id[6:]); err != nil {
		return ZeroID, fmt.Errorf("ledger: random id: %w", err)
	}
	return id, nil
}

// ParseID parses a 32-character hex string.
func ParseID(s string) (ID, error) {
	var id ID
	if len(s) != 32 {
		return ZeroID, fmt.Errorf("%w: %q", ErrBadID, s)
	}
	b, err := hex.DecodeString(s)
	if err != nil {
		return ZeroID, fmt.Errorf("%w: %q", ErrBadID, s)
	}
	copy(id[:], b)
	return id, nil
}

// String returns the hex form.
func (id ID) String() string { return hex.EncodeToString(id[:]) }

// IsZero reports whether the ID is unset.
func (id ID) IsZero() bool { return id == ZeroID }
