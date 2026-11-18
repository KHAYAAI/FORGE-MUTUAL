// Package service implements the payouts API: create a batch (maker), approve it (checker), and resume it when some items
// are still open. Approving or resuming starts a run of the BulkPayout workflow after the batch's state has been committed.
package service

import (
	"bytes"
	"context"
	"errors"
	"math"
	"regexp"
	"strconv"
	"strings"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/protobuf/proto"

	payoutsv1 "github.com/khayaai/forge-mutual/pkg/api/forge/payouts/v1"
	"github.com/khayaai/forge-mutual/pkg/api/forge/payouts/v1/payoutsv1connect"
	"github.com/khayaai/forge-mutual/pkg/uuid"
	"github.com/khayaai/forge-mutual/services/bank/payouts-service/internal/store"
)

// Ownership reports the accounts a party owns, and their assets.
type Ownership interface {
	Accounts(ctx context.Context, party string) (map[string]string, error)
}

// Starter starts a run of a batch.
type Starter interface {
	StartRun(ctx context.Context, batchID string, run uint32) error
}

// Limits bound what one batch may hold. The values are placeholders until their owner confirms them (docs/THRESHOLDS.md).
type Limits struct {
	MaxItems     int
	MaxItemMinor int64
}

// Service implements PayoutsService.
type Service struct {
	Pool   *pgxpool.Pool
	Owners Ownership
	Runs   Starter
	Limits Limits
}

var _ payoutsv1connect.PayoutsServiceHandler = (*Service)(nil)

var (
	itemKeyRe  = regexp.MustCompile(`^[A-Za-z0-9._-]{1,64}$`)
	accountRe  = regexp.MustCompile(`^[A-Za-z0-9]{4,34}$`)
	bankCodeRe = regexp.MustCompile(`^[A-Za-z0-9]{1,11}$`)
	amountRe   = regexp.MustCompile(`^[1-9][0-9]*$`)
)

// CreateBatch implements PayoutsServiceHandler. The same key with the same request returns the batch already created.
func (s *Service) CreateBatch(ctx context.Context, req *connect.Request[payoutsv1.CreateBatchRequest]) (*connect.Response[payoutsv1.CreateBatchResponse], error) {
	m := req.Msg
	if k := m.IdempotencyKey; k == "" || len(k) > 128 {
		return nil, invalid("idempotency_key must be 1 to 128 characters")
	}
	if _, err := uuid.Parse(m.PartyId); err != nil {
		return nil, invalid("party_id must be a canonical UUID")
	}
	if a := strings.TrimSpace(m.FromAccountId); a == "" || len(a) > 64 {
		return nil, invalid("from_account_id is required, up to 64 characters")
	}
	if m.Asset != "ZAR" {
		return nil, invalid("asset must be ZAR: domestic payments only")
	}
	if a := strings.TrimSpace(m.Actor); a == "" || len(a) > 64 {
		return nil, invalid("actor is required, up to 64 characters")
	}
	items, total, err := validateItems(m.Items, s.Limits)
	if err != nil {
		return nil, err
	}

	owned, err := s.Owners.Accounts(ctx, m.PartyId)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnavailable, errors.New("account ownership is unavailable"))
	}
	asset, ok := owned[m.FromAccountId]
	if !ok {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("the account does not belong to the party"))
	}
	if asset != m.Asset {
		return nil, invalid("the account does not hold " + m.Asset)
	}

	hash, err := requestHash(m)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	id, err := uuid.NewV4()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	b, created, err := store.Create(ctx, s.Pool, store.Batch{
		ID: id, IdempotencyKey: m.IdempotencyKey, RequestHash: hash, PartyID: m.PartyId,
		FromAccountID: m.FromAccountId, Asset: m.Asset, TotalMinor: total, State: store.StateDraft,
		CreatedBy: m.Actor, Items: items,
	})
	if err != nil {
		return nil, mapErr(err)
	}
	if !created && !bytes.Equal(b.RequestHash, hash) {
		return nil, connect.NewError(connect.CodeAlreadyExists, errors.New("idempotency key reused with a different request"))
	}
	return connect.NewResponse(&payoutsv1.CreateBatchResponse{Batch: toProto(b)}), nil
}

// GetBatch implements PayoutsServiceHandler.
func (s *Service) GetBatch(ctx context.Context, req *connect.Request[payoutsv1.GetBatchRequest]) (*connect.Response[payoutsv1.GetBatchResponse], error) {
	if _, err := uuid.Parse(req.Msg.BatchId); err != nil {
		return nil, invalid("batch_id must be a canonical UUID")
	}
	b, err := store.Get(ctx, s.Pool, req.Msg.BatchId)
	if err != nil {
		return nil, mapErr(err)
	}
	return connect.NewResponse(&payoutsv1.GetBatchResponse{Batch: toProto(b)}), nil
}

// ApproveBatch implements PayoutsServiceHandler. The checker must not be the maker. Approving a batch that is already
// running or finished changes nothing; a running batch's current run is started again, which is safe.
func (s *Service) ApproveBatch(ctx context.Context, req *connect.Request[payoutsv1.ApproveBatchRequest]) (*connect.Response[payoutsv1.ApproveBatchResponse], error) {
	m := req.Msg
	if _, err := uuid.Parse(m.BatchId); err != nil {
		return nil, invalid("batch_id must be a canonical UUID")
	}
	if strings.TrimSpace(m.Actor) == "" {
		return nil, invalid("actor is required")
	}
	b, err := s.advance(ctx, m.BatchId, m.Actor, false)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&payoutsv1.ApproveBatchResponse{Batch: toProto(b)}), nil
}

// ResumeBatch implements PayoutsServiceHandler. It runs again the items with no final outcome. A batch that is still
// running has its current run started again; a batch that ended with exceptions gets a new run.
func (s *Service) ResumeBatch(ctx context.Context, req *connect.Request[payoutsv1.ResumeBatchRequest]) (*connect.Response[payoutsv1.ResumeBatchResponse], error) {
	m := req.Msg
	if _, err := uuid.Parse(m.BatchId); err != nil {
		return nil, invalid("batch_id must be a canonical UUID")
	}
	if strings.TrimSpace(m.Actor) == "" {
		return nil, invalid("actor is required")
	}
	b, err := s.advance(ctx, m.BatchId, m.Actor, true)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&payoutsv1.ResumeBatchResponse{Batch: toProto(b)}), nil
}

// advance moves a batch to its next run inside a transaction, under a row lock, so an approval and a resume cannot
// interleave. The run itself starts after the commit.
func (s *Service) advance(ctx context.Context, batchID, actor string, resume bool) (store.Batch, error) {
	var run uint32
	start := false
	err := pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
		b, err := store.Lock(ctx, tx, batchID)
		if err != nil {
			return mapErr(err)
		}
		switch b.State {
		case store.StateDraft:
			if resume {
				return connect.NewError(connect.CodeFailedPrecondition, errors.New("the batch is not approved"))
			}
			if sameActor(actor, b.CreatedBy) {
				return connect.NewError(connect.CodePermissionDenied, errors.New("the checker must not be the maker"))
			}
			run = 1
			if err := store.Start(ctx, tx, batchID, run, actor); err != nil {
				return err
			}
			start = true
		case store.StateRunning:
			run = b.Run
			start = true
		case store.StateExceptions:
			if resume {
				run = b.Run + 1
				if err := store.Start(ctx, tx, batchID, run, ""); err != nil {
					return err
				}
				start = true
			}
		}
		return nil
	})
	if err != nil {
		return store.Batch{}, err
	}
	if start {
		if err := s.Runs.StartRun(ctx, batchID, run); err != nil {
			// The state is committed as running. Calling again starts the same run, so nothing is lost.
			return store.Batch{}, connect.NewError(connect.CodeUnavailable, errors.New("workflow service unavailable; retry"))
		}
	}
	b, err := store.Get(ctx, s.Pool, batchID)
	if err != nil {
		return store.Batch{}, mapErr(err)
	}
	return b, nil
}

func validateItems(in []*payoutsv1.ItemInput, lim Limits) ([]store.Item, int64, error) {
	if len(in) == 0 || len(in) > lim.MaxItems {
		return nil, 0, invalid("a batch holds 1 to " + strconv.Itoa(lim.MaxItems) + " items")
	}
	seen := make(map[string]bool, len(in))
	out := make([]store.Item, 0, len(in))
	var total int64
	for _, x := range in {
		if !itemKeyRe.MatchString(x.ItemKey) {
			return nil, 0, invalid("item_key must be 1 to 64 letters, digits, dots, dashes or underscores")
		}
		if seen[x.ItemKey] {
			return nil, 0, invalid("item_key " + x.ItemKey + " appears twice")
		}
		seen[x.ItemKey] = true
		if !amountRe.MatchString(x.AmountMinor) {
			return nil, 0, invalid("amount_minor must be a positive whole number of minor units")
		}
		amount, err := strconv.ParseInt(x.AmountMinor, 10, 64)
		if err != nil || amount > lim.MaxItemMinor {
			return nil, 0, invalid("an item is above the per-item limit or out of range")
		}
		if total > math.MaxInt64-amount {
			return nil, 0, invalid("the batch total is out of range")
		}
		total += amount
		if n := strings.TrimSpace(x.BeneficiaryName); n == "" || len(n) > 140 {
			return nil, 0, invalid("beneficiary_name is required, up to 140 characters")
		}
		if !accountRe.MatchString(x.AccountNumber) {
			return nil, 0, invalid("account_number must be 4 to 34 letters or digits")
		}
		if !bankCodeRe.MatchString(x.BankCode) {
			return nil, 0, invalid("bank_code must be 1 to 11 letters or digits")
		}
		if r := strings.TrimSpace(x.Reference); r == "" || len(r) > 140 {
			return nil, 0, invalid("reference is required, up to 140 characters")
		}
		out = append(out, store.Item{
			Key: x.ItemKey, Seq: len(out) + 1, AmountMinor: amount, BeneficiaryName: x.BeneficiaryName,
			AccountNumber: x.AccountNumber, BankCode: x.BankCode, Reference: x.Reference,
		})
	}
	return out, total, nil
}

func sameActor(a, b string) bool {
	return strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(b))
}

func invalid(msg string) error { return connect.NewError(connect.CodeInvalidArgument, errors.New(msg)) }

func mapErr(err error) error {
	var conn *connect.Error
	if errors.As(err, &conn) {
		return conn
	}
	if errors.Is(err, store.ErrNotFound) {
		return connect.NewError(connect.CodeNotFound, errors.New("batch not found"))
	}
	return connect.NewError(connect.CodeInternal, errors.New("internal error"))
}

func requestHash(m proto.Message) ([]byte, error) {
	b, err := proto.MarshalOptions{Deterministic: true}.Marshal(m)
	if err != nil {
		return nil, err
	}
	return b, nil
}

var batchStates = map[string]payoutsv1.BatchState{
	store.StateDraft:      payoutsv1.BatchState_BATCH_STATE_DRAFT,
	store.StateRunning:    payoutsv1.BatchState_BATCH_STATE_RUNNING,
	store.StateCompleted:  payoutsv1.BatchState_BATCH_STATE_COMPLETED,
	store.StateExceptions: payoutsv1.BatchState_BATCH_STATE_COMPLETED_WITH_EXCEPTIONS,
}

var itemStates = map[string]payoutsv1.ItemState{
	store.ItemPending:   payoutsv1.ItemState_ITEM_STATE_PENDING,
	store.ItemSent:      payoutsv1.ItemState_ITEM_STATE_SENT,
	store.ItemSettled:   payoutsv1.ItemState_ITEM_STATE_SETTLED,
	store.ItemFailed:    payoutsv1.ItemState_ITEM_STATE_FAILED,
	store.ItemUncertain: payoutsv1.ItemState_ITEM_STATE_UNCERTAIN,
}

func toProto(b store.Batch) *payoutsv1.Batch {
	out := &payoutsv1.Batch{
		BatchId: b.ID, PartyId: b.PartyID, FromAccountId: b.FromAccountID, Asset: b.Asset,
		TotalMinor: strconv.FormatInt(b.TotalMinor, 10), State: batchStates[b.State],
		CreatedBy: b.CreatedBy, ApprovedBy: b.ApprovedBy, Run: b.Run,
	}
	for _, it := range b.Items {
		out.Items = append(out.Items, &payoutsv1.Item{
			ItemKey: it.Key, AmountMinor: strconv.FormatInt(it.AmountMinor, 10), BeneficiaryName: it.BeneficiaryName,
			AccountNumber: it.AccountNumber, BankCode: it.BankCode, Reference: it.Reference,
			State: itemStates[it.State], PaymentId: it.PaymentID, Reason: it.Reason,
		})
	}
	return out
}
