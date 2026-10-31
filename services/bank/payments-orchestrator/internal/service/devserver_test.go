//go:build temporaldev

package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"os"
	"testing"
	"time"

	"connectrpc.com/connect"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"

	paymentsv1 "github.com/khayaai/forge-mutual/pkg/api/forge/payments/v1"
	railsv1 "github.com/khayaai/forge-mutual/pkg/api/forge/rails/v1"
	"github.com/khayaai/forge-mutual/services/bank/payments-orchestrator/internal/payments"
)

// devActivities is a happy-path set for the dev-server tests: the party is active, policy allows, the hold
// is placed, and the rail's behaviour is set per test. The workflow and the service run unchanged.
type devActivities struct {
	neverFinal bool
}

func (d *devActivities) LoadParty(context.Context, string) (payments.PartyInfo, error) {
	return payments.PartyInfo{Status: "PARTY_STATUS_ACTIVE", Tier: "standard"}, nil
}

func (d *devActivities) Screen(context.Context, payments.Input) ([]string, error) { return nil, nil }

func (d *devActivities) CheckAssetEntity(context.Context, payments.Input) (payments.Decision, error) {
	return payments.Decision{Allow: true, Reason: "ok"}, nil
}

func (d *devActivities) CheckLimits(context.Context, payments.Input, payments.PartyInfo) (payments.Decision, error) {
	return payments.Decision{Allow: true, Reason: "ok"}, nil
}

func (d *devActivities) Reserve(context.Context, payments.Input) (payments.ReserveResult, error) {
	return payments.ReserveResult{OK: true, TransferID: "hold-dev"}, nil
}

func (d *devActivities) RailInitiate(context.Context, payments.Input) (payments.RailResult, error) {
	return payments.RailResult{Status: railsv1.RailStatus_RAIL_STATUS_ACCEPTED, Reference: "RS-dev"}, nil
}

func (d *devActivities) RailStatus(context.Context, payments.Input) (payments.RailResult, error) {
	if d.neverFinal {
		return payments.RailResult{Status: railsv1.RailStatus_RAIL_STATUS_PENDING}, nil
	}
	return payments.RailResult{Status: railsv1.RailStatus_RAIL_STATUS_SETTLED}, nil
}

func (d *devActivities) Settle(context.Context, payments.Input, string, string) error { return nil }

func (d *devActivities) Release(context.Context, payments.Input, string, string, string, string) error {
	return nil
}

func (d *devActivities) Record(context.Context, payments.Input, string, string, string) error {
	return nil
}

func devRand(t *testing.T) string {
	t.Helper()
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(b)
}

// devService starts the service against a real server, with a worker on a fresh queue.
func devService(t *testing.T, acts *devActivities, cfg Config) (*Service, client.Client) {
	t.Helper()
	hostPort := os.Getenv("TEMPORAL_HOST")
	if hostPort == "" {
		t.Skip("TEMPORAL_HOST not set")
	}
	c, err := client.Dial(client.Options{HostPort: hostPort, Namespace: "default"})
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(c.Close)
	cfg.TaskQueue = "payments-dev-" + devRand(t)
	w := worker.New(c, cfg.TaskQueue, worker.Options{})
	w.RegisterWorkflow(payments.DomesticPayment)
	w.RegisterActivity(acts)
	if err := w.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(w.Stop)
	return New(c, cfg), c
}

func devRequest(key string) *connect.Request[paymentsv1.CreatePaymentRequest] {
	return connect.NewRequest(&paymentsv1.CreatePaymentRequest{
		IdempotencyKey: key, PartyId: "11111111-1111-4111-8111-111111111111",
		FromAccountId: "0123456789abcdef0123456789abcdef", AmountMinor: "15000", Asset: "ZAR",
		BeneficiaryName: "Sipho Ndlovu", AccountNumber: "1234567890", BankCode: "250655", Actor: "founder",
	})
}

func TestDevPaymentSettlesAndIsIdempotent(t *testing.T) {
	s, c := devService(t, &devActivities{}, Config{PollSeconds: 1})
	key := "dev-pay-" + devRand(t)
	first, err := s.CreatePayment(context.Background(), devRequest(key))
	if err != nil {
		t.Fatal(err)
	}
	again, err := s.CreatePayment(context.Background(), devRequest(key))
	if err != nil || again.Msg.PaymentId != first.Msg.PaymentId {
		t.Fatalf("replay on a real server: %v %v", again, err)
	}
	changed := devRequest(key)
	changed.Msg.AmountMinor = "15001"
	if _, err := s.CreatePayment(context.Background(), changed); connect.CodeOf(err) != connect.CodeAlreadyExists {
		t.Fatalf("changed request under the same key: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	var final payments.Status
	if err := c.GetWorkflow(ctx, first.Msg.PaymentId, "").Get(ctx, &final); err != nil {
		t.Fatalf("workflow: %v", err)
	}
	if final.State != payments.StateCompleted {
		t.Fatalf("final = %+v", final)
	}
}

// A rail that never reaches an outcome drives the payment to repair; an operator's decision on the real
// server then completes it. This exercises the query and the repair signal end to end.
func TestDevRepairIsResolvedThroughTheService(t *testing.T) {
	s, _ := devService(t, &devActivities{neverFinal: true}, Config{PollSeconds: 1, MaxPolls: 2})
	created, err := s.CreatePayment(context.Background(), devRequest("dev-repair-"+devRand(t)))
	if err != nil {
		t.Fatal(err)
	}
	id := created.Msg.PaymentId
	deadline := time.Now().Add(60 * time.Second)
	for {
		got, err := s.GetPayment(context.Background(), connect.NewRequest(&paymentsv1.GetPaymentRequest{PaymentId: id}))
		if err == nil && got.Msg.Status.State == paymentsv1.PaymentState_PAYMENT_STATE_REPAIR_REQUIRED {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("payment never reached repair: %v %v", got, err)
		}
		time.Sleep(200 * time.Millisecond)
	}
	if _, err := s.ResolveRepair(context.Background(), connect.NewRequest(&paymentsv1.ResolveRepairRequest{
		PaymentId: id, Settled: true, Operator: "ops",
	})); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	got, err := s.GetPayment(ctx, connect.NewRequest(&paymentsv1.GetPaymentRequest{PaymentId: id}))
	for i := 0; i < 50 && err == nil && got.Msg.Status.State != paymentsv1.PaymentState_PAYMENT_STATE_COMPLETED; i++ {
		time.Sleep(200 * time.Millisecond)
		got, err = s.GetPayment(ctx, connect.NewRequest(&paymentsv1.GetPaymentRequest{PaymentId: id}))
	}
	if err != nil || got.Msg.Status.State != paymentsv1.PaymentState_PAYMENT_STATE_COMPLETED {
		t.Fatalf("after the operator's settlement: %v %v", got, err)
	}
}
