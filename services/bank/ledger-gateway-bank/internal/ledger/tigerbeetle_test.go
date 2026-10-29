package ledger

import (
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
