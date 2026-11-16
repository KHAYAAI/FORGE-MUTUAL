package ledger

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	tbtypes "github.com/tigerbeetle/tigerbeetle-go/pkg/types"
)

// These tests need no cluster. They check the adapter's conversions and result mapping
// against the client's own types.

func TestIDRoundTripThroughUint128(t *testing.T) {
	id, err := NewID(time.Unix(1_800_000_000, 0))
	if err != nil {
		t.Fatal(err)
	}
	if got := fromU128(toU128(id)); got != id {
		t.Fatalf("round trip changed id: %s -> %s", id, got)
	}
}

// TigerBeetle orders IDs by their integer value, read little-endian. Later IDs should be larger
// so the cluster can append in order (TigerBeetle's recommendation for write performance).
func TestNewIDIsTimeOrderedAsTigerBeetleInteger(t *testing.T) {
	earlier, _ := NewID(time.UnixMilli(1_800_000_000_000))
	later, _ := NewID(time.UnixMilli(1_800_000_000_001))
	if earlier.toBig().Cmp(later.toBig()) >= 0 {
		t.Fatalf("ID from a later millisecond is not larger: %s >= %s", earlier, later)
	}
}

func TestU64Conversion(t *testing.T) {
	for _, v := range []uint64{0, 1, 1 << 40, math.MaxUint64} {
		got, err := u64(toU128Amount(v))
		if err != nil || got != v {
			t.Fatalf("u64(%d) = %d, %v", v, got, err)
		}
	}
	big := tbtypes.Uint128{}
	b := [16]byte{}
	b[8] = 1 // bit 64: does not fit in uint64
	big = tbtypes.BytesToUint128(b)
	if _, err := u64(big); err == nil {
		t.Fatal("value above 64 bits must be an error, not truncated")
	}
}

func TestTransferResultMapping(t *testing.T) {
	cases := map[tbtypes.CreateTransferResult]Reason{
		tbtypes.TransferOK:                                ReasonOK,
		tbtypes.TransferExists:                            ReasonExists,
		tbtypes.TransferExistsWithDifferentAmount:         ReasonExistsWithDifferentFields,
		tbtypes.TransferLinkedEventFailed:                 ReasonLinkedEventFailed,
		tbtypes.TransferDebitAccountNotFound:              ReasonAccountNotFound,
		tbtypes.TransferAccountsMustBeDifferent:           ReasonSameAccount,
		tbtypes.TransferExceedsCredits:                    ReasonInsufficientFunds,
		tbtypes.TransferOverflowsCreditsPosted:            ReasonOverflow,
		tbtypes.TransferPendingTransferNotFound:           ReasonPendingNotFound,
		tbtypes.TransferPendingTransferExpired:            ReasonPendingExpired,
		tbtypes.TransferPendingTransferHasDifferentAmount: ReasonPendingMismatch,
		tbtypes.TransferCodeMustNotBeZero:                 ReasonInvalidTransfer,
	}
	for code, want := range cases {
		if got := mapTransferResult(code); got != want {
			t.Errorf("mapTransferResult(%v) = %q, want %q", code, got, want)
		}
	}
}

func TestAccountResultMapping(t *testing.T) {
	cases := map[tbtypes.CreateAccountResult]Reason{
		tbtypes.AccountOK:                       ReasonOK,
		tbtypes.AccountExists:                   ReasonExists,
		tbtypes.AccountExistsWithDifferentFlags: ReasonExistsWithDifferentFields,
		tbtypes.AccountLinkedEventFailed:        ReasonLinkedEventFailed,
		tbtypes.AccountLedgerMustNotBeZero:      ReasonInvalidAccount,
	}
	for code, want := range cases {
		if got := mapAccountResult(code); got != want {
			t.Errorf("mapAccountResult(%v) = %q, want %q", code, got, want)
		}
	}
}

// With no quorum the client waits for the cluster for as long as it takes. The adapter must not: it gives up at its
// deadline, or the caller's, and says the outcome is unknown. Nothing listens on the address used here, which is how a
// cluster that has lost its quorum looks from outside.
func TestTigerBeetleStopsWaitingWhenTheClusterDoesNotAnswer(t *testing.T) {
	store, err := NewTigerBeetle(1, []string{"127.0.0.1:1"})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	store.maxWait = 300 * time.Millisecond
	transfer := []Transfer{{ID: ID{1}, DebitAccountID: ID{2}, CreditAccountID: ID{3}, Amount: 1, Ledger: 710, Code: 10}}

	start := time.Now()
	_, err = store.CreateTransfers(context.Background(), transfer)
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("err = %v, want ErrUnavailable", err)
	}
	if waited := time.Since(start); waited < 250*time.Millisecond || waited > 5*time.Second {
		t.Fatalf("waited %s for a 300ms limit", waited)
	}

	// The caller's own deadline is honoured when it is the shorter one.
	store.maxWait = time.Minute
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	start = time.Now()
	if err := store.Ping(ctx); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("ping: err = %v, want ErrUnavailable", err)
	}
	if waited := time.Since(start); waited > 5*time.Second {
		t.Fatalf("ping waited %s past a 200ms deadline", waited)
	}
	for _, call := range []func() error{
		func() error { _, err := store.LookupAccounts(ctx, []ID{{9}}); return err },
		func() error { _, err := store.LookupTransfers(ctx, []ID{{9}}); return err },
		func() error {
			_, err := store.CreateAccounts(ctx, []Account{{ID: ID{9}, Ledger: 710, Code: 1}})
			return err
		},
	} {
		if err := call(); !errors.Is(err, ErrUnavailable) {
			t.Fatalf("err = %v, want ErrUnavailable", err)
		}
	}
}

// Calls that are still waiting hold a slot each. When every slot is taken, the next call is refused at once instead of
// joining an unbounded queue behind a cluster that is away.
func TestTigerBeetleRefusesAtOnceWhenTooManyCallsAreWaiting(t *testing.T) {
	store, err := NewTigerBeetle(1, []string{"127.0.0.1:1"})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	store.maxWait = 100 * time.Millisecond
	store.slots = make(chan struct{}, 3)
	for i := 0; i < 3; i++ {
		if err := store.Ping(context.Background()); !errors.Is(err, ErrUnavailable) {
			t.Fatalf("call %d: %v", i, err)
		}
	}
	if len(store.slots) != 3 {
		t.Fatalf("%d slots held, want 3: the calls are still waiting for the cluster", len(store.slots))
	}
	start := time.Now()
	err = store.Ping(context.Background())
	if !errors.Is(err, ErrUnavailable) || time.Since(start) > 50*time.Millisecond {
		t.Fatalf("a call beyond the limit must be refused at once: %v after %s", err, time.Since(start))
	}
}
