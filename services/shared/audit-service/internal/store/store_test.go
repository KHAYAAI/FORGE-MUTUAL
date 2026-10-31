package store

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"

	"github.com/khayaai/forge-mutual/services/shared/audit-service/internal/chain"
)

// openTest needs AUDIT_TEST_DSN, a throwaway database. The table is dropped and recreated, so the immutability
// triggers are exercised from a clean state.
func openTest(t *testing.T) *Store {
	t.Helper()
	dsn := os.Getenv("AUDIT_TEST_DSN")
	if dsn == "" {
		t.Skip("AUDIT_TEST_DSN not set")
	}
	ctx := context.Background()
	s, err := Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	if _, err := s.pool.Exec(ctx, `alter table audit_entry disable trigger audit_entry_no_update`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.pool.Exec(ctx, `delete from audit_entry`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.pool.Exec(ctx, `alter table audit_entry enable trigger audit_entry_no_update`); err != nil {
		t.Fatal(err)
	}
	return s
}

func in(action string) chain.Input {
	return chain.Input{Actor: "svc-payments", Action: action, Entity: "BANK", Payload: []byte(`{"amount_minor":"100"}`)}
}

func TestAppendBuildsAVerifiableChain(t *testing.T) {
	ctx := context.Background()
	s := openTest(t)
	var last chain.Entry
	for _, a := range []string{"a", "b", "c"} {
		e, err := s.Append(ctx, in(a))
		if err != nil {
			t.Fatal(err)
		}
		last = e
	}
	n, head, err := s.VerifyAll(ctx)
	if err != nil || n != 3 || head != last.Hash {
		t.Fatalf("verify: n=%d head matches=%v err=%v", n, head == last.Hash, err)
	}
	seq, h, err := s.Head(ctx)
	if err != nil || seq != 3 || h != last.Hash {
		t.Fatalf("head = %d, %v", seq, err)
	}
}

// The log is append-only at the database. Updates, deletes and truncation are refused.
func TestUpdateDeleteAndTruncateAreRefused(t *testing.T) {
	ctx := context.Background()
	s := openTest(t)
	if _, err := s.Append(ctx, in("a")); err != nil {
		t.Fatal(err)
	}
	for name, sql := range map[string]string{
		"update":   `update audit_entry set actor = 'attacker'`,
		"delete":   `delete from audit_entry`,
		"truncate": `truncate audit_entry`,
	} {
		_, err := s.pool.Exec(ctx, sql)
		if err == nil {
			t.Fatalf("%s must be refused", name)
		}
	}
}

// Someone who disables the trigger and edits a row is caught by the verifier, at the row they changed.
func TestTamperingIsDetectedAtTheChangedRow(t *testing.T) {
	ctx := context.Background()
	s := openTest(t)
	for _, a := range []string{"a", "b", "c"} {
		if _, err := s.Append(ctx, in(a)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.pool.Exec(ctx, `alter table audit_entry disable trigger audit_entry_no_update`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.pool.Exec(ctx, `update audit_entry set payload = $1 where seq = 2`, []byte(`{"amount_minor":"999999"}`)); err != nil {
		t.Fatal(err)
	}
	if _, err := s.pool.Exec(ctx, `alter table audit_entry enable trigger audit_entry_no_update`); err != nil {
		t.Fatal(err)
	}
	_, _, err := s.VerifyAll(ctx)
	if !errors.Is(err, chain.ErrBroken) {
		t.Fatalf("tampering not detected: %v", err)
	}
}

// Concurrent writers must produce one unbroken chain: no duplicate sequence numbers and no fork.
func TestConcurrentWritersDoNotFork(t *testing.T) {
	ctx := context.Background()
	s := openTest(t)
	var wg sync.WaitGroup
	errs := make(chan error, 100)
	for w := 0; w < 10; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 10; i++ {
				if _, err := s.Append(ctx, in("concurrent")); err != nil {
					errs <- err
				}
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
	n, _, err := s.VerifyAll(ctx)
	if err != nil || n != 100 {
		t.Fatalf("n=%d err=%v", n, err)
	}
}
