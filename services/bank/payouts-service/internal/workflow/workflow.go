// Package workflow runs a payout batch. BulkPayout is the parent: it fans out one PayoutItem child per open item, a few at
// a time, and then records the batch's state. An item is sent to payments under a key derived from the batch and the item,
// so a retry or a resume never pays it twice.
//
// Workflow inputs and results carry item keys and states, never beneficiary details, so account numbers stay out of the
// Temporal history.
package workflow

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	paymentsv1 "github.com/khayaai/forge-mutual/pkg/api/forge/payments/v1"
	"github.com/khayaai/forge-mutual/services/bank/payouts-service/internal/store"
)

// DefaultConcurrency is how many items a batch sends at once.
const DefaultConcurrency = 10

// Payments is the part of the payments service the activities use.
type Payments interface {
	// CreatePayment returns the payment id. A refusal is a Connect error with a code that Refused recognises.
	CreatePayment(ctx context.Context, req *paymentsv1.CreatePaymentRequest) (string, error)
	GetPayment(ctx context.Context, paymentID string) (paymentsv1.PaymentState, error)
}

// BatchInput starts a run of a batch.
type BatchInput struct {
	BatchID      string
	Run          uint32
	Concurrency  int
	PollInterval time.Duration
	MaxPolls     int
}

// ItemInput is one item's share of a run.
type ItemInput struct {
	BatchID      string
	Key          string
	Run          uint32
	PollInterval time.Duration
	MaxPolls     int
}

// ItemResult is what a PayoutItem child reports: the item's state when it stopped following it.
type ItemResult struct {
	Key   string
	State string
}

// SendInput names the item to send.
type SendInput struct {
	BatchID string
	ItemKey string
}

// SendResult says what happened to the send. Outcome is "sent" (payments accepted it; the outcome is not known yet),
// "refused" (payments refused it: final), "settled" or "failed" (already final in the store).
type SendResult struct {
	Outcome   string
	PaymentID string
	Reason    string
}

// PollInput names the payment to look at.
type PollInput struct {
	BatchID   string
	ItemKey   string
	PaymentID string
}

// PollResult is "settled", "failed" or "pending".
type PollResult struct {
	Outcome string
	Reason  string
}

// MarkInput records that an item's outcome is unknown.
type MarkInput struct {
	BatchID string
	ItemKey string
	Reason  string
}

// Activities hold the side effects. They are the only code that writes to the store or calls payments.
type Activities struct {
	Pool     *pgxpool.Pool
	Payments Payments
}

// PaymentKey is the idempotency key an item is sent under. Payments derives the payment from it, so an item always maps
// to the same payment.
func PaymentKey(batchID, itemKey string) string {
	return "payout:" + batchID + ":" + itemKey
}

// Refused reports whether payments refused the request itself. Such a refusal is final: the item is not sent again.
// Anything else (unreachable, timeout, server error) leaves the outcome unknown.
func Refused(err error) bool {
	if err == nil {
		return false
	}
	switch connect.CodeOf(err) {
	case connect.CodeInvalidArgument, connect.CodeAlreadyExists, connect.CodeFailedPrecondition,
		connect.CodePermissionDenied, connect.CodeNotFound:
		return true
	}
	return false
}

// OpenItems returns the keys of the items that still need a send or a poll.
func (a *Activities) OpenItems(ctx context.Context, batchID string) ([]string, error) {
	items, err := store.OpenItems(ctx, a.Pool, batchID)
	if err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(items))
	for _, it := range items {
		keys = append(keys, it.Key)
	}
	return keys, nil
}

// SendItem sends one item. An item already accepted is not sent again: its payment id is returned.
func (a *Activities) SendItem(ctx context.Context, in SendInput) (SendResult, error) {
	b, err := store.Header(ctx, a.Pool, in.BatchID)
	if err != nil {
		return SendResult{}, err
	}
	it, err := store.GetItem(ctx, a.Pool, in.BatchID, in.ItemKey)
	if err != nil {
		return SendResult{}, err
	}
	switch it.State {
	case store.ItemSettled:
		return SendResult{Outcome: "settled"}, nil
	case store.ItemFailed:
		return SendResult{Outcome: "failed", Reason: it.Reason}, nil
	case store.ItemSent:
		return SendResult{Outcome: "sent", PaymentID: it.PaymentID}, nil
	}

	id, err := a.Payments.CreatePayment(ctx, &paymentsv1.CreatePaymentRequest{
		IdempotencyKey:  PaymentKey(in.BatchID, in.ItemKey),
		PartyId:         b.PartyID,
		FromAccountId:   b.FromAccountID,
		AmountMinor:     strconv.FormatInt(it.AmountMinor, 10),
		Asset:           b.Asset,
		BeneficiaryName: it.BeneficiaryName,
		AccountNumber:   it.AccountNumber,
		BankCode:        it.BankCode,
		Reference:       it.Reference,
		Actor:           b.ApprovedBy,
	})
	if err != nil && Refused(err) {
		reason := err.Error()
		if err := store.SetItem(ctx, a.Pool, in.BatchID, in.ItemKey, store.ItemFailed, "", reason); err != nil {
			return SendResult{}, err
		}
		return SendResult{Outcome: "refused", Reason: reason}, nil
	}
	if err != nil {
		// Not final. Temporal retries the send with the same key, which payments answers as a replay if it already took it.
		return SendResult{}, err
	}
	if err := store.SetItem(ctx, a.Pool, in.BatchID, in.ItemKey, store.ItemSent, id, ""); err != nil {
		return SendResult{}, err
	}
	return SendResult{Outcome: "sent", PaymentID: id}, nil
}

// PollItem reads the payment's state and records a final outcome. A payment still in progress is "pending".
func (a *Activities) PollItem(ctx context.Context, in PollInput) (PollResult, error) {
	state, err := a.Payments.GetPayment(ctx, in.PaymentID)
	if err != nil {
		return PollResult{}, err
	}
	switch state {
	case paymentsv1.PaymentState_PAYMENT_STATE_COMPLETED:
		if err := store.SetItem(ctx, a.Pool, in.BatchID, in.ItemKey, store.ItemSettled, "", ""); err != nil {
			return PollResult{}, err
		}
		return PollResult{Outcome: "settled"}, nil
	case paymentsv1.PaymentState_PAYMENT_STATE_DECLINED,
		paymentsv1.PaymentState_PAYMENT_STATE_FAILED,
		paymentsv1.PaymentState_PAYMENT_STATE_CANCELLED:
		reason := state.String()
		if err := store.SetItem(ctx, a.Pool, in.BatchID, in.ItemKey, store.ItemFailed, "", reason); err != nil {
			return PollResult{}, err
		}
		return PollResult{Outcome: "failed", Reason: reason}, nil
	}
	return PollResult{Outcome: "pending"}, nil
}

// MarkUncertain records that payments could not be reached, so the item's outcome is unknown.
func (a *Activities) MarkUncertain(ctx context.Context, in MarkInput) error {
	return store.SetItem(ctx, a.Pool, in.BatchID, in.ItemKey, store.ItemUncertain, "", in.Reason)
}

// FinishRun records the batch's state at the end of a run.
func (a *Activities) FinishRun(ctx context.Context, batchID string, run uint32) (string, error) {
	return store.FinishRun(ctx, a.Pool, batchID, run)
}

var (
	sendOptions = workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval: time.Second, BackoffCoefficient: 2, MaximumInterval: 30 * time.Second, MaximumAttempts: 5,
		},
	}
	bookkeepingOptions = workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval: time.Second, BackoffCoefficient: 2, MaximumInterval: time.Minute, MaximumAttempts: 10,
		},
	}
)

// BulkPayout runs the open items of a batch and returns the batch's state afterwards.
func BulkPayout(ctx workflow.Context, in BatchInput) (string, error) {
	log := workflow.GetLogger(ctx)
	var a *Activities
	actx := workflow.WithActivityOptions(ctx, bookkeepingOptions)

	var keys []string
	if err := workflow.ExecuteActivity(actx, a.OpenItems, in.BatchID).Get(actx, &keys); err != nil {
		return "", err
	}

	conc := in.Concurrency
	if conc <= 0 {
		conc = DefaultConcurrency
	}
	for start := 0; start < len(keys); start += conc {
		end := min(start+conc, len(keys))
		futures := make([]workflow.ChildWorkflowFuture, 0, end-start)
		for _, key := range keys[start:end] {
			cctx := workflow.WithChildOptions(ctx, workflow.ChildWorkflowOptions{
				WorkflowID: fmt.Sprintf("payout-item:%s:%d:%s", in.BatchID, in.Run, key),
			})
			futures = append(futures, workflow.ExecuteChildWorkflow(cctx, PayoutItem, ItemInput{
				BatchID: in.BatchID, Key: key, Run: in.Run, PollInterval: in.PollInterval, MaxPolls: in.MaxPolls,
			}))
		}
		for _, f := range futures {
			var r ItemResult
			if err := f.Get(ctx, &r); err != nil {
				// The item's state is in the store. FinishRun reads it, and a resume picks up anything still open.
				log.Error("payout item workflow failed", "err", err)
			}
		}
	}

	var state string
	if err := workflow.ExecuteActivity(actx, a.FinishRun, in.BatchID, in.Run).Get(actx, &state); err != nil {
		return "", err
	}
	return state, nil
}

// PayoutItem sends one item and follows it until payments reports a final outcome, or until it has polled MaxPolls times.
// It always returns a result rather than an error, so one item cannot fail the batch.
func PayoutItem(ctx workflow.Context, in ItemInput) (ItemResult, error) {
	var a *Activities
	sctx := workflow.WithActivityOptions(ctx, sendOptions)

	var sent SendResult
	err := workflow.ExecuteActivity(sctx, a.SendItem, SendInput{BatchID: in.BatchID, ItemKey: in.Key}).Get(sctx, &sent)
	if err != nil {
		// Payments could not be reached within the retries, so whether it took the payment is unknown. The item is marked
		// uncertain. A resume sends it again under the same key, which payments answers as a replay if it had taken it.
		bctx := workflow.WithActivityOptions(ctx, bookkeepingOptions)
		_ = workflow.ExecuteActivity(bctx, a.MarkUncertain, MarkInput{
			BatchID: in.BatchID, ItemKey: in.Key, Reason: "payments could not be reached",
		}).Get(bctx, nil)
		return ItemResult{Key: in.Key, State: store.ItemUncertain}, nil
	}
	switch sent.Outcome {
	case "refused", "failed":
		return ItemResult{Key: in.Key, State: store.ItemFailed}, nil
	case "settled":
		return ItemResult{Key: in.Key, State: store.ItemSettled}, nil
	}

	for i := 0; i < in.MaxPolls; i++ {
		var poll PollResult
		if err := workflow.ExecuteActivity(sctx, a.PollItem, PollInput{
			BatchID: in.BatchID, ItemKey: in.Key, PaymentID: sent.PaymentID,
		}).Get(sctx, &poll); err == nil {
			switch poll.Outcome {
			case "settled":
				return ItemResult{Key: in.Key, State: store.ItemSettled}, nil
			case "failed":
				return ItemResult{Key: in.Key, State: store.ItemFailed}, nil
			}
		}
		if err := workflow.Sleep(ctx, in.PollInterval); err != nil {
			return ItemResult{}, err
		}
	}
	// Still in progress after the polls. The item stays sent, and a resume follows it again.
	return ItemResult{Key: in.Key, State: store.ItemSent}, nil
}
