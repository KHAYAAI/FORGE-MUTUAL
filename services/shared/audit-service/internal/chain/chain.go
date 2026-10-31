// Package chain holds the hash chain of the audit log. It is pure: it computes and verifies hashes and does no I/O.
//
// Each entry commits to the previous entry's hash, so changing or removing any entry breaks every later hash. The
// encoding is length-prefixed, so no two different entries can encode to the same bytes.
package chain

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"time"
	"unicode/utf8"
)

// MaxPayload bounds one entry. Audit payloads are event descriptions, not documents.
const MaxPayload = 64 << 10

// Genesis is the previous hash of the first entry.
var Genesis [32]byte

// Entry is one link in the chain.
type Entry struct {
	Seq         uint64
	At          time.Time // stored at microsecond precision, the precision that is hashed
	Actor       string
	Action      string
	Entity      string
	Payload     []byte
	PayloadHash [32]byte
	PrevHash    [32]byte
	Hash        [32]byte
}

// Input is what a writer provides. The store assigns Seq, At and the hashes.
type Input struct {
	Actor   string
	Action  string
	Entity  string
	Payload []byte
}

// ErrInvalidInput marks an input the log refuses. It is the caller's fault, not the store's.
var ErrInvalidInput = errors.New("chain: invalid input")

// Validate refuses inputs that cannot be recorded faithfully.
func (in Input) Validate() error {
	for name, v := range map[string]string{"actor": in.Actor, "action": in.Action, "entity": in.Entity} {
		if v == "" {
			return fmt.Errorf("%w: %s is required", ErrInvalidInput, name)
		}
		if len(v) > 200 || !utf8.ValidString(v) {
			return fmt.Errorf("%w: %s must be valid UTF-8 of at most 200 bytes", ErrInvalidInput, name)
		}
	}
	if len(in.Payload) == 0 || len(in.Payload) > MaxPayload {
		return fmt.Errorf("%w: payload must be 1 to %d bytes", ErrInvalidInput, MaxPayload)
	}
	return nil
}

// PayloadHash hashes the exact payload bytes.
func PayloadHash(payload []byte) [32]byte { return sha256.Sum256(payload) }

// Micros truncates a time to the precision that is stored and hashed.
func Micros(t time.Time) time.Time { return time.UnixMicro(t.UTC().UnixMicro()).UTC() }

// Hash computes an entry's hash from its fields and its predecessor's hash.
func Hash(prev [32]byte, seq uint64, at time.Time, actor, action, entity string, payloadHash [32]byte) [32]byte {
	var buf bytes.Buffer
	buf.Write(prev[:])
	_ = binary.Write(&buf, binary.BigEndian, seq)
	_ = binary.Write(&buf, binary.BigEndian, at.UTC().UnixMicro())
	for _, s := range []string{actor, action, entity} {
		_ = binary.Write(&buf, binary.BigEndian, uint32(len(s)))
		buf.WriteString(s)
	}
	buf.Write(payloadHash[:])
	return sha256.Sum256(buf.Bytes())
}

// Seal fills in the hashes of an entry whose fields and PrevHash are set.
func Seal(e *Entry) {
	e.PayloadHash = PayloadHash(e.Payload)
	e.Hash = Hash(e.PrevHash, e.Seq, e.At, e.Actor, e.Action, e.Entity, e.PayloadHash)
}

// ErrBroken is returned, wrapped, by Verify. The wrapped message names the first sequence number that fails.
var ErrBroken = errors.New("chain: broken")

// Verify checks a contiguous run of entries. The first entry must have PrevHash equal to the expected previous hash,
// and every entry must recompute to its stored hashes. It returns the hash of the last entry.
func Verify(prev [32]byte, startSeq uint64, entries []Entry) ([32]byte, error) {
	seq := startSeq
	for i := range entries {
		e := entries[i]
		if e.Seq != seq {
			return prev, fmt.Errorf("%w at seq %d: expected seq %d", ErrBroken, e.Seq, seq)
		}
		if e.PrevHash != prev {
			return prev, fmt.Errorf("%w at seq %d: previous hash does not link", ErrBroken, e.Seq)
		}
		if PayloadHash(e.Payload) != e.PayloadHash {
			return prev, fmt.Errorf("%w at seq %d: payload was altered", ErrBroken, e.Seq)
		}
		want := Hash(e.PrevHash, e.Seq, e.At, e.Actor, e.Action, e.Entity, e.PayloadHash)
		if want != e.Hash {
			return prev, fmt.Errorf("%w at seq %d: stored hash does not match its fields", ErrBroken, e.Seq)
		}
		prev = e.Hash
		seq++
	}
	return prev, nil
}
