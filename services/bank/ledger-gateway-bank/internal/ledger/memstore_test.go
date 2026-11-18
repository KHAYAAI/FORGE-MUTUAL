package ledger

import (
	"context"
	"math/rand"
	"testing"
	"time"
)

var ctx = context.Background()

// fixture builds a store with a clock the test controls. Account 0 is the operating
// account (no overdraft limit, so it can fund others); accounts 1..n-1 are customer
// accounts with debits_must_not_exceed_credits when dmnec is true.
type fixture struct {
	t     *testing.T
	s     *MemStore
	clock time.Time
	ids   []ID
}

func newFixture(t *testing.T, n int, dmnec bool) *fixture {
	t.Helper()
	f := &fixture{t: t, clock: time.Unix(1_800_000_000, 0)}
	f.s = NewMemStore(func() time.Time { return f.clock })
	var accs []Account
	for i := 0; i < n; i++ {
		id, err := NewID(f.clock)
		if err != nil {
			t.Fatal(err)
		}
		f.ids = append(f.ids, id)
		accs = append(accs, Account{ID: id, Ledger: 710, Code: 1001, DebitsMustNotExceedCredits: dmnec && i > 0})
	}
	out, _ := f.s.CreateAccounts(ctx, accs)
	for _, o := range out {
		if !o.Reason.IsSuccess() {
			t.Fatalf("create account %d: %s", o.Index, o.Reason)
		}
	}
	return f
}

func (f *fixture) newID() ID {
	id, err := NewID(f.clock)
	if err != nil {
		f.t.Fatal(err)
	}
	return id
}

func (f *fixture) transfer(debit, credit int, amt uint64) Transfer {
	return Transfer{ID: f.newID(), DebitAccountID: f.ids[debit], CreditAccountID: f.ids[credit], Amount: amt, Ledger: 710, Code: 10}
}

func (f *fixture) run(batch []Transfer) []Outcome {
	f.t.Helper()
	out, err := f.s.CreateTransfers(ctx, batch)
	if err != nil {
		f.t.Fatal(err)
	}
	return out
}

func (f *fixture) acc(i int) Account {
	f.t.Helper()
	as, _ := f.s.LookupAccounts(ctx, []ID{f.ids[i]})
	if len(as) != 1 {
		f.t.Fatalf("account %d missing", i)
	}
	return as[0]
}

func TestDepositThenPaymentMovesFunds(t *testing.T) {
	f := newFixture(t, 3, true)
	fund := f.transfer(0, 1, 10_000)
	if out := f.run([]Transfer{fund}); out[0].Reason != ReasonOK {
		t.Fatalf("fund: %s", out[0].Reason)
	}
	pay := f.transfer(1, 2, 4_000)
	if out := f.run([]Transfer{pay}); out[0].Reason != ReasonOK {
		t.Fatalf("pay: %s", out[0].Reason)
	}
	if got := f.acc(1).Available().Int64(); got != 6_000 {
		t.Fatalf("available = %d, want 6000", got)
	}
}

func TestInsufficientFundsRejectedNoNegativeBalance(t *testing.T) {
	f := newFixture(t, 2, true)
	out := f.run([]Transfer{f.transfer(1, 0, 1)})
	if out[0].Reason != ReasonInsufficientFunds {
		t.Fatalf("want insufficient_funds, got %s", out[0].Reason)
	}
	if f.acc(1).Available().Sign() < 0 {
		t.Fatal("negative balance on DMNEC account")
	}
}

func TestLinkedChainIsAtomic(t *testing.T) {
	f := newFixture(t, 3, true)
	legA := f.transfer(0, 1, 500) // succeeds on its own
	legA.Linked = true
	legB := f.transfer(1, 2, 100)
	legB.Ledger = 840 // ledger mismatch: fails
	out := f.run([]Transfer{legA, legB})
	if out[0].Reason != ReasonLinkedEventFailed {
		t.Fatalf("leg A: want linked_event_failed, got %s", out[0].Reason)
	}
	if out[1].Reason != ReasonLedgerMismatch {
		t.Fatalf("leg B: want ledger_mismatch, got %s", out[1].Reason)
	}
	if f.acc(0).DebitsPosted != 0 || f.acc(1).CreditsPosted != 0 {
		t.Fatal("failed chain left a partial effect")
	}
}

func TestOpenChainRejected(t *testing.T) {
	f := newFixture(t, 2, false)
	a := f.transfer(0, 1, 1)
	a.Linked = true
	out := f.run([]Transfer{a})
	if out[0].Reason != ReasonInvalidTransfer {
		t.Fatalf("want invalid_transfer for open chain, got %s", out[0].Reason)
	}
}

func TestReplayIsIdempotent(t *testing.T) {
	f := newFixture(t, 2, true)
	tr := f.transfer(0, 1, 250)
	if out := f.run([]Transfer{tr}); out[0].Reason != ReasonOK {
		t.Fatalf("first: %s", out[0].Reason)
	}
	if out := f.run([]Transfer{tr}); out[0].Reason != ReasonExists {
		t.Fatalf("replay: want exists, got %s", out[0].Reason)
	}
	if f.acc(1).CreditsPosted != 250 {
		t.Fatalf("replay moved money: credits=%d", f.acc(1).CreditsPosted)
	}
	changed := tr
	changed.Amount = 251
	if out := f.run([]Transfer{changed}); out[0].Reason != ReasonExistsWithDifferentFields {
		t.Fatalf("different payload: want exists_with_different_fields, got %s", out[0].Reason)
	}
}

func TestPendingPostAndVoid(t *testing.T) {
	f := newFixture(t, 2, true)
	p := f.transfer(0, 1, 300)
	p.Pending = true
	p.Timeout = 600
	if out := f.run([]Transfer{p}); out[0].Reason != ReasonOK {
		t.Fatalf("pending: %s", out[0].Reason)
	}
	if f.acc(0).DebitsPending != 300 || f.acc(1).CreditsPending != 300 {
		t.Fatal("pending did not reserve")
	}
	post := Transfer{ID: f.newID(), DebitAccountID: f.ids[0], CreditAccountID: f.ids[1], Amount: 300, PendingID: p.ID, Ledger: 710, Code: 10, PostPending: true}
	if out := f.run([]Transfer{post}); out[0].Reason != ReasonOK {
		t.Fatalf("post: %s", out[0].Reason)
	}
	if f.acc(1).CreditsPosted != 300 || f.acc(1).CreditsPending != 0 {
		t.Fatal("post did not settle")
	}
	void := Transfer{ID: f.newID(), DebitAccountID: f.ids[0], CreditAccountID: f.ids[1], Amount: 300, PendingID: p.ID, Ledger: 710, Code: 10, VoidPending: true}
	if out := f.run([]Transfer{void}); out[0].Reason != ReasonPendingAlreadyPosted {
		t.Fatalf("void after post: want already_posted, got %s", out[0].Reason)
	}
}

func TestPendingExpiryReleasesReservation(t *testing.T) {
	f := newFixture(t, 2, true)
	f.run([]Transfer{f.transfer(0, 1, 1_000)})
	p := f.transfer(1, 0, 800)
	p.Pending = true
	p.Timeout = 60
	if out := f.run([]Transfer{p}); out[0].Reason != ReasonOK {
		t.Fatalf("pending: %s", out[0].Reason)
	}
	if f.acc(1).DebitsPending != 800 {
		t.Fatal("reservation missing")
	}
	f.clock = f.clock.Add(61 * time.Second)
	if f.acc(1).DebitsPending != 0 || f.acc(1).Available().Int64() != 1_000 {
		t.Fatal("expired pending did not release funds")
	}
	post := Transfer{ID: f.newID(), DebitAccountID: f.ids[1], CreditAccountID: f.ids[0], Amount: 800, PendingID: p.ID, Ledger: 710, Code: 10, PostPending: true}
	if out := f.run([]Transfer{post}); out[0].Reason != ReasonPendingExpired {
		t.Fatalf("post after expiry: want pending_expired, got %s", out[0].Reason)
	}
}

func TestConservationAndNoNegativeProperty(t *testing.T) {
	const accounts = 6
	r := rand.New(rand.NewSource(42))
	for run := 0; run < 200; run++ {
		f := newFixture(t, accounts, true)
		// Seed each account so transfers can succeed.
		for i := 1; i < accounts; i++ {
			if out := f.run([]Transfer{f.transfer(0, i, 1_000)}); out[0].Reason != ReasonOK {
				t.Fatalf("seed %d: %s", i, out[0].Reason)
			}
		}
		var pendings []Transfer
		for step := 0; step < 40; step++ {
			var batch []Transfer
			n := 1 + r.Intn(4)
			for k := 0; k < n; k++ {
				d, c := r.Intn(accounts), r.Intn(accounts)
				if d == c {
					c = (c + 1) % accounts
				}
				tr := f.transfer(d, c, uint64(1+r.Intn(500)))
				tr.Linked = k < n-1 && r.Intn(2) == 0
				switch r.Intn(4) {
				case 0:
					tr.Pending = true
					tr.Timeout = uint32(r.Intn(120))
				case 1:
					if len(pendings) > 0 {
						pt := pendings[r.Intn(len(pendings))]
						tr = Transfer{ID: f.newID(), DebitAccountID: pt.DebitAccountID, CreditAccountID: pt.CreditAccountID, Amount: pt.Amount, PendingID: pt.ID, Ledger: 710, Code: 10, PostPending: r.Intn(2) == 0, VoidPending: false}
						tr.VoidPending = !tr.PostPending
						tr.Linked = false
					}
				}
				batch = append(batch, tr)
			}
			// The batch must close: the last event never links to a nonexistent successor.
			batch[len(batch)-1].Linked = false
			out := f.run(batch)
			for i, o := range out {
				if o.Reason == ReasonOK && batch[i].Pending {
					pendings = append(pendings, batch[i])
				}
			}
			if r.Intn(3) == 0 {
				f.clock = f.clock.Add(time.Duration(r.Intn(90)) * time.Second)
			}
			checkInvariants(t, f)
		}
	}
}

// checkInvariants verifies conservation (sum debits == sum credits per side) and that no
// DMNEC account is overdrawn.
func checkInvariants(t *testing.T, f *fixture) {
	t.Helper()
	var dPosted, cPosted, dPend, cPend uint64
	for i := range f.ids {
		a := f.acc(i)
		dPosted += a.DebitsPosted
		cPosted += a.CreditsPosted
		dPend += a.DebitsPending
		cPend += a.CreditsPending
		if a.DebitsMustNotExceedCredits && a.Available().Sign() < 0 {
			t.Fatalf("account %d overdrawn: %s", i, a.Available())
		}
	}
	if dPosted != cPosted {
		t.Fatalf("posted conservation broken: debits %d credits %d", dPosted, cPosted)
	}
	if dPend != cPend {
		t.Fatalf("pending conservation broken: debits %d credits %d", dPend, cPend)
	}
}
