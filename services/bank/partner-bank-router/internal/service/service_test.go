package service

import (
	"context"
	"errors"
	"os"
	"testing"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5/pgxpool"

	partnerv1 "github.com/khayaai/forge-mutual/pkg/api/forge/partner/v1"
	"github.com/khayaai/forge-mutual/pkg/banklayer"
	"github.com/khayaai/forge-mutual/pkg/banklayer/sim"
	"github.com/khayaai/forge-mutual/services/bank/partner-bank-router/internal/store"
)

// partner wraps a simulated USD partner under its own name, and can be told to fail. failPay runs the payout first and
// then reports an error: the partner has the payment, and the caller does not know that.
type partner struct {
	banklayer.ForeignAccountProvider
	name       string
	failOpen   error
	failPay    error
	acceptThen bool
}

func (p *partner) Partner() string { return p.name }

func (p *partner) OpenAccount(ctx context.Context, r banklayer.OpenAccountRequest) (banklayer.Account, error) {
	if p.failOpen != nil {
		return banklayer.Account{}, p.failOpen
	}
	return p.ForeignAccountProvider.OpenAccount(ctx, r)
}

func (p *partner) Payout(ctx context.Context, r banklayer.PayRequest) (banklayer.PayResult, error) {
	if p.acceptThen {
		res, err := p.ForeignAccountProvider.Payout(ctx, r)
		if err != nil {
			return res, err
		}
		return res, errors.New("connection reset after the partner accepted the payment")
	}
	if p.failPay != nil {
		return banklayer.PayResult{}, p.failPay
	}
	return p.ForeignAccountProvider.Payout(ctx, r)
}

func setup(t *testing.T) (*Service, *pgxpool.Pool, *partner, *partner) {
	t.Helper()
	dsn := os.Getenv("PARTNER_ROUTER_TEST_DSN")
	if dsn == "" {
		t.Skip("PARTNER_ROUTER_TEST_DSN not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := store.Open(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `truncate pbr_outcome, pbr_route`); err != nil {
		t.Fatal(err)
	}
	a := &partner{ForeignAccountProvider: sim.New(nil).ForeignCurrency()[0], name: "partner-a"}
	b := &partner{ForeignAccountProvider: sim.New(nil).ForeignCurrency()[0], name: "partner-b"}
	svc := &Service{Pool: pool, Partners: []Partner{{Provider: a, CostBps: 10}, {Provider: b, CostBps: 30}}}
	return svc, pool, a, b
}

func open(t *testing.T, s *Service) (*connect.Response[partnerv1.OpenAccountResponse], error) {
	t.Helper()
	return s.OpenAccount(context.Background(), connect.NewRequest(&partnerv1.OpenAccountRequest{Owner: "party-1", Asset: "USD"}))
}

func payout(s *Service, id string) (*connect.Response[partnerv1.PayoutResponse], error) {
	return s.Payout(context.Background(), connect.NewRequest(&partnerv1.PayoutRequest{
		PaymentId: id, Asset: "USD", AmountMinor: "1200", Beneficiary: "Cloud Vendor",
		AccountNumber: "9876543210", BankCode: "BIC", Reference: "inv",
	}))
}

// The cheapest healthy partner takes the operation.
func TestCheapestHealthyPartnerIsChosen(t *testing.T) {
	s, _, _, _ := setup(t)
	res, err := open(t, s)
	if err != nil || res.Msg.Partner != "partner-a" {
		t.Fatalf("open: %+v, %v; want partner-a (cheapest)", res, err)
	}
}

// After three consecutive failures the cheap partner is down, and new operations go to the next partner.
func TestFailoverAfterThreeFailures(t *testing.T) {
	s, _, a, _ := setup(t)
	a.failOpen = errors.New("partner unreachable")
	for i := 0; i < 3; i++ {
		if _, err := open(t, s); err == nil {
			t.Fatal("a failing partner must fail the call")
		}
	}
	res, err := open(t, s)
	if err != nil || res.Msg.Partner != "partner-b" {
		t.Fatalf("after three failures: %+v, %v; want partner-b", res, err)
	}
}

// A payment the partner may already hold stays with that partner. Even when it is down, a retry never goes elsewhere,
// because a second partner could pay the same money again. This is the case the pin exists for.
func TestPaymentStaysWithItsPartner(t *testing.T) {
	s, pool, a, b := setup(t)
	ctx := context.Background()
	a.acceptThen = true
	if _, err := payout(s, "pay-1"); err == nil {
		t.Fatal("the lost response must surface as an error")
	}
	for i := 0; i < 3; i++ {
		a.acceptThen = false
		a.failPay = errors.New("partner unreachable")
		_, _ = payout(s, "pay-other-"+string(rune('a'+i)))
	}
	a.failPay = errors.New("partner still unreachable")
	if _, err := payout(s, "pay-1"); err == nil {
		t.Fatal("a retry to the holding partner that fails must fail")
	}
	holder, err := store.PinnedPartner(ctx, pool, "pay-1")
	if err != nil || holder != "partner-a" {
		t.Fatalf("pin = %q, %v; want partner-a", holder, err)
	}
	if _, err := b.ForeignAccountProvider.PayoutStatus(ctx, "pay-1"); !errors.Is(err, banklayer.ErrNotFound) {
		t.Fatalf("partner-b must never have seen pay-1: %v", err)
	}
	st, err := s.PayoutStatus(ctx, connect.NewRequest(&partnerv1.PayoutStatusRequest{PaymentId: "pay-1"}))
	if err != nil || st.Msg.Partner != "partner-a" {
		t.Fatalf("status = %+v, %v; want from partner-a", st, err)
	}
}

// A request the partner refuses (invalid details) says nothing about the partner's health.
func TestRefusalDoesNotCountAgainstThePartner(t *testing.T) {
	s, _, a, _ := setup(t)
	a.failPay = banklayer.ErrInvalid
	for i := 0; i < 5; i++ {
		if _, err := payout(s, "bad-"+string(rune('a'+i))); err == nil {
			t.Fatal("a refused request must fail the call")
		}
	}
	health, err := s.GetHealth(context.Background(), connect.NewRequest(&partnerv1.GetHealthRequest{}))
	if err != nil {
		t.Fatal(err)
	}
	for _, h := range health.Msg.Partners {
		if h.Partner == "partner-a" && (h.State != "HEALTHY" || h.ConsecutiveFailures != 0) {
			t.Fatalf("refusals counted as failures: %+v", h)
		}
	}
}

// With no partner for the asset, the operation is refused. It is not sent to another asset's partner.
func TestNoPartnerForTheAssetRefuses(t *testing.T) {
	s, _, _, _ := setup(t)
	_, err := s.OpenAccount(context.Background(), connect.NewRequest(&partnerv1.OpenAccountRequest{Owner: "p", Asset: "EUR"}))
	if connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("err = %v, want FailedPrecondition", err)
	}
}
