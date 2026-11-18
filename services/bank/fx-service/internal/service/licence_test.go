package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"connectrpc.com/connect"

	fxv1 "github.com/khayaai/forge-mutual/pkg/api/forge/fx/v1"
	"github.com/khayaai/forge-mutual/pkg/licence"
	"github.com/khayaai/forge-mutual/services/bank/fx-service/internal/rates"
)

func quote(svc *Service, sell, buy string) error {
	_, err := svc.Quote(context.Background(), connect.NewRequest(&fxv1.QuoteRequest{PartyId: "p", SellAsset: sell, BuyAsset: buy, SellAmountMinor: "100000"}))
	return err
}

// In production the euro and the pound are refused until a decision approves them; the dollar is unaffected. The
// refusal comes before any rate is read or anything is stored.
func TestProductionRefusesTheEuroAndThePoundUntilApproved(t *testing.T) {
	gate, err := licence.FromEnv(func(k string) string {
		return map[string]string{"FORGE_ENV": "production", "FORGE_APPROVAL_GBP_ACCOUNTS": "DEC-0040"}[k]
	})
	if err != nil {
		t.Fatal(err)
	}
	svc := &Service{Rates: rates.Static{Rates: map[string]int64{}, AsOf: time.Now()}, MaxAge: time.Hour, Licence: gate}
	if err := quote(svc, "ZAR", "EUR"); connect.CodeOf(err) != connect.CodeFailedPrecondition || !errors.Is(err, licence.ErrNotApproved) {
		t.Fatalf("euro in production: %v", err)
	}
	if err := quote(svc, "GBP", "ZAR"); errors.Is(err, licence.ErrNotApproved) {
		t.Fatalf("an approved pound was refused by the licence: %v", err)
	}
}

// A currency with no configured rate is refused, not priced at zero.
func TestACurrencyWithNoRateIsRefused(t *testing.T) {
	svc := &Service{Rates: rates.Static{Rates: map[string]int64{"USD": 1_825_000_000}, AsOf: time.Now()}, MaxAge: time.Hour}
	if err := quote(svc, "ZAR", "GBP"); connect.CodeOf(err) != connect.CodeFailedPrecondition || !errors.Is(err, rates.ErrNoRate) {
		t.Fatalf("pound with no rate: %v", err)
	}
	if err := quote(svc, "EUR", "USD"); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("a cross without the rand: %v", err)
	}
}
