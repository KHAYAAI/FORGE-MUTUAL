package inflight

import (
	"context"
	"os"
	"testing"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"

	"github.com/khayaai/forge-mutual/pkg/uuid"

	paymentsv1 "github.com/khayaai/forge-mutual/pkg/api/forge/payments/v1"
	railsv1 "github.com/khayaai/forge-mutual/pkg/api/forge/rails/v1"
	"github.com/khayaai/forge-mutual/services/bank/payments-orchestrator/internal/inbound"
	"github.com/khayaai/forge-mutual/services/bank/payments-orchestrator/internal/payments"
	"github.com/khayaai/forge-mutual/services/bank/payments-orchestrator/internal/store"
)

// These tests use PAYMENTS_TEST_PG. The payment and inbound tables are truncated, so use a throwaway database.
func setup(t *testing.T) (*Service, *store.Store) {
	t.Helper()
	dsn := os.Getenv("PAYMENTS_TEST_PG")
	if dsn == "" {
		t.Skip("PAYMENTS_TEST_PG not set")
	}
	ctx := context.Background()
	st, err := store.Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(st.Close)
	if err := inbound.Open(ctx, st.Pool()); err != nil {
		t.Fatal(err)
	}
	if _, err := st.Pool().Exec(ctx, `truncate payment, inbound_credit, inbound_reference`); err != nil {
		t.Fatal(err)
	}
	return &Service{Pool: st.Pool(), Inbound: &inbound.Worker{Pool: st.Pool()}}, st
}

func query(t *testing.T, s *Service, asset string) (string, string) {
	t.Helper()
	res, err := s.GetInFlight(context.Background(), connect.NewRequest(&paymentsv1.GetInFlightRequest{Asset: asset}))
	if err != nil {
		t.Fatal(err)
	}
	return res.Msg.InboundMinor, res.Msg.OutboundMinor
}

func payment(t *testing.T, st *store.Store, amount int64, state string, rail railsv1.RailStatus) {
	t.Helper()
	id, err := uuid.NewV4()
	if err != nil {
		t.Fatal(err)
	}
	party, err := uuid.NewV4()
	if err != nil {
		t.Fatal(err)
	}
	err = st.WithTx(context.Background(), func(tx pgx.Tx) error {
		return store.Upsert(context.Background(), tx, store.Row{
			PaymentID: id, PartyID: party, FromAccountID: "acc", Asset: "ZAR",
			AmountMinor: amount, State: state, RailStatus: rail.String(), InitiatedBy: "test",
		})
	})
	if err != nil {
		t.Fatal(err)
	}
}

// Outbound counts a payment the rail has settled before it completes: the sponsor has debited the pool and the
// books have not yet cleared it. Completed payments and rejected ones do not count.
func TestOutboundCountsSettledButNotCompleted(t *testing.T) {
	s, st := setup(t)
	payment(t, st, 1000, payments.StateRunning, railsv1.RailStatus_RAIL_STATUS_SETTLED)
	payment(t, st, 5000, payments.StateCompleted, railsv1.RailStatus_RAIL_STATUS_SETTLED)
	payment(t, st, 7000, payments.StateFailed, railsv1.RailStatus_RAIL_STATUS_REJECTED)
	payment(t, st, 900, payments.StateRepairRequired, railsv1.RailStatus_RAIL_STATUS_SETTLED)

	inboundTotal, outbound := query(t, s, "ZAR")
	if inboundTotal != "0" {
		t.Fatalf("inbound = %s, want 0", inboundTotal)
	}
	// 1000 running and settled, plus 900 needing repair: the bank has debited both and the books do not show it.
	if outbound != "1900" {
		t.Fatalf("outbound = %s, want 1900", outbound)
	}
}

// Inbound is the sum of pending credits, which the bank has reported and the books do not yet hold.
func TestInboundIsThePendingCredits(t *testing.T) {
	s, st := setup(t)
	if _, err := st.Pool().Exec(context.Background(), `
		insert into inbound_credit (asset, movement_id, amount_minor, reference, status)
		values ('ZAR', 'm1', 300, 'r1', 'PENDING'), ('ZAR', 'm2', 700, 'r2', 'BOOKED')`); err != nil {
		t.Fatal(err)
	}
	inboundTotal, _ := query(t, s, "ZAR")
	if inboundTotal != "300" {
		t.Fatalf("inbound = %s, want only the pending 300", inboundTotal)
	}
}

func TestAssetIsRequired(t *testing.T) {
	s := &Service{}
	if _, err := s.GetInFlight(context.Background(), connect.NewRequest(&paymentsv1.GetInFlightRequest{})); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("empty asset: %v", err)
	}
}
