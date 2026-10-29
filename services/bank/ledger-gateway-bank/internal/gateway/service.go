package gateway

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/timestamppb"

	eventsv1 "github.com/khayaai/forge-mutual/pkg/api/forge/events/v1"
	ledgerv1 "github.com/khayaai/forge-mutual/pkg/api/forge/ledger/v1"
	"github.com/khayaai/forge-mutual/pkg/api/forge/ledger/v1/ledgerv1connect"
	"github.com/khayaai/forge-mutual/pkg/money"
	"github.com/khayaai/forge-mutual/services/bank/ledger-gateway-bank/internal/ledger"
)

// TopicTransferCommitted is the event published for every committed transfer leg.
const TopicTransferCommitted = "forge.bank.ledger.transfer.committed.v1"

const maxKeyLen = 128

// Service implements the LedgerService RPCs. It validates, applies idempotency, submits to the
// ledger and records outbox events. Authorisation is done by the interceptor, not here.
type Service struct {
	cfg   Config
	store ledger.Store
	repo  Repository
}

var _ ledgerv1connect.LedgerServiceHandler = (*Service)(nil)

// NewService wires the gateway to a ledger store and repository.
func NewService(cfg Config, store ledger.Store, repo Repository) *Service {
	return &Service{cfg: cfg, store: store, repo: repo}
}

// ---- RPCs ----

// CreateAccounts implements LedgerServiceHandler.
func (s *Service) CreateAccounts(ctx context.Context, req *connect.Request[ledgerv1.CreateAccountsRequest]) (*connect.Response[ledgerv1.CreateAccountsResponse], error) {
	m := req.Msg
	if err := s.checkEnvelope(m.IdempotencyKey, m.BusinessTxnId, m.EntityCode); err != nil {
		return nil, err
	}
	if len(m.Accounts) == 0 || len(m.Accounts) > s.cfg.MaxLegs {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("accounts: count out of range"))
	}
	resp, err := idempotent(ctx, s, req.Header().Get(CallerHeader), "CreateAccounts", m, len(m.Accounts),
		func(ids []ledger.ID) (*ledgerv1.CreateAccountsResponse, []DirectoryEntry, []OutboxEvent, error) {
			return s.doCreateAccounts(ctx, m, ids)
		})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

// CreateTransfers implements LedgerServiceHandler.
func (s *Service) CreateTransfers(ctx context.Context, req *connect.Request[ledgerv1.CreateTransfersRequest]) (*connect.Response[ledgerv1.CreateTransfersResponse], error) {
	m := req.Msg
	if err := s.checkEnvelope(m.IdempotencyKey, m.BusinessTxnId, m.EntityCode); err != nil {
		return nil, err
	}
	if len(m.Legs) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("legs: at least one leg required"))
	}
	if len(m.Legs) > s.cfg.MaxLegs {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("legs: too many"))
	}
	resp, err := idempotent(ctx, s, req.Header().Get(CallerHeader), "CreateTransfers", m, len(m.Legs),
		func(ids []ledger.ID) (*ledgerv1.CreateTransfersResponse, []DirectoryEntry, []OutboxEvent, error) {
			return s.doCreateTransfers(ctx, m, ids)
		})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

// PostPending implements LedgerServiceHandler.
func (s *Service) PostPending(ctx context.Context, req *connect.Request[ledgerv1.PostPendingRequest]) (*connect.Response[ledgerv1.PostPendingResponse], error) {
	m := req.Msg
	if err := s.checkEnvelope(m.IdempotencyKey, m.BusinessTxnId, m.EntityCode); err != nil {
		return nil, err
	}
	if len(m.PendingTransferIds) == 0 || len(m.PendingTransferIds) > s.cfg.MaxLegs {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("pending_transfer_ids: count out of range"))
	}
	resp, err := idempotent(ctx, s, req.Header().Get(CallerHeader), "PostPending", m, len(m.PendingTransferIds),
		func(ids []ledger.ID) (*ledgerv1.PostPendingResponse, []DirectoryEntry, []OutboxEvent, error) {
			r, ev, err := s.doResolvePending(ctx, m.BusinessTxnId, m.PendingTransferIds, ids, true)
			if err != nil {
				return nil, nil, nil, err
			}
			return asPostPending(r), nil, ev, nil
		})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

// VoidPending implements LedgerServiceHandler.
func (s *Service) VoidPending(ctx context.Context, req *connect.Request[ledgerv1.VoidPendingRequest]) (*connect.Response[ledgerv1.VoidPendingResponse], error) {
	m := req.Msg
	if err := s.checkEnvelope(m.IdempotencyKey, m.BusinessTxnId, m.EntityCode); err != nil {
		return nil, err
	}
	if len(m.PendingTransferIds) == 0 || len(m.PendingTransferIds) > s.cfg.MaxLegs {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("pending_transfer_ids: count out of range"))
	}
	resp, err := idempotent(ctx, s, req.Header().Get(CallerHeader), "VoidPending", m, len(m.PendingTransferIds),
		func(ids []ledger.ID) (*ledgerv1.VoidPendingResponse, []DirectoryEntry, []OutboxEvent, error) {
			r, ev, err := s.doResolvePending(ctx, m.BusinessTxnId, m.PendingTransferIds, ids, false)
			if err != nil {
				return nil, nil, nil, err
			}
			return asVoidPending(r), nil, ev, nil
		})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

// GetBalances implements LedgerServiceHandler. Only accounts in this entity are returned.
func (s *Service) GetBalances(ctx context.Context, req *connect.Request[ledgerv1.GetBalancesRequest]) (*connect.Response[ledgerv1.GetBalancesResponse], error) {
	m := req.Msg
	if m.EntityCode != s.cfg.Entity {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("entity_code must be %s", s.cfg.Entity))
	}
	ids, err := parseIDList(m.AccountIds)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	dir, err := s.repo.Directory(ctx, ids)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	var inEntity []ledger.ID
	for _, id := range ids {
		if e, ok := dir[id]; ok && e.Entity == s.cfg.Entity {
			inEntity = append(inEntity, id)
		}
	}
	accs, err := s.store.LookupAccounts(ctx, inEntity)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnavailable, err)
	}
	out := &ledgerv1.GetBalancesResponse{}
	for _, a := range accs {
		out.Balances = append(out.Balances, &ledgerv1.AccountBalance{
			AccountId:      a.ID.String(),
			Asset:          dir[a.ID].Asset,
			DebitsPending:  fmt.Sprint(a.DebitsPending),
			DebitsPosted:   fmt.Sprint(a.DebitsPosted),
			CreditsPending: fmt.Sprint(a.CreditsPending),
			CreditsPosted:  fmt.Sprint(a.CreditsPosted),
			AvailableMinor: a.Available().String(),
		})
	}
	return connect.NewResponse(out), nil
}

// LookupTransfers implements LedgerServiceHandler. A transfer is returned only when both its
// accounts belong to this entity.
func (s *Service) LookupTransfers(ctx context.Context, req *connect.Request[ledgerv1.LookupTransfersRequest]) (*connect.Response[ledgerv1.LookupTransfersResponse], error) {
	m := req.Msg
	if m.EntityCode != s.cfg.Entity {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("entity_code must be %s", s.cfg.Entity))
	}
	ids, err := parseIDList(m.TransferIds)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	trs, err := s.store.LookupTransfers(ctx, ids)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnavailable, err)
	}
	var accIDs []ledger.ID
	for _, t := range trs {
		accIDs = append(accIDs, t.DebitAccountID, t.CreditAccountID)
	}
	dir, err := s.repo.Directory(ctx, accIDs)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := &ledgerv1.LookupTransfersResponse{}
	for _, t := range trs {
		d, okD := dir[t.DebitAccountID]
		c, okC := dir[t.CreditAccountID]
		if !okD || !okC || d.Entity != s.cfg.Entity || c.Entity != s.cfg.Entity {
			continue
		}
		out.Transfers = append(out.Transfers, &ledgerv1.TransferRecord{
			TransferId:      t.ID.String(),
			DebitAccountId:  t.DebitAccountID.String(),
			CreditAccountId: t.CreditAccountID.String(),
			Asset:           d.Asset,
			AmountMinor:     fmt.Sprint(t.Amount),
			Code:            uint32(t.Code),
			Pending:         t.Pending,
			PendingId:       idOrEmpty(t.PendingID),
		})
	}
	return connect.NewResponse(out), nil
}

// ---- business logic ----

func (s *Service) doCreateAccounts(ctx context.Context, m *ledgerv1.CreateAccountsRequest, ids []ledger.ID) (*ledgerv1.CreateAccountsResponse, []DirectoryEntry, []OutboxEvent, error) {
	reject := func(code string) (*ledgerv1.CreateAccountsResponse, []DirectoryEntry, []OutboxEvent, error) {
		return &ledgerv1.CreateAccountsResponse{Status: ledgerv1.Status_STATUS_REJECTED, RejectionCodes: []string{code}}, nil, nil, nil
	}
	accounts := make([]ledger.Account, len(m.Accounts))
	var dir []DirectoryEntry
	for i, spec := range m.Accounts {
		ac, ok := s.cfg.Assets[spec.Asset]
		if !ok {
			return reject(CodeAssetNotAllowed)
		}
		if spec.Code > 0xffff || !s.cfg.AccountCodes[uint16(spec.Code)] {
			return reject(CodeCodeNotAllowed)
		}
		owner, err := parseOptionalUUID(spec.OwnerPartyId)
		if err != nil {
			return nil, nil, nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
		if len(spec.ProductId) > 64 {
			return nil, nil, nil, connect.NewError(connect.CodeInvalidArgument, errors.New("product_id too long"))
		}
		accounts[i] = ledger.Account{
			ID:                         ids[i],
			Ledger:                     ac.Ledger,
			Code:                       uint16(spec.Code),
			DebitsMustNotExceedCredits: spec.DebitsMustNotExceedCredits,
			UserData128:                owner,
			Linked:                     i < len(m.Accounts)-1,
		}
		dir = append(dir, DirectoryEntry{
			AccountID:    ids[i],
			Entity:       s.cfg.Entity,
			Asset:        spec.Asset,
			Ledger:       ac.Ledger,
			Code:         uint16(spec.Code),
			OwnerPartyID: spec.OwnerPartyId,
			ProductID:    spec.ProductId,
			Insured:      spec.Insured,
		})
	}
	outcomes, err := s.store.CreateAccounts(ctx, accounts)
	if err != nil {
		return nil, nil, nil, connect.NewError(connect.CodeUnavailable, err)
	}
	for _, o := range outcomes {
		switch {
		case o.Reason.IsSuccess():
		case o.Reason == ledger.ReasonLinkedEventFailed:
		default:
			return reject(CodeAccountConflict)
		}
	}
	ids2 := make([]string, len(ids))
	for i, id := range ids {
		ids2[i] = id.String()
	}
	return &ledgerv1.CreateAccountsResponse{Status: ledgerv1.Status_STATUS_OK, AccountIds: ids2}, dir, nil, nil
}

func (s *Service) doCreateTransfers(ctx context.Context, m *ledgerv1.CreateTransfersRequest, ids []ledger.ID) (*ledgerv1.CreateTransfersResponse, []DirectoryEntry, []OutboxEvent, error) {
	reject := func(code string) (*ledgerv1.CreateTransfersResponse, []DirectoryEntry, []OutboxEvent, error) {
		return &ledgerv1.CreateTransfersResponse{Status: ledgerv1.Status_STATUS_REJECTED, RejectionCodes: []string{code}}, nil, nil, nil
	}
	if m.Pending && m.PendingTimeoutSeconds == 0 {
		return reject(CodeTimeoutRequired)
	}
	if !m.Pending && m.PendingTimeoutSeconds != 0 {
		return reject(CodeTimeoutNotPending)
	}
	if m.Pending && m.PendingTimeoutSeconds > s.cfg.MaxPendingTimeout {
		return reject(CodeTimeoutLimit)
	}

	// Syntax errors were rejected before this point; business checks below.
	ref, err := parseUUID(m.BusinessTxnId)
	if err != nil {
		return nil, nil, nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	type legInfo struct{ debit, credit ledger.ID }
	legs := make([]legInfo, len(m.Legs))
	transfers := make([]ledger.Transfer, len(m.Legs))
	for i, leg := range m.Legs {
		if leg.Amount == nil {
			return nil, nil, nil, connect.NewError(connect.CodeInvalidArgument, errors.New("leg amount required"))
		}
		ac, ok := s.cfg.Assets[leg.Amount.Asset]
		if !ok {
			return reject(CodeAssetNotAllowed)
		}
		if leg.Code > 0xffff || !s.cfg.TransferCodes[uint16(leg.Code)] {
			return reject(CodeCodeNotAllowed)
		}
		amount, err := parseAmount(leg.Amount)
		if err != nil {
			return nil, nil, nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
		if amount == 0 {
			return reject(CodeInvalidAmount)
		}
		if amount > ac.MaxAmountMinor {
			return reject(CodeAmountLimitExceeded)
		}
		debit, credit, err := parseLegAccounts(leg)
		if err != nil {
			return nil, nil, nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
		if debit == credit {
			return reject(CodeSameAccount)
		}
		legs[i] = legInfo{debit, credit}
		if code := s.checkAccountsInEntity(ctx, []ledger.ID{debit, credit}, leg.Amount.Asset); code != "" {
			return reject(code)
		}
		transfers[i] = ledger.Transfer{
			ID:              ids[i],
			DebitAccountID:  debit,
			CreditAccountID: credit,
			Amount:          amount,
			UserData128:     ref,
			Timeout:         m.PendingTimeoutSeconds,
			Ledger:          ac.Ledger,
			Code:            uint16(leg.Code),
			Pending:         m.Pending,
			Linked:          i < len(m.Legs)-1,
		}
	}

	outcomes, err := s.store.CreateTransfers(ctx, transfers)
	if err != nil {
		return nil, nil, nil, connect.NewError(connect.CodeUnavailable, err)
	}
	if code, failed := firstFailure(outcomes); failed {
		return reject(code)
	}
	events, err := s.transferEvents(ctx, m.BusinessTxnId, ids)
	if err != nil {
		return nil, nil, nil, connect.NewError(connect.CodeUnavailable, err)
	}
	return &ledgerv1.CreateTransfersResponse{Status: ledgerv1.Status_STATUS_OK, TransferIds: idStrings(ids)}, nil, events, nil
}

func (s *Service) doResolvePending(ctx context.Context, businessTxnID string, pendingIDs []string, ids []ledger.ID, post bool) (*resolveResult, []OutboxEvent, error) {
	ref, err := parseUUID(businessTxnID)
	if err != nil {
		return nil, nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	pIDs, err := parseIDList(pendingIDs)
	if err != nil {
		return nil, nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	found, err := s.store.LookupTransfers(ctx, pIDs)
	if err != nil {
		return nil, nil, connect.NewError(connect.CodeUnavailable, err)
	}
	byID := make(map[ledger.ID]ledger.Transfer, len(found))
	for _, t := range found {
		byID[t.ID] = t
	}
	transfers := make([]ledger.Transfer, len(pIDs))
	for i, pid := range pIDs {
		pt, ok := byID[pid]
		if !ok || !pt.Pending {
			return &resolveResult{reject: CodePendingNotFound}, nil, nil
		}
		if code := s.checkAccountsInEntity(ctx, []ledger.ID{pt.DebitAccountID, pt.CreditAccountID}, ""); code != "" {
			return &resolveResult{reject: code}, nil, nil
		}
		transfers[i] = ledger.Transfer{
			ID:              ids[i],
			DebitAccountID:  pt.DebitAccountID,
			CreditAccountID: pt.CreditAccountID,
			Amount:          pt.Amount,
			PendingID:       pt.ID,
			UserData128:     ref,
			Ledger:          pt.Ledger,
			Code:            pt.Code,
			PostPending:     post,
			VoidPending:     !post,
			Linked:          i < len(pIDs)-1,
		}
	}
	outcomes, err := s.store.CreateTransfers(ctx, transfers)
	if err != nil {
		return nil, nil, connect.NewError(connect.CodeUnavailable, err)
	}
	if code, failed := firstFailure(outcomes); failed {
		return &resolveResult{reject: code}, nil, nil
	}
	events, err := s.transferEvents(ctx, businessTxnID, ids)
	if err != nil {
		return nil, nil, connect.NewError(connect.CodeUnavailable, err)
	}
	return &resolveResult{ids: idStrings(ids)}, events, nil
}

// resolveResult carries either the new transfer IDs or a rejection code.
type resolveResult struct {
	ids    []string
	reject string
}

func asPostPending(r *resolveResult) *ledgerv1.PostPendingResponse {
	if r.reject != "" {
		return &ledgerv1.PostPendingResponse{Status: ledgerv1.Status_STATUS_REJECTED, RejectionCodes: []string{r.reject}}
	}
	return &ledgerv1.PostPendingResponse{Status: ledgerv1.Status_STATUS_OK, TransferIds: r.ids}
}

func asVoidPending(r *resolveResult) *ledgerv1.VoidPendingResponse {
	if r.reject != "" {
		return &ledgerv1.VoidPendingResponse{Status: ledgerv1.Status_STATUS_REJECTED, RejectionCodes: []string{r.reject}}
	}
	return &ledgerv1.VoidPendingResponse{Status: ledgerv1.Status_STATUS_OK, TransferIds: r.ids}
}

// checkAccountsInEntity returns a rejection code, or "" if every account is in this entity.
// If asset is non-empty, each account must also hold that asset.
func (s *Service) checkAccountsInEntity(ctx context.Context, ids []ledger.ID, asset string) string {
	dir, err := s.repo.Directory(ctx, ids)
	if err != nil {
		return CodeAccountNotFound
	}
	for _, id := range ids {
		d, ok := dir[id]
		if !ok {
			return CodeAccountNotFound
		}
		if d.Entity != s.cfg.Entity {
			return CodeAccountNotInEntity
		}
		if asset != "" && d.Asset != asset {
			return CodeAssetMismatch
		}
	}
	return ""
}

// transferEvents builds one committed-transfer event per ID, read back from the ledger for
// the timestamp.
func (s *Service) transferEvents(ctx context.Context, businessTxnID string, ids []ledger.ID) ([]OutboxEvent, error) {
	trs, err := s.store.LookupTransfers(ctx, ids)
	if err != nil {
		return nil, err
	}
	byID := make(map[ledger.ID]ledger.Transfer, len(trs))
	for _, t := range trs {
		byID[t.ID] = t
	}
	now := s.cfg.Clock()
	var out []OutboxEvent
	for _, id := range ids {
		t, ok := byID[id]
		if !ok {
			return nil, fmt.Errorf("gateway: committed transfer %s not found on lookup", id)
		}
		asset := assetForLedger(s.cfg.Assets, t.Ledger)
		payload, err := proto.Marshal(&eventsv1.TransferCommitted{
			TransferId:      id.String(),
			BusinessTxnId:   businessTxnID,
			DebitAccountId:  t.DebitAccountID.String(),
			CreditAccountId: t.CreditAccountID.String(),
			Asset:           asset,
			AmountMinor:     fmt.Sprint(t.Amount),
			Code:            uint32(t.Code),
			Pending:         t.Pending,
			PendingId:       idOrEmpty(t.PendingID),
			LedgerTimestamp: t.Timestamp,
		})
		if err != nil {
			return nil, err
		}
		eventID, err := newUUIDv4()
		if err != nil {
			return nil, err
		}
		env, err := proto.Marshal(&eventsv1.EventEnvelope{
			EventId:       eventID,
			EventType:     TopicTransferCommitted,
			SchemaVersion: 1,
			EntityCode:    s.cfg.Entity,
			CorrelationId: businessTxnID,
			CausationId:   id.String(),
			Producer:      s.cfg.Producer,
			OccurredAt:    timestamppb.New(now),
			Payload:       payload,
		})
		if err != nil {
			return nil, err
		}
		out = append(out, OutboxEvent{EventID: eventID, Topic: TopicTransferCommitted, Key: []byte(businessTxnID), Payload: env})
	}
	return out, nil
}

// ---- idempotency ----

// idempotent runs an idempotent request. A committed record returns its stored response with
// STATUS_REPLAYED. Otherwise it runs the step with the record's IDs and stores the outcome.
// A step error (for example the ledger is unreachable) leaves the record PENDING, so a retry
// with the same key resubmits with the same IDs.
func idempotent[R proto.Message](
	ctx context.Context,
	s *Service,
	caller, method string,
	req proto.Message,
	idCount int,
	step func(ids []ledger.ID) (R, []DirectoryEntry, []OutboxEvent, error),
) (R, error) {
	var zero R
	key := idempotencyKey(req)
	if key == "" {
		return zero, connect.NewError(connect.CodeInvalidArgument, errors.New("idempotency_key required"))
	}
	hash, err := requestHash(method, req)
	if err != nil {
		return zero, connect.NewError(connect.CodeInternal, err)
	}
	proposed, err := newIDs(s.cfg.Clock(), idCount)
	if err != nil {
		return zero, connect.NewError(connect.CodeInternal, err)
	}
	rec, created, err := s.repo.Begin(ctx, caller, key, hash, proposed)
	if errors.Is(err, ErrConflict) {
		return zero, connect.NewError(connect.CodeAlreadyExists, err)
	}
	if err != nil {
		return zero, connect.NewError(connect.CodeUnavailable, err)
	}
	if !created && rec.Status == StatusCommitted {
		return replay[R](rec.Result)
	}
	if len(rec.IDs) != idCount {
		return zero, connect.NewError(connect.CodeInternal, errors.New("stored id count does not match request"))
	}

	resp, dir, events, err := step(rec.IDs)
	if err != nil {
		return zero, err
	}
	result, err := proto.Marshal(resp)
	if err != nil {
		return zero, connect.NewError(connect.CodeInternal, err)
	}
	if err := s.repo.Complete(ctx, caller, key, result, dir, events); err != nil {
		return zero, connect.NewError(connect.CodeUnavailable, err)
	}
	return resp, nil
}

func replay[R proto.Message](stored []byte) (R, error) {
	var zero R
	msg := newResponse[R]()
	if err := proto.Unmarshal(stored, msg); err != nil {
		return zero, connect.NewError(connect.CodeInternal, err)
	}
	setStatus(msg, ledgerv1.Status_STATUS_REPLAYED)
	return msg.(R), nil
}

// newResponse returns an empty response of type R, chosen by the Go type parameter.
func newResponse[R proto.Message]() proto.Message {
	var zero R
	switch any(zero).(type) {
	case *ledgerv1.CreateAccountsResponse:
		return &ledgerv1.CreateAccountsResponse{}
	case *ledgerv1.CreateTransfersResponse:
		return &ledgerv1.CreateTransfersResponse{}
	case *ledgerv1.PostPendingResponse:
		return &ledgerv1.PostPendingResponse{}
	case *ledgerv1.VoidPendingResponse:
		return &ledgerv1.VoidPendingResponse{}
	}
	panic(fmt.Sprintf("gateway: no response type for %T", zero))
}

func setStatus(m proto.Message, st ledgerv1.Status) {
	r := m.ProtoReflect()
	fd := r.Descriptor().Fields().ByName("status")
	if fd != nil {
		r.Set(fd, protoreflect.ValueOfEnum(protoreflect.EnumNumber(st)))
	}
}

func idempotencyKey(req proto.Message) string {
	r := req.ProtoReflect()
	fd := r.Descriptor().Fields().ByName("idempotency_key")
	return r.Get(fd).String()
}

// requestHash is SHA-256 over the method name and the deterministic encoding of the request.
func requestHash(method string, req proto.Message) ([]byte, error) {
	b, err := proto.MarshalOptions{Deterministic: true}.Marshal(req)
	if err != nil {
		return nil, err
	}
	h := sha256.New()
	h.Write([]byte(method))
	h.Write([]byte{0})
	h.Write(b)
	return h.Sum(nil), nil
}

// newIDs generates n time-ordered ledger IDs.
func newIDs(now time.Time, n int) ([]ledger.ID, error) {
	out := make([]ledger.ID, n)
	for i := range out {
		id, err := ledger.NewID(now)
		if err != nil {
			return nil, err
		}
		out[i] = id
	}
	return out, nil
}

// ---- validation helpers ----

func (s *Service) checkEnvelope(key, businessTxnID, entity string) error {
	if key == "" || len(key) > maxKeyLen {
		return connect.NewError(connect.CodeInvalidArgument, errors.New("idempotency_key must be 1 to 128 characters"))
	}
	if _, err := parseUUID(businessTxnID); err != nil {
		return connect.NewError(connect.CodeInvalidArgument, err)
	}
	if entity != s.cfg.Entity {
		return connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("entity_code must be %s", s.cfg.Entity))
	}
	return nil
}

func parseAmount(m *ledgerv1.Money) (uint64, error) {
	mon, err := money.Parse(m.Asset, m.AmountMinor)
	if err != nil {
		return 0, err
	}
	if err := mon.NonNegative(); err != nil {
		return 0, err
	}
	return uint64(mon.Minor()), nil
}

func parseLegAccounts(leg *ledgerv1.TransferLeg) (ledger.ID, ledger.ID, error) {
	d, err := ledger.ParseID(leg.DebitAccountId)
	if err != nil {
		return ledger.ZeroID, ledger.ZeroID, err
	}
	c, err := ledger.ParseID(leg.CreditAccountId)
	if err != nil {
		return ledger.ZeroID, ledger.ZeroID, err
	}
	return d, c, nil
}

func parseIDList(ss []string) ([]ledger.ID, error) {
	out := make([]ledger.ID, len(ss))
	for i, s := range ss {
		id, err := ledger.ParseID(s)
		if err != nil {
			return nil, err
		}
		out[i] = id
	}
	return out, nil
}

// parseUUID parses a canonical 8-4-4-4-12 UUID into 16 bytes.
func parseUUID(s string) ([16]byte, error) {
	var out [16]byte
	if len(s) != 36 || s[8] != '-' || s[13] != '-' || s[18] != '-' || s[23] != '-' {
		return out, fmt.Errorf("business_txn_id: not a UUID: %q", s)
	}
	hexPart := strings.ReplaceAll(s, "-", "")
	id, err := ledger.ParseID(hexPart)
	if err != nil {
		return out, fmt.Errorf("business_txn_id: not a UUID: %q", s)
	}
	return [16]byte(id), nil
}

// parseOptionalUUID treats "" as the zero UUID (system accounts have no owner).
func parseOptionalUUID(s string) ([16]byte, error) {
	if s == "" {
		return [16]byte{}, nil
	}
	return parseUUID(s)
}

func firstFailure(outcomes []ledger.Outcome) (string, bool) {
	for _, o := range outcomes {
		if o.Reason.IsSuccess() || o.Reason == ledger.ReasonLinkedEventFailed {
			continue
		}
		return rejectionCode(o.Reason), true
	}
	return "", false
}

func assetForLedger(assets map[string]AssetConfig, l uint32) string {
	for name, ac := range assets {
		if ac.Ledger == l {
			return name
		}
	}
	return ""
}

func idStrings(ids []ledger.ID) []string {
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = id.String()
	}
	return out
}

func idOrEmpty(id ledger.ID) string {
	if id.IsZero() {
		return ""
	}
	return id.String()
}

func newUUIDv4() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	var buf bytes.Buffer
	for i, x := range b {
		if i == 4 || i == 6 || i == 8 || i == 10 {
			buf.WriteByte('-')
		}
		fmt.Fprintf(&buf, "%02x", x)
	}
	return buf.String(), nil
}

// CallerHeader carries the caller identity in local development. Production uses the mTLS identity.
const CallerHeader = "X-Forge-Caller"
