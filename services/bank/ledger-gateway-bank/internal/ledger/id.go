// Package ledger holds the ledger domain: accounts, transfers and a Store with
// TigerBeetle semantics. It has no dependency on the TigerBeetle client, so the
// rules can be tested in-process.
package ledger

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"time"
)

// ID is a 128-bit identifier. Its string form is 32 lowercase hex characters.
type ID [16]byte

// ErrBadID is returned when an ID string is not 32 hex characters.
var ErrBadID = errors.New("ledger: bad id")

// ZeroID is the unset ID.
var ZeroID ID

// NewID returns a time-ordered ID. TigerBeetle reads an ID as a little-endian 128-bit integer,
// so the 48-bit millisecond timestamp goes in the highest-order bytes (ID[10:16], big-endian)
// and 80 random bits fill the low bytes. Later IDs are then larger integers, which lets the
// cluster append in order (TigerBeetle's recommendation for write performance, VERIFY-TB).
func NewID(now time.Time) (ID, error) {
	var id ID
	if _, err := rand.Read(id[:10]); err != nil {
		return ZeroID, fmt.Errorf("ledger: random id: %w", err)
	}
	ms := uint64(now.UnixMilli())
	for i := 0; i < 6; i++ {
		id[15-i] = byte(ms >> (8 * (5 - i)))
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

// toBig returns the ID as TigerBeetle reads it: a little-endian 128-bit integer.
func (id ID) toBig() *big.Int {
	var be [16]byte
	for i := range id {
		be[15-i] = id[i]
	}
	return new(big.Int).SetBytes(be[:])
}
