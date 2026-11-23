//go:build tbintegration

package ledger_test

import (
	"context"
	"errors"
	"math/rand"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/khayaai/forge-mutual/services/bank/ledger-gateway-bank/internal/ledger"
)

// TestClusterSurvivesLosingReplicas takes a three-replica cluster apart while it is in use.
//
//  1. One replica is killed. The cluster keeps working, and the model still agrees with it on every outcome and balance.
//  2. It is started again and left to catch up, and then another is killed. The cluster now needs the one that came
//     back, so this passes only if that replica really did catch up.
//  3. A second replica is killed: no quorum. The adapter must say "unavailable" within its deadline, not wait for ever,
//     and nothing may be applied while it says so.
//  4. Everything is started again. The transfer that was tried during the outage is retried under the same ID, and must
//     have been applied exactly once: not lost, and not twice.
//
// Needs TB_ADDRESSES and TB_CLUSTER_ID for a three-replica cluster, and TB_CLUSTER_CONTAINERS, the names of its three
// containers, which this test kills and starts with docker. `make test-ledger-tb-cluster` sets all of it up.
func TestClusterSurvivesLosingReplicas(t *testing.T) {
	names := strings.Split(os.Getenv("TB_CLUSTER_CONTAINERS"), ",")
	if len(names) != 3 {
		t.Skip("TB_CLUSTER_CONTAINERS must name the three replica containers")
	}
	tb := openCluster(t)
	docker := func(args ...string) {
		t.Helper()
		if out, err := exec.Command("docker", args...).CombinedOutput(); err != nil {
			t.Fatalf("docker %v: %v: %s", args, err, out)
		}
	}
	t.Cleanup(func() { _ = exec.Command("docker", append([]string{"start"}, names...)...).Run() })
	ctx := context.Background()

	// caughtUp waits until a restarted replica has finished repairing. A replica that is behind asks its peers for the
	// operations it missed and says so in its log, many times a second; when it has them all, it goes quiet. There is no
	// other way to ask a replica how far behind it is. Taking a second replica away before then leaves the cluster
	// without a working majority until the first has finished (safe, but it answers "unavailable" for that long), and in
	// this development build on one machine a replica replays only a few operations a second.
	caughtUp := func(name string) {
		t.Helper()
		quiet := 0
		for deadline := time.Now().Add(10 * time.Minute); quiet < 3; {
			if time.Now().After(deadline) {
				t.Fatalf("%s was still repairing after ten minutes", name)
			}
			time.Sleep(time.Second)
			out, err := exec.Command("docker", "logs", "--since", "1s", name).CombinedOutput()
			if err != nil {
				t.Fatalf("docker logs %s: %v", name, err)
			}
			if strings.Contains(string(out), "no matching prepare") || strings.Contains(string(out), "lagging behind") {
				quiet = 0
			} else {
				quiet++
			}
		}
	}

	// agree runs the differential test's batches. A call that is cut off by a view change is sent again unchanged,
	// which is what the adapter's contract tells a caller to do.
	agree := func(what string, seed int64, steps int) {
		t.Helper()
		w := newWorld(t, rand.New(rand.NewSource(seed)), tb)
		for step := 0; step < steps; step++ {
			batch, kinds := w.nextBatch()
			if d := w.apply(batch, kinds); d != nil {
				t.Fatalf("%s, step %d: %s\n  %s", what, step, d.key, d.detail)
			}
		}
	}

	agree("all three replicas up", 1, 150)

	docker("kill", names[0])
	agree("one replica killed", 2, 80)

	docker("start", names[0])
	caughtUp(names[0])
	docker("kill", names[1])
	agree("the killed replica is back and another is gone", 3, 150)

	// No quorum.
	w := newWorld(t, rand.New(rand.NewSource(4)), tb) // accounts are created while two replicas are still up
	src, dst := w.accounts[0], w.accounts[1]
	docker("kill", names[2])
	short, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if err := tb.Ping(short); !errors.Is(err, ledger.ErrUnavailable) {
		t.Fatalf("ping with one replica of three: %v, want ErrUnavailable", err)
	}
	during := ledger.Transfer{ID: w.newID(), DebitAccountID: src.ID, CreditAccountID: dst.ID, Amount: 700, Ledger: ledgerZAR, Code: 10}
	short2, cancel2 := context.WithTimeout(ctx, 3*time.Second)
	defer cancel2()
	started := time.Now()
	if _, err := tb.CreateTransfers(short2, []ledger.Transfer{during}); !errors.Is(err, ledger.ErrUnavailable) {
		t.Fatalf("transfer with no quorum: %v, want ErrUnavailable", err)
	}
	if waited := time.Since(started); waited > 10*time.Second {
		t.Fatalf("the adapter waited %s past a 3s deadline", waited)
	}

	// Back to three replicas. The same transfer, sent again, is applied exactly once.
	docker("start", names[1], names[2])
	var reason ledger.Reason
	deadline := time.Now().Add(90 * time.Second)
	for {
		out, err := tb.CreateTransfers(ctx, []ledger.Transfer{during})
		if err == nil {
			reason = out[0].Reason
			break
		}
		if !errors.Is(err, ledger.ErrUnavailable) || time.Now().After(deadline) {
			t.Fatalf("the cluster did not come back: %v", err)
		}
	}
	if !reason.IsSuccess() {
		t.Fatalf("the retried transfer: %q, want ok or exists", reason)
	}
	out, err := tb.CreateTransfers(ctx, []ledger.Transfer{during})
	if err != nil || out[0].Reason != ledger.ReasonExists {
		t.Fatalf("a further retry: %v %v, want exists", out, err)
	}
	accs, err := tb.LookupAccounts(ctx, []ledger.ID{src.ID, dst.ID})
	if err != nil || len(accs) != 2 {
		t.Fatalf("lookup: %v %v", accs, err)
	}
	if accs[0].DebitsPosted != 700 || accs[1].CreditsPosted != 700 {
		t.Fatalf("applied %d on the debit side and %d on the credit side, want 700 once on each", accs[0].DebitsPosted, accs[1].CreditsPosted)
	}

	agree("all three replicas up again", 5, 150)
}
