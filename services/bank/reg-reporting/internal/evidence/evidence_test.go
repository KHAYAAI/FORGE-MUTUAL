package evidence

import (
	"errors"
	"testing"
	"time"
)

var at = time.Date(2026, 10, 31, 10, 0, 0, 0, time.UTC)

func pack(t *testing.T) Pack {
	t.Helper()
	inputs := []Input{{Name: "payments", Digest: "aa", Count: 12}, {Name: "accounts", Digest: "bb", Count: 40}}
	p, err := New("run-1", "cross-border-summary", "2026-10", "abc123", "2026.10.29-1", "analyst-1", inputs, []string{"select 1"}, at)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// The same inputs give the same bytes, regardless of the order they were supplied in.
func TestDigestIsStableAcrossInputOrder(t *testing.T) {
	a := pack(t)
	b, _ := New("run-1", "cross-border-summary", "2026-10", "abc123", "2026.10.29-1", "analyst-1",
		[]Input{{Name: "accounts", Digest: "bb", Count: 40}, {Name: "payments", Digest: "aa", Count: 12}},
		[]string{"select 1"}, at)
	da, _ := a.Digest()
	db, _ := b.Digest()
	if da != db {
		t.Fatal("input order must not change the digest")
	}
}

// Changing any input changes the digest.
func TestDigestCoversEveryField(t *testing.T) {
	a := pack(t)
	base, _ := a.Digest()
	a.Inputs[0].Count++
	if d, _ := a.Digest(); d == base {
		t.Fatal("an input count change must change the digest")
	}
}

// The preparer cannot approve their own pack.
func TestPreparerCannotApprove(t *testing.T) {
	p := pack(t)
	if err := p.Approve("analyst-1", at); !errors.Is(err, ErrSameApprover) {
		t.Fatalf("err = %v, want ErrSameApprover", err)
	}
	if err := p.Approve("officer-2", at); err != nil {
		t.Fatalf("a different approver: %v", err)
	}
}

// A stored pack that has been changed no longer matches its digest.
func TestAlteredPackFailsVerification(t *testing.T) {
	p := pack(t)
	_ = p.Approve("officer-2", at)
	b, _ := p.Canonical()
	d, _ := p.Digest()
	if _, err := Verify(b, d, true); err != nil {
		t.Fatalf("intact pack: %v", err)
	}
	tampered := []byte(string(b) + " ")
	if _, err := Verify(tampered, d, true); !errors.Is(err, ErrAltered) {
		t.Fatalf("tampered pack: %v", err)
	}
}

// A pack without approval is refused when approval is required.
func TestUnapprovedPackIsRefusedWhenApprovalIsRequired(t *testing.T) {
	p := pack(t)
	b, _ := p.Canonical()
	d, _ := p.Digest()
	if _, err := Verify(b, d, true); !errors.Is(err, ErrNotApproved) {
		t.Fatalf("unapproved: %v", err)
	}
	if _, err := Verify(b, d, false); err != nil {
		t.Fatalf("unapproved is allowed for drafts: %v", err)
	}
}
