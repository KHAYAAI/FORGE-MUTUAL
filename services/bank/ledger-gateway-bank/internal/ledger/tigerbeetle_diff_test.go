//go:build tbintegration

package ledger_test

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/khayaai/forge-mutual/services/bank/ledger-gateway-bank/internal/ledger"
)

// Every other suite in this repository runs on MemStore, on the claim that it behaves as TigerBeetle does. This test
// holds it to that. It sends the same random batches to the model and to a live cluster, and after every batch the
// per-event outcomes, every account's four balances and the stored transfers must be identical.
//
// The batches are the ones the gateway can produce, and the ones a retry after a crash produces: linked chains, pending
// transfers and their posts and voids, exact replays of an earlier batch, the same ID with a changed field, an ID that
// failed before, accounts that do not exist, and overdrafts on accounts that must not go negative.
//
// Requires TB_ADDRESSES and TB_CLUSTER_ID. TB_DIFF_SEED repeats a run; TB_DIFF_RUNS and TB_DIFF_STEPS size it.
func TestMemStoreAgreesWithTigerBeetle(t *testing.T) {
	tb := openCluster(t)
	seed := envInt(t, "TB_DIFF_SEED", time.Now().UnixNano())
	runs := int(envInt(t, "TB_DIFF_RUNS", 60))
	steps := int(envInt(t, "TB_DIFF_STEPS", 60))
	t.Logf("seed %d, %d runs of %d batches (set TB_DIFF_SEED to repeat)", seed, runs, steps)

	found := map[string]*divergence{}
	batches, events := 0, 0
	for run := 0; run < runs; run++ {
		w := newWorld(t, rand.New(rand.NewSource(seed+int64(run))), tb)
		for step := 0; step < steps; step++ {
			batch, kinds := w.nextBatch()
			batches++
			events += len(batch)
			if d := w.apply(batch, kinds); d != nil {
				d.where = fmt.Sprintf("seed %d run %d step %d", seed, run, step)
				if prev, ok := found[d.key]; ok {
					prev.count++
				} else {
					found[d.key] = d
				}
				break // the two ledgers now differ, so the rest of this run would only repeat it
			}
		}
	}
	t.Logf("%d batches, %d events compared", batches, events)
	if len(found) == 0 {
		return
	}
	keys := make([]string, 0, len(found))
	for k := range found {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		d := found[k]
		fmt.Fprintf(&b, "\n  %s  (%d runs; first at %s)\n      %s", d.key, d.count, d.where, d.detail)
	}
	t.Fatalf("the model and TigerBeetle disagree in %d ways:%s", len(found), b.String())
}

type divergence struct {
	key    string // what differed, in a form that groups repeats
	detail string
	where  string
	count  int
}

func openCluster(t *testing.T) *ledger.TigerBeetle {
	t.Helper()
	addrs := os.Getenv("TB_ADDRESSES")
	if addrs == "" {
		t.Skip("TB_ADDRESSES not set")
	}
	cluster, err := strconv.ParseUint(os.Getenv("TB_CLUSTER_ID"), 10, 64)
	if err != nil {
		t.Fatalf("TB_CLUSTER_ID: %v", err)
	}
	store, err := ledger.NewTigerBeetle(cluster, strings.Split(addrs, ","))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(store.Close)
	return store
}

func envInt(t *testing.T, name string, def int64) int64 {
	t.Helper()
	v := os.Getenv(name)
	if v == "" {
		return def
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return n
}

const (
	ledgerZAR = 710
	ledgerUSD = 840
)

// world is one run: a fresh model, fresh accounts on the shared cluster, and what has been sent so far.
type world struct {
	t   *testing.T
	r   *rand.Rand
	mem *ledger.MemStore
	tb  *ledger.TigerBeetle

	accounts []ledger.Account // as created; balances are read back, not tracked here
	sent     [][]ledger.Transfer
	pendings []ledger.Transfer // pending transfers the model accepted
	posted   []ledger.Transfer // plain transfers the model accepted
	failed   []ledger.Transfer // transfers the model refused
}

func newWorld(t *testing.T, r *rand.Rand, tb *ledger.TigerBeetle) *world {
	t.Helper()
	w := &world{t: t, r: r, mem: ledger.NewMemStore(nil), tb: tb}
	// Account 0 on each ledger is a source that may go negative, as the bank's own accounts may. The rest must not.
	for _, l := range []uint32{ledgerZAR, ledgerUSD} {
		for i := 0; i < 4; i++ {
			w.accounts = append(w.accounts, ledger.Account{
				ID: w.newID(), Ledger: l, Code: 1001, DebitsMustNotExceedCredits: i != 0,
				UserData64: uint64(r.Int63()), UserData32: r.Uint32(),
			})
		}
	}
	ctx := context.Background()
	mo, err := w.mem.CreateAccounts(ctx, w.accounts)
	if err != nil {
		t.Fatal(err)
	}
	// If the cluster does not answer in time (a replica is repairing after a restart), the same accounts are sent
	// again, as the adapter's contract says. They are then new, or already there from the attempt whose answer was lost.
	var to []ledger.Outcome
	for deadline := time.Now().Add(120 * time.Second); ; {
		to, err = w.tb.CreateAccounts(ctx, w.accounts)
		if err == nil {
			break
		}
		if !errors.Is(err, ledger.ErrUnavailable) || time.Now().After(deadline) {
			t.Fatal(err)
		}
	}
	for i := range mo {
		if mo[i].Reason != ledger.ReasonOK || !to[i].Reason.IsSuccess() {
			t.Fatalf("creating account %d: model %q, tigerbeetle %q", i, mo[i].Reason, to[i].Reason)
		}
	}
	return w
}

func (w *world) newID() ledger.ID {
	id, err := ledger.NewID(time.Now())
	if err != nil {
		w.t.Fatal(err)
	}
	return id
}

// pair returns two different accounts on one ledger.
func (w *world) pair() (ledger.Account, ledger.Account) {
	base := 0
	if w.r.Intn(2) == 0 {
		base = 4
	}
	d := w.r.Intn(4)
	c := (d + 1 + w.r.Intn(3)) % 4
	return w.accounts[base+d], w.accounts[base+c]
}

func (w *world) plain() ledger.Transfer {
	d, c := w.pair()
	amount := uint64(1 + w.r.Intn(500))
	if w.r.Intn(10) == 0 {
		amount = uint64(5_000 + w.r.Intn(50_000)) // large enough to overdraw an account that must not go negative
	}
	return ledger.Transfer{
		ID: w.newID(), DebitAccountID: d.ID, CreditAccountID: c.ID, Amount: amount, Ledger: d.Ledger, Code: 10,
		UserData64: uint64(w.r.Int63()), UserData32: w.r.Uint32(),
	}
}

func resolve(id ledger.ID, p ledger.Transfer, post bool) ledger.Transfer {
	return ledger.Transfer{
		ID: id, DebitAccountID: p.DebitAccountID, CreditAccountID: p.CreditAccountID, Amount: p.Amount, PendingID: p.ID,
		Ledger: p.Ledger, Code: p.Code, PostPending: post, VoidPending: !post,
	}
}

// nextBatch returns one to four events and, for each, what kind of event it is (for the report).
func (w *world) nextBatch() ([]ledger.Transfer, []string) {
	// A whole earlier batch again, exactly: what a caller sends when it never saw the first answer.
	if len(w.sent) > 0 && w.r.Intn(12) == 0 {
		prev := w.sent[w.r.Intn(len(w.sent))]
		kinds := make([]string, len(prev))
		for i := range kinds {
			kinds[i] = "replay of a batch"
			if len(prev) > 1 && prev[0].Linked {
				kinds[i] = "replay of a linked chain"
			}
		}
		return append([]ledger.Transfer(nil), prev...), kinds
	}
	n := 1 + w.r.Intn(4)
	batch := make([]ledger.Transfer, 0, n)
	kinds := make([]string, 0, n)
	for k := 0; k < n; k++ {
		tr, kind := w.one()
		tr.Linked = k < n-1 && w.r.Intn(2) == 0
		batch = append(batch, tr)
		kinds = append(kinds, kind)
	}
	return batch, kinds
}

func (w *world) one() (ledger.Transfer, string) {
	switch p := w.r.Intn(100); {
	case p < 45:
		return w.plain(), "transfer"
	case p < 60:
		tr := w.plain()
		tr.Pending = true
		tr.Timeout = 3600 // long enough that nothing expires during the run; expiry has its own test
		return tr, "pending transfer"
	case p < 78 && len(w.pendings) > 0:
		target := w.pendings[w.r.Intn(len(w.pendings))] // may already be posted or voided
		post := w.r.Intn(2) == 0
		if post {
			return resolve(w.newID(), target, true), "post"
		}
		return resolve(w.newID(), target, false), "void"
	case p < 82 && len(w.posted) > 0:
		// The same ID as an accepted transfer, with another amount.
		tr := w.posted[w.r.Intn(len(w.posted))]
		tr.Amount++
		tr.Linked = false
		return tr, "same id, different amount"
	case p < 86 && len(w.posted) > 0:
		tr := w.posted[w.r.Intn(len(w.posted))]
		tr.UserData64++
		tr.Linked = false
		return tr, "same id, different user data"
	case p < 91 && len(w.failed) > 0:
		// An ID that was refused before, sent again unchanged. Its account may have been funded since.
		tr := w.failed[w.r.Intn(len(w.failed))]
		tr.Linked = false
		return tr, "retry of a refused id"
	case p < 94:
		tr := w.plain()
		tr.CreditAccountID = w.newID()
		return tr, "unknown credit account"
	case p < 96:
		tr := w.plain()
		tr.DebitAccountID = w.newID()
		return tr, "unknown debit account"
	case p < 97:
		tr := w.plain()
		tr.CreditAccountID = tr.DebitAccountID
		return tr, "same account both sides"
	case p < 98:
		tr := w.plain()
		tr.CreditAccountID = w.accounts[(w.indexOf(tr.DebitAccountID)+4)%8].ID
		return tr, "accounts on two ledgers"
	case p < 99 && len(w.posted) > 0:
		return resolve(w.newID(), w.posted[w.r.Intn(len(w.posted))], w.r.Intn(2) == 0), "post or void of a transfer that is not pending"
	default:
		return resolve(w.newID(), ledger.Transfer{
			ID: w.newID(), DebitAccountID: w.accounts[0].ID, CreditAccountID: w.accounts[1].ID, Amount: 1, Ledger: ledgerZAR, Code: 10,
		}, w.r.Intn(2) == 0), "post or void of an unknown pending"
	}
}

// awaitCluster waits until the cluster answers again, and until the batch whose answer was lost has been settled one way
// or the other. The client delivers a request it still holds once the cluster is back, so a lookup that is answered
// after it comes after the batch too.
func (w *world) awaitCluster() {
	w.t.Helper()
	deadline := time.Now().Add(120 * time.Second)
	for {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		err := w.tb.Ping(ctx)
		cancel()
		if err == nil {
			return
		}
		if time.Now().After(deadline) {
			w.t.Fatalf("the cluster did not answer for two minutes: %v", err)
		}
	}
}

func (w *world) indexOf(id ledger.ID) int {
	for i, a := range w.accounts {
		if a.ID == id {
			return i
		}
	}
	w.t.Fatalf("account %s is not in this run", id)
	return -1
}

// apply sends the batch to both ledgers and compares everything. It returns the first difference, or nil.
func (w *world) apply(batch []ledger.Transfer, kinds []string) *divergence {
	w.t.Helper()
	ctx := context.Background()
	batch[len(batch)-1].Linked = false // a batch never leaves a chain open
	mo, err := w.mem.CreateTransfers(ctx, batch)
	if err != nil {
		w.t.Fatalf("model: %v", err)
	}
	to, err := w.tb.CreateTransfers(ctx, batch)
	if errors.Is(err, ledger.ErrUnavailable) {
		// The answer did not arrive in time (a replica died, a view changed). Whether the batch was applied is unknown,
		// and asking again would get "exists" where the lost answer said "ok". So this batch's outcomes cannot be
		// compared; its effect still can. Wait until the cluster answers, then compare balances and stored transfers.
		w.awaitCluster()
		to = nil
	} else if err != nil {
		w.t.Fatalf("tigerbeetle: %v", err)
	}
	w.sent = append(w.sent, batch)

	describe := func() string {
		var b strings.Builder
		for i, tr := range batch {
			link := ""
			if tr.Linked {
				link = " (linked to next)"
			}
			theirs := ledger.Reason("(answer lost)")
			if to != nil {
				theirs = to[i].Reason
			}
			fmt.Fprintf(&b, "[%d] %s%s: model %q, tigerbeetle %q; ", i, kinds[i], link, mo[i].Reason, theirs)
		}
		return b.String()
	}
	for i := range batch {
		if to == nil {
			break // outcomes unknown for this batch; balances and stored transfers are compared below
		}
		if mo[i].Reason != to[i].Reason {
			chain := ""
			if (i > 0 && batch[i-1].Linked) || batch[i].Linked {
				chain = " in a linked chain"
			}
			return &divergence{
				key:    fmt.Sprintf("%s%s: model says %q, TigerBeetle says %q", kinds[i], chain, mo[i].Reason, to[i].Reason),
				detail: describe(), count: 1,
			}
		}
	}
	for i, tr := range batch {
		switch {
		case mo[i].Reason == ledger.ReasonOK && tr.Pending:
			w.pendings = append(w.pendings, tr)
		case mo[i].Reason == ledger.ReasonOK && !tr.PostPending && !tr.VoidPending:
			w.posted = append(w.posted, tr)
		case !mo[i].Reason.IsSuccess() && mo[i].Reason != ledger.ReasonLinkedEventFailed:
			w.failed = append(w.failed, tr) // posts and voids too: a refused post is retried like any other
		}
	}

	// Balances.
	ids := make([]ledger.ID, len(w.accounts))
	for i, a := range w.accounts {
		ids[i] = a.ID
	}
	ma, err := w.mem.LookupAccounts(ctx, ids)
	if err != nil {
		w.t.Fatal(err)
	}
	ta, err := w.tb.LookupAccounts(ctx, ids)
	if err != nil {
		w.t.Fatal(err)
	}
	if len(ma) != len(ta) {
		w.t.Fatalf("lookup returned %d accounts from the model and %d from tigerbeetle", len(ma), len(ta))
	}
	for i := range ma {
		a, b := ma[i], ta[i]
		if a.ID != b.ID || a.DebitsPending != b.DebitsPending || a.DebitsPosted != b.DebitsPosted ||
			a.CreditsPending != b.CreditsPending || a.CreditsPosted != b.CreditsPosted {
			return &divergence{
				key: "balances differ although every outcome matched",
				detail: fmt.Sprintf("account %d: model dp=%d dP=%d cp=%d cP=%d, tigerbeetle dp=%d dP=%d cp=%d cP=%d; after %s", i,
					a.DebitsPending, a.DebitsPosted, a.CreditsPending, a.CreditsPosted,
					b.DebitsPending, b.DebitsPosted, b.CreditsPending, b.CreditsPosted, describe()),
				count: 1,
			}
		}
	}

	// Stored transfers: the same ones exist on both sides, with the same fields.
	tids := make([]ledger.ID, len(batch))
	for i, tr := range batch {
		tids[i] = tr.ID
	}
	mt, err := w.mem.LookupTransfers(ctx, tids)
	if err != nil {
		w.t.Fatal(err)
	}
	tt, err := w.tb.LookupTransfers(ctx, tids)
	if err != nil {
		w.t.Fatal(err)
	}
	if len(mt) != len(tt) {
		return &divergence{
			key:    "a transfer is stored on one side only",
			detail: fmt.Sprintf("model holds %d of the batch, tigerbeetle %d; after %s", len(mt), len(tt), describe()), count: 1,
		}
	}
	for i := range mt {
		a, b := mt[i], tt[i]
		a.Timestamp, b.Timestamp = 0, 0
		a.Linked, b.Linked = false, false // linking belongs to the batch that sent it, and a replay may link differently
		if a != b {
			return &divergence{
				key:    "a stored transfer has different fields",
				detail: fmt.Sprintf("model %+v, tigerbeetle %+v", a, b), count: 1,
			}
		}
	}
	return nil
}

// A pending transfer that times out releases its reservation without anyone asking. The model does it from its clock;
// TigerBeetle does it itself. Both must end in the same place, and a post afterwards must be refused as expired.
func TestPendingExpiryOnTigerBeetle(t *testing.T) {
	tb := openCluster(t)
	w := newWorld(t, rand.New(rand.NewSource(1)), tb)
	ctx := context.Background()
	src, dst := w.accounts[0], w.accounts[1]

	fund := ledger.Transfer{ID: w.newID(), DebitAccountID: src.ID, CreditAccountID: dst.ID, Amount: 1_000, Ledger: ledgerZAR, Code: 10}
	hold := ledger.Transfer{ID: w.newID(), DebitAccountID: dst.ID, CreditAccountID: src.ID, Amount: 800, Ledger: ledgerZAR, Code: 10, Pending: true, Timeout: 1}
	if d := w.apply([]ledger.Transfer{fund}, []string{"transfer"}); d != nil {
		t.Fatal(d.key, d.detail)
	}
	if d := w.apply([]ledger.Transfer{hold}, []string{"pending transfer"}); d != nil {
		t.Fatal(d.key, d.detail)
	}

	deadline := time.Now().Add(20 * time.Second)
	for {
		accs, err := tb.LookupAccounts(ctx, []ledger.ID{dst.ID})
		if err != nil {
			t.Fatal(err)
		}
		if accs[0].DebitsPending == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("tigerbeetle still holds the reservation 20s after a 1s timeout: %+v", accs[0])
		}
		time.Sleep(200 * time.Millisecond)
	}
	// By now the model's clock has passed the timeout too. A post is refused the same way on both, and the balances agree.
	post := resolve(w.newID(), hold, true)
	if d := w.apply([]ledger.Transfer{post}, []string{"post after expiry"}); d != nil {
		t.Fatal(d.key, d.detail)
	}
	out, err := tb.CreateTransfers(ctx, []ledger.Transfer{resolve(w.newID(), hold, false)})
	if err != nil {
		t.Fatal(err)
	}
	if out[0].Reason != ledger.ReasonPendingExpired {
		t.Fatalf("void after expiry: %q, want pending_expired", out[0].Reason)
	}
}
