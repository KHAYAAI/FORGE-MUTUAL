package payments

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/khayaai/forge-mutual/pkg/events"
	"strconv"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"go.temporal.io/sdk/temporal"
	"google.golang.org/protobuf/types/known/structpb"

	eventsv1 "github.com/khayaai/forge-mutual/pkg/api/forge/events/v1"
	ledgerv1 "github.com/khayaai/forge-mutual/pkg/api/forge/ledger/v1"
	"github.com/khayaai/forge-mutual/pkg/api/forge/ledger/v1/ledgerv1connect"
	partyv1 "github.com/khayaai/forge-mutual/pkg/api/forge/party/v1"
	"github.com/khayaai/forge-mutual/pkg/api/forge/party/v1/partyv1connect"
	policyv1 "github.com/khayaai/forge-mutual/pkg/api/forge/policy/v1"
	"github.com/khayaai/forge-mutual/pkg/api/forge/policy/v1/policyv1connect"
	railsv1 "github.com/khayaai/forge-mutual/pkg/api/forge/rails/v1"
	"github.com/khayaai/forge-mutual/pkg/api/forge/rails/v1/railsv1connect"
	"github.com/khayaai/forge-mutual/pkg/outbox"
	"github.com/khayaai/forge-mutual/pkg/screening"
	"github.com/khayaai/forge-mutual/services/bank/payments-orchestrator/internal/store"
)

// Ledger transfer code for a payment out (Appendix B: PAYMENT_OUT).
const (
	codePaymentOut = 11
	codeRailSettle = 31 // RAIL_SETTLE: the sponsor has debited the pool for a payment
)

// Event topics (ARCHITECTURE.md Appendix C).
const (
	TopicRequested = "forge.bank.payment.requested.v1"
	TopicSettled   = "forge.bank.payment.settled.v1"
	TopicFailed    = "forge.bank.payment.failed.v1"
)

// Activities holds the collaborators. Each method is one activity.
type Activities struct {
	Store       *store.Store
	Party       partyv1connect.PartyServiceClient
	Policy      policyv1connect.PolicyServiceClient
	Ledger      ledgerv1connect.LedgerServiceClient
	Rails       railsv1connect.RailServiceClient
	Screener    screening.Screener
	Recon       ReconStatus
	Settlement  string // the ledger account of RAIL_SETTLEMENT_EFT (code 3001)
	SponsorPool string // the ledger account of SPONSOR_POOLED (code 3003), the mirror of the sponsor's pooled account
	Entity      string // "BANK"
}

// LoadParty reads the paying party's status and tier.
func (a *Activities) LoadParty(ctx context.Context, partyID string) (PartyInfo, error) {
	res, err := a.Party.GetParty(ctx, connect.NewRequest(&partyv1.GetPartyRequest{PartyId: partyID}))
	if err != nil {
		return PartyInfo{}, retryable(err)
	}
	return PartyInfo{Status: res.Msg.Party.Status.String(), Tier: res.Msg.Party.Tier}, nil
}

// Screen checks the beneficiary's name against the sanctions and PEP list.
func (a *Activities) Screen(ctx context.Context, in Input) ([]string, error) {
	res, err := a.Screener.Screen(ctx, []string{in.BeneficiaryName})
	if err != nil {
		return nil, err
	}
	return res.Hits, nil
}

// CheckAssetEntity asks policy whether the entity may hold the asset and whether the transfer stays inside
// the entity.
func (a *Activities) CheckAssetEntity(ctx context.Context, in Input) (Decision, error) {
	assets, err := a.evaluate(ctx, "assets", in.PaymentID, map[string]any{"entity_code": a.Entity, "asset": in.Asset})
	if err != nil {
		return Decision{}, err
	}
	if !assets.Allow {
		return Decision{Reason: "asset_" + assets.Reason}, nil
	}
	entity, err := a.evaluate(ctx, "entity", in.PaymentID, map[string]any{
		"entity_code": a.Entity, "debit_entity": a.Entity, "credit_entity": a.Entity, "code": codePaymentOut,
	})
	if err != nil {
		return Decision{}, err
	}
	if !entity.Allow {
		return Decision{Reason: "entity_" + entity.Reason}, nil
	}
	return Decision{Allow: true, Reason: "ok"}, nil
}

// CheckLimits asks policy whether the payment is within the party's tier limits, given the day's velocity
// and the reconciliation status. The screening hit is false here: the screening step has already declined
// any match.
func (a *Activities) CheckLimits(ctx context.Context, in Input, party PartyInfo) (Decision, error) {
	since := time.Now().UTC().Truncate(24 * time.Hour)
	held, err := store.HeldSince(ctx, a.Store.Pool(), in.PartyID, in.Asset, since)
	if err != nil {
		return Decision{}, err
	}
	breaking, err := a.Recon.Breaking(ctx)
	if err != nil {
		return Decision{}, err
	}
	d, err := a.evaluate(ctx, "limits", in.PaymentID, map[string]any{
		"party":        map[string]any{"status": party.Status, "tier": party.Tier},
		"screening":    map[string]any{"hit": false},
		"recon":        map[string]any{"break_active": breaking},
		"asset":        in.Asset,
		"amount_minor": in.AmountMinor,
		"velocity":     map[string]any{"day_total_minor": strconv.FormatInt(held, 10)},
	})
	if err != nil {
		return Decision{}, err
	}
	return d, nil
}

func (a *Activities) evaluate(ctx context.Context, point, requestID string, input map[string]any) (Decision, error) {
	st, err := structpb.NewStruct(input)
	if err != nil {
		return Decision{}, err
	}
	res, err := a.Policy.Evaluate(ctx, connect.NewRequest(&policyv1.EvaluateRequest{
		DecisionPoint: point, EntityCode: a.Entity, RequestId: requestID, Input: st,
	}))
	if err != nil {
		return Decision{}, retryable(err)
	}
	return Decision{Allow: res.Msg.Allow, StepUp: res.Msg.Stepup, Reason: res.Msg.Reason}, nil
}

// Reserve places the hold: the customer's account is debited pending, against the settlement account.
// The ledger's idempotency key is the payment ID, so a retry returns the same hold. The payment row and its
// REQUESTED event are written in the same transaction as the hold is recorded.
func (a *Activities) Reserve(ctx context.Context, in Input) (ReserveResult, error) {
	res, err := a.Ledger.CreateTransfers(ctx, connect.NewRequest(&ledgerv1.CreateTransfersRequest{
		IdempotencyKey: in.PaymentID + ":reserve",
		BusinessTxnId:  in.PaymentID,
		EntityCode:     a.Entity,
		Legs: []*ledgerv1.TransferLeg{{
			DebitAccountId:  in.FromAccountID,
			CreditAccountId: a.Settlement,
			Amount:          &ledgerv1.Money{Asset: in.Asset, AmountMinor: in.AmountMinor},
			Code:            codePaymentOut,
		}},
		Pending:               true,
		PendingTimeoutSeconds: uint32(in.ReserveTimeoutSeconds),
		Actor:                 in.Actor,
		Reason:                "payment " + in.PaymentID,
	}))
	if err != nil {
		return ReserveResult{}, retryable(err)
	}
	switch res.Msg.Status {
	case ledgerv1.Status_STATUS_OK, ledgerv1.Status_STATUS_REPLAYED:
		if len(res.Msg.TransferIds) != 1 {
			return ReserveResult{}, temporal.NewNonRetryableApplicationError("ledger returned no hold", ErrTypeValidation, nil)
		}
		transfer := res.Msg.TransferIds[0]
		if err := a.writeRecord(ctx, in, StateReserved, "", "", transfer, TopicRequested); err != nil {
			return ReserveResult{}, err
		}
		return ReserveResult{OK: true, TransferID: transfer}, nil
	default:
		reason := "ledger_rejected"
		for _, code := range res.Msg.RejectionCodes {
			if code == "INSUFFICIENT_FUNDS" {
				reason = "insufficient_funds"
			}
		}
		return ReserveResult{OK: false, Reason: reason}, nil
	}
}

// RailInitiate sends the instruction. The rail's idempotency key is the payment ID.
func (a *Activities) RailInitiate(ctx context.Context, in Input) (RailResult, error) {
	res, err := a.Rails.Initiate(ctx, connect.NewRequest(&railsv1.InitiateRequest{
		InstructionId: in.PaymentID, Rail: "EFT", AmountMinor: in.AmountMinor, Asset: in.Asset,
		BeneficiaryName: in.BeneficiaryName, AccountNumber: in.AccountNumber, BankCode: in.BankCode,
		Scenario: in.Scenario,
	}))
	if err != nil {
		return RailResult{}, retryable(err)
	}
	return RailResult{Status: res.Msg.Status, Reference: res.Msg.Reference}, nil
}

// RailStatus reads the rail's current status for the instruction and records it. The record matters: once the rail
// reports SETTLED, the sponsor has debited the pool, and the payment is in flight until Settle clears it. Reconciliation
// reads that from the record, so the status must be written when it is seen, not only at the end.
func (a *Activities) RailStatus(ctx context.Context, in Input) (RailResult, error) {
	res, err := a.Rails.Status(ctx, connect.NewRequest(&railsv1.StatusRequest{InstructionId: in.PaymentID}))
	if err != nil {
		return RailResult{}, retryable(err)
	}
	if err := a.writeRecord(ctx, in, StateReserved, "", res.Msg.Status.String(), "", ""); err != nil {
		return RailResult{}, retryable(err)
	}
	return RailResult{Status: res.Msg.Status, Reference: res.Msg.Reference}, nil
}

// Settle posts the hold to the settlement account, then records the sponsor's debit: RAIL_SETTLE moves the amount from
// the settlement account to the sponsor pool. Without that second leg the pool would still hold the money, and
// reconciliation would break on every settled payment. Both legs have fixed keys, so a retry replays them.
func (a *Activities) Settle(ctx context.Context, in Input, transferID, railStatus string) error {
	res, err := a.Ledger.PostPending(ctx, connect.NewRequest(&ledgerv1.PostPendingRequest{
		IdempotencyKey:     in.PaymentID + ":settle",
		BusinessTxnId:      in.PaymentID,
		EntityCode:         a.Entity,
		PendingTransferIds: []string{transferID},
		Actor:              "payments",
		Reason:             "settled",
	}))
	if err != nil {
		return retryable(err)
	}
	if res.Msg.Status != ledgerv1.Status_STATUS_OK && res.Msg.Status != ledgerv1.Status_STATUS_REPLAYED {
		return temporal.NewNonRetryableApplicationError(fmt.Sprintf("hold cannot be posted: %v", res.Msg.RejectionCodes), ErrTypeValidation, nil)
	}
	if a.SponsorPool == "" {
		return temporal.NewNonRetryableApplicationError("sponsor pool account is not configured", ErrTypeValidation, nil)
	}
	clear, err := a.Ledger.CreateTransfers(ctx, connect.NewRequest(&ledgerv1.CreateTransfersRequest{
		IdempotencyKey: in.PaymentID + ":rail-settle",
		BusinessTxnId:  in.PaymentID,
		EntityCode:     a.Entity,
		Legs: []*ledgerv1.TransferLeg{{
			DebitAccountId:  a.Settlement,
			CreditAccountId: a.SponsorPool,
			Amount:          &ledgerv1.Money{Asset: in.Asset, AmountMinor: in.AmountMinor},
			Code:            codeRailSettle,
		}},
		Actor:  "payments",
		Reason: "sponsor debited for payment",
	}))
	if err != nil {
		return retryable(err)
	}
	if clear.Msg.Status != ledgerv1.Status_STATUS_OK && clear.Msg.Status != ledgerv1.Status_STATUS_REPLAYED {
		return temporal.NewNonRetryableApplicationError(fmt.Sprintf("rail settlement refused: %v", clear.Msg.RejectionCodes), ErrTypeValidation, nil)
	}
	return a.writeRecord(ctx, in, StateCompleted, "", railStatus, transferID, TopicSettled)
}

// Release voids the hold, if one was placed, and records the outcome with its event.
func (a *Activities) Release(ctx context.Context, in Input, transferID, state, reason, railStatus string) error {
	if transferID != "" {
		res, err := a.Ledger.VoidPending(ctx, connect.NewRequest(&ledgerv1.VoidPendingRequest{
			IdempotencyKey:     in.PaymentID + ":release",
			BusinessTxnId:      in.PaymentID,
			EntityCode:         a.Entity,
			PendingTransferIds: []string{transferID},
			Actor:              "payments",
			Reason:             reason,
		}))
		if err != nil {
			return retryable(err)
		}
		if res.Msg.Status != ledgerv1.Status_STATUS_OK && res.Msg.Status != ledgerv1.Status_STATUS_REPLAYED {
			return temporal.NewNonRetryableApplicationError(fmt.Sprintf("hold cannot be voided: %v", res.Msg.RejectionCodes), ErrTypeValidation, nil)
		}
	}
	topic := TopicFailed
	if state == StateCancelled {
		topic = "" // no money was ever held; nothing to tell the customer's bank
	}
	return a.writeRecord(ctx, in, state, reason, railStatus, transferID, topic)
}

// Record writes a state change that has no ledger effect: approval waits, repairs, declines and the first
// record of a payment.
func (a *Activities) Record(ctx context.Context, in Input, state, reason, railStatus string) error {
	topic := ""
	if state == StateDeclined {
		topic = TopicFailed
	}
	return a.writeRecord(ctx, in, state, reason, railStatus, "", topic)
}

// writeRecord upserts the payment row and, when topic is set, appends the event in the same transaction.
func (a *Activities) writeRecord(ctx context.Context, in Input, state, reason, railStatus, transferID, topic string) error {
	amount, err := strconv.ParseInt(in.AmountMinor, 10, 64)
	if err != nil {
		return temporal.NewNonRetryableApplicationError("amount is not an integer", ErrTypeValidation, err)
	}
	row := store.Row{
		PaymentID: in.PaymentID, PartyID: in.PartyID, FromAccountID: in.FromAccountID, Asset: in.Asset,
		AmountMinor: amount, State: state, Reason: reason, RailStatus: railStatus,
		ReserveTransferID: transferID, InitiatedBy: in.Actor,
	}
	return a.Store.WithTx(ctx, func(tx pgx.Tx) error {
		if err := store.Upsert(ctx, tx, row); err != nil {
			return err
		}
		if topic == "" {
			return nil
		}
		eventID, payload, err := events.Wrap(events.Params{
			EventID: "", Type: topic, Entity: a.Entity, Producer: "payments-orchestrator", Correlation: in.PaymentID,
		}, &eventsv1.PaymentEvent{
			PaymentId: in.PaymentID, PartyId: in.PartyID, FromAccountId: in.FromAccountID,
			AmountMinor: in.AmountMinor, Asset: in.Asset, Status: state, Reason: reason,
			BeneficiaryKey: beneficiaryKey(in.BankCode, in.AccountNumber),
		})
		if err != nil {
			return err
		}
		return outbox.Append(ctx, tx, outbox.Event{EventID: eventID, Topic: topic, Key: []byte(in.PaymentID), Payload: payload})
	})
}

// retryable keeps transient failures retryable and marks request errors non-retryable.
func retryable(err error) error {
	if err == nil {
		return nil
	}
	var ce *connect.Error
	if errors.As(err, &ce) {
		switch ce.Code() {
		case connect.CodeInvalidArgument, connect.CodeFailedPrecondition, connect.CodeAlreadyExists,
			connect.CodeNotFound, connect.CodePermissionDenied, connect.CodeUnauthenticated:
			return temporal.NewNonRetryableApplicationError(ce.Message(), ErrTypeValidation, err)
		}
	}
	return err
}

// beneficiaryKey identifies a payee on the event bus without carrying the account number: monitoring counts payments to
// the same payee by it.
func beneficiaryKey(bankCode, accountNumber string) string {
	sum := sha256.Sum256([]byte(bankCode + "|" + accountNumber))
	return hex.EncodeToString(sum[:])
}
