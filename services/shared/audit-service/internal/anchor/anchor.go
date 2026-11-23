// Package anchor records the chain head outside the database. A full rewrite of the log, resealed consistently by
// someone with database access, still passes the internal check. It fails against an anchor written earlier, because
// the anchored hash no longer matches the chain at that sequence number.
package anchor

import (
	"bufio"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/khayaai/forge-mutual/services/shared/audit-service/internal/store"
)

// Anchor is a head hash, written outside the database.
type Anchor struct {
	Seq  uint64    `json:"seq"`
	Hash string    `json:"hash"` // hex
	At   time.Time `json:"at"`
}

// Sink receives anchors. Production must use write-once storage with object lock (VERIFY-WORM). FileSink is for
// development only.
type Sink interface {
	Write(ctx context.Context, a Anchor) error
}

// FileSink appends anchors as JSON lines to a local file. It gives no protection against someone who can rewrite the
// file, so it is for development and tests only.
type FileSink struct {
	Path string
}

// Write implements Sink.
func (f FileSink) Write(_ context.Context, a Anchor) error {
	fh, err := os.OpenFile(f.Path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("anchor: open: %w", err)
	}
	defer fh.Close()
	b, err := json.Marshal(a)
	if err != nil {
		return err
	}
	if _, err := fh.Write(append(b, '\n')); err != nil {
		return fmt.Errorf("anchor: write: %w", err)
	}
	return fh.Sync()
}

// Read returns every anchor in a FileSink file, in order.
func Read(path string) ([]Anchor, error) {
	fh, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer fh.Close()
	var out []Anchor
	sc := bufio.NewScanner(fh)
	for sc.Scan() {
		var a Anchor
		if err := json.Unmarshal(sc.Bytes(), &a); err != nil {
			return nil, fmt.Errorf("anchor: line %d: %w", len(out)+1, err)
		}
		out = append(out, a)
	}
	return out, sc.Err()
}

// Anchorer writes the head whenever it has moved since the last anchor.
type Anchorer struct {
	Store *store.Store
	Sink  Sink
	Now   func() time.Time
	last  uint64
}

// Once anchors the current head if it is new. It reports whether it wrote one.
func (a *Anchorer) Once(ctx context.Context) (Anchor, bool, error) {
	seq, hash, err := a.Store.Head(ctx)
	if err != nil {
		return Anchor{}, false, err
	}
	if seq == 0 || seq == a.last {
		return Anchor{}, false, nil
	}
	now := time.Now
	if a.Now != nil {
		now = a.Now
	}
	an := Anchor{Seq: seq, Hash: hex.EncodeToString(hash[:]), At: now().UTC()}
	if err := a.Sink.Write(ctx, an); err != nil {
		return Anchor{}, false, err
	}
	a.last = seq
	return an, true, nil
}

// Check confirms every anchor against the chain. An anchor past the current head is an error: the log has been
// truncated since the anchor was written.
func Check(ctx context.Context, st *store.Store, anchors []Anchor) error {
	for _, an := range anchors {
		want, err := hex.DecodeString(an.Hash)
		if err != nil || len(want) != 32 {
			return fmt.Errorf("anchor %d: hash is not 32 bytes of hex", an.Seq)
		}
		got, err := st.HashAt(ctx, an.Seq)
		if errors.Is(err, store.ErrNotFound) {
			return fmt.Errorf("anchor %d: no entry at that sequence number; the log was truncated", an.Seq)
		}
		if err != nil {
			return err
		}
		if hex.EncodeToString(got[:]) != an.Hash {
			return fmt.Errorf("anchor %d: the chain no longer matches the anchored hash; history was rewritten", an.Seq)
		}
	}
	return nil
}
