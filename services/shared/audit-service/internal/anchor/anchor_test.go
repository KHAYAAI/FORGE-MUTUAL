package anchor

import (
	"context"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/khayaai/forge-mutual/services/shared/audit-service/internal/chain"
	"github.com/khayaai/forge-mutual/services/shared/audit-service/internal/store"
)

func openTest(t *testing.T) *store.Store {
	t.Helper()
	dsn := os.Getenv("AUDIT_TEST_DSN")
	if dsn == "" {
		t.Skip("AUDIT_TEST_DSN not set")
	}
	ctx := context.Background()
	s, err := store.Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	pool := s.Pool()
	_, _ = pool.Exec(ctx, `alter table audit_entry disable trigger audit_entry_no_update`)
	if _, err := pool.Exec(ctx, `delete from audit_entry`); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `alter table audit_entry enable trigger audit_entry_no_update`); err != nil {
		t.Fatal(err)
	}
	return s
}

func in(action string) chain.Input {
	return chain.Input{Actor: "svc-policy", Action: action, Entity: "BANK", Payload: []byte(`{"allow":true}`)}
}

// The key property: someone who rewrites history and reseals every hash consistently passes the internal check, and
// fails against the anchor written before the rewrite.
func TestFullRewriteIsCaughtByAnAnchorTakenBeforeIt(t *testing.T) {
	ctx := context.Background()
	s := openTest(t)
	for _, a := range []string{"a", "b", "c", "d"} {
		if _, err := s.Append(ctx, in(a)); err != nil {
			t.Fatal(err)
		}
	}
	path := filepath.Join(t.TempDir(), "anchors.jsonl")
	an := &Anchorer{Store: s, Sink: FileSink{Path: path}, Now: time.Now}
	if _, wrote, err := an.Once(ctx); err != nil || !wrote {
		t.Fatalf("anchor: wrote=%v err=%v", wrote, err)
	}
	anchors, err := Read(path)
	if err != nil || len(anchors) != 1 {
		t.Fatalf("read anchors: %v", err)
	}
	if err := Check(ctx, s, anchors); err != nil {
		t.Fatalf("an honest chain must pass its anchors: %v", err)
	}

	// Rewrite seq 2 and reseal 2..4 so the internal chain verifies.
	entries, err := s.Range(ctx, "", 1, 10)
	if err != nil {
		t.Fatal(err)
	}
	entries[1].Action = "rewritten"
	prev := entries[0].Hash
	for i := 1; i < len(entries); i++ {
		e := &entries[i]
		e.PrevHash = prev
		chain.Seal(e)
		prev = e.Hash
	}
	pool := s.Pool()
	if _, err := pool.Exec(ctx, `alter table audit_entry disable trigger audit_entry_no_update`); err != nil {
		t.Fatal(err)
	}
	for _, e := range entries[1:] {
		if _, err := pool.Exec(ctx, `update audit_entry set action = $2, prev_hash = $3, hash = $4, payload_hash = $5 where seq = $1`,
			int64(e.Seq), e.Action, e.PrevHash[:], e.Hash[:], e.PayloadHash[:]); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := pool.Exec(ctx, `alter table audit_entry enable trigger audit_entry_no_update`); err != nil {
		t.Fatal(err)
	}

	if _, _, err := s.VerifyAll(ctx); err != nil {
		t.Fatalf("the resealed chain should pass the internal check, which is the point of this test: %v", err)
	}
	err = Check(ctx, s, anchors)
	if err == nil || !strings.Contains(err.Error(), "rewritten") {
		t.Fatalf("the anchor must catch the rewrite: %v", err)
	}
}

// An anchor past the head means the log was truncated. That is an error, not a silent pass.
func TestAnchorPastTheHeadIsAnError(t *testing.T) {
	ctx := context.Background()
	s := openTest(t)
	_, _ = s.Append(ctx, in("a"))
	bogus := []Anchor{{Seq: 99, Hash: hex.EncodeToString(make([]byte, 32)), At: time.Now()}}
	if err := Check(ctx, s, bogus); err == nil {
		t.Fatal("an anchor beyond the log must fail")
	}
}

// Nothing is written when the head has not moved.
func TestAnchorerOnlyWritesNewHeads(t *testing.T) {
	ctx := context.Background()
	s := openTest(t)
	_, _ = s.Append(ctx, in("a"))
	path := filepath.Join(t.TempDir(), "a.jsonl")
	an := &Anchorer{Store: s, Sink: FileSink{Path: path}}
	if _, wrote, _ := an.Once(ctx); !wrote {
		t.Fatal("first head must be anchored")
	}
	if _, wrote, _ := an.Once(ctx); wrote {
		t.Fatal("an unchanged head must not be anchored again")
	}
	_, _ = s.Append(ctx, in("b"))
	if _, wrote, _ := an.Once(ctx); !wrote {
		t.Fatal("a new head must be anchored")
	}
}
