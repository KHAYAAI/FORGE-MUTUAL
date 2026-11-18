// Package service is the payments API. It starts, reports, approves and repairs payments as Temporal workflows.
package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"regexp"
	"strings"

	"connectrpc.com/connect"
	enumspb "go.temporal.io/api/enums/v1"
	"go.temporal.io/api/serviceerror"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/converter"
	"google.golang.org/protobuf/proto"

	paymentsv1 "github.com/khayaai/forge-mutual/pkg/api/forge/payments/v1"
	"github.com/khayaai/forge-mutual/pkg/api/forge/payments/v1/paymentsv1connect"
	"github.com/khayaai/forge-mutual/pkg/uuid"
	"github.com/khayaai/forge-mutual/services/bank/payments-orchestrator/internal/payments"
)

// paymentsNamespace names the UUIDs derived from idempotency keys.
const paymentsNamespace = "5a7e0c2d-9f41-5b3e-8d6a-1c0b9e7f4a21"

// maxAmountMinor is the largest single payment (the policy limit for standard tier is higher; this bounds the
// input before policy is asked).
const maxAmountMinor = 1_000_000_000

var (
	amountRe   = regexp.MustCompile(`^[1-9][0-9]{0,9}$`)
	accountRe  = regexp.MustCompile(`^[0-9]{6,16}$`)
	bankRe     = regexp.MustCompile(`^[A-Za-z0-9]{1,12}$`)
	ledgerIDRe = regexp.MustCompile(`^[0-9a-f]{32}$`)
)

// Temporal is the part of the Temporal client the service uses. client.Client satisfies it.
type Temporal interface {
	ExecuteWorkflow(ctx context.Context, options client.StartWorkflowOptions, workflow interface{}, args ...interface{}) (client.WorkflowRun, error)
	SignalWorkflow(ctx context.Context, workflowID, runID, signalName string, arg interface{}) error
	QueryWorkflow(ctx context.Context, workflowID, runID, queryType string, args ...interface{}) (converter.EncodedValue, error)
}

// Config is the service's operating settings. Zero values take the workflow defaults.
type Config struct {
	TaskQueue       string
	AllowSimulation bool  // scenarios are honoured only when true; never in production
	PollSeconds     int64 // tests and demonstrations shorten this
	ApprovalSeconds int64
	MaxPolls        int
}

// Service implements PaymentsService.
type Service struct {
	tc  Temporal
	cfg Config
}

var (
	_ paymentsv1connect.PaymentsServiceHandler = (*Service)(nil)
	_ Temporal                                 = (client.Client)(nil)
)

// New returns the service.
func New(tc Temporal, cfg Config) *Service { return &Service{tc: tc, cfg: cfg} }

// CreatePayment implements PaymentsServiceHandler. A repeated key with the same request returns the same
// payment; the same key with a different request is refused.
func (s *Service) CreatePayment(ctx context.Context, req *connect.Request[paymentsv1.CreatePaymentRequest]) (*connect.Response[paymentsv1.CreatePaymentResponse], error) {
	m := req.Msg
	if m.IdempotencyKey == "" || len(m.IdempotencyKey) > 128 {
		return nil, invalid("idempotency_key must be 1 to 128 characters")
	}
	if _, err := uuid.Parse(m.PartyId); err != nil {
		return nil, invalid("party_id must be a canonical UUID")
	}
	if !ledgerIDRe.MatchString(m.FromAccountId) {
		return nil, invalid("from_account_id must be a ledger account id")
	}
	if !amountRe.MatchString(m.AmountMinor) || atoi(m.AmountMinor) > maxAmountMinor {
		return nil, invalid("amount_minor must be a positive integer of minor units, up to 1000000000")
	}
	if m.Asset != "ZAR" {
		return nil, invalid("domestic EFT carries ZAR only")
	}
	if strings.TrimSpace(m.BeneficiaryName) == "" || len(m.BeneficiaryName) > 100 {
		return nil, invalid("beneficiary_name is required, up to 100 characters")
	}
	if !accountRe.MatchString(m.AccountNumber) {
		return nil, invalid("account_number must be 6 to 16 digits")
	}
	if !bankRe.MatchString(m.BankCode) {
		return nil, invalid("bank_code is required")
	}
	if len(m.Reference) > 140 {
		return nil, invalid("reference is at most 140 characters")
	}
	if m.Actor == "" {
		return nil, invalid("actor is required")
	}
	if m.SimulationScenario != "" && !s.cfg.AllowSimulation {
		return nil, invalid("simulation_scenario is refused on this service")
	}

	id, err := uuid.NewV5(paymentsNamespace, m.IdempotencyKey)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	hash, err := requestHash(m)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	in := payments.Input{
		PaymentID: id, RequestHash: hash, PartyID: m.PartyId, FromAccountID: m.FromAccountId,
		AmountMinor: m.AmountMinor, Asset: m.Asset, BeneficiaryName: m.BeneficiaryName,
		AccountNumber: m.AccountNumber, BankCode: m.BankCode, Reference: m.Reference, Actor: m.Actor,
		Scenario: m.SimulationScenario, PollSeconds: s.cfg.PollSeconds, ApprovalSeconds: s.cfg.ApprovalSeconds,
		MaxPolls: s.cfg.MaxPolls,
	}
	// SDK note: WorkflowExecutionErrorWhenAlreadyStarted is required. Without it the SDK returns the existing
	// run with no error, and the duplicate-key check below never runs (see onboarding SPEC section 6).
	_, err = s.tc.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
		ID:                                       id,
		TaskQueue:                                s.cfg.TaskQueue,
		WorkflowIDReusePolicy:                    enumspb.WORKFLOW_ID_REUSE_POLICY_REJECT_DUPLICATE,
		WorkflowExecutionErrorWhenAlreadyStarted: true,
	}, payments.DomesticPayment, in)
	var started *serviceerror.WorkflowExecutionAlreadyStarted
	if errors.As(err, &started) {
		prev, err := s.status(ctx, id)
		if err != nil {
			return nil, err
		}
		if prev.RequestHash != hash {
			return nil, connect.NewError(connect.CodeAlreadyExists, errors.New("idempotency key reused with a different request"))
		}
		return connect.NewResponse(&paymentsv1.CreatePaymentResponse{PaymentId: id}), nil
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeUnavailable, errors.New("workflow service unavailable"))
	}
	return connect.NewResponse(&paymentsv1.CreatePaymentResponse{PaymentId: id}), nil
}

// GetPayment implements PaymentsServiceHandler.
func (s *Service) GetPayment(ctx context.Context, req *connect.Request[paymentsv1.GetPaymentRequest]) (*connect.Response[paymentsv1.GetPaymentResponse], error) {
	if _, err := uuid.Parse(req.Msg.PaymentId); err != nil {
		return nil, invalid("payment_id must be a canonical UUID")
	}
	st, err := s.status(ctx, req.Msg.PaymentId)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&paymentsv1.GetPaymentResponse{Status: toProto(st)}), nil
}

// ApprovePayment implements PaymentsServiceHandler. The approver must be someone other than the initiator.
func (s *Service) ApprovePayment(ctx context.Context, req *connect.Request[paymentsv1.ApprovePaymentRequest]) (*connect.Response[paymentsv1.ApprovePaymentResponse], error) {
	m := req.Msg
	if _, err := uuid.Parse(m.PaymentId); err != nil {
		return nil, invalid("payment_id must be a canonical UUID")
	}
	if m.Approver == "" {
		return nil, invalid("approver is required")
	}
	st, err := s.status(ctx, m.PaymentId)
	if err != nil {
		return nil, err
	}
	if st.InitiatedBy == m.Approver {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("the initiator cannot approve their own payment"))
	}
	if err := s.tc.SignalWorkflow(ctx, m.PaymentId, "", payments.SignalApproval, payments.Approval{Approve: m.Approve, Approver: m.Approver}); err != nil {
		return nil, signalErr(err)
	}
	return connect.NewResponse(&paymentsv1.ApprovePaymentResponse{}), nil
}

// ResolveRepair implements PaymentsServiceHandler. An operator states whether the rail confirmed the money moved.
func (s *Service) ResolveRepair(ctx context.Context, req *connect.Request[paymentsv1.ResolveRepairRequest]) (*connect.Response[paymentsv1.ResolveRepairResponse], error) {
	m := req.Msg
	if _, err := uuid.Parse(m.PaymentId); err != nil {
		return nil, invalid("payment_id must be a canonical UUID")
	}
	if m.Operator == "" {
		return nil, invalid("operator is required")
	}
	st, err := s.status(ctx, m.PaymentId)
	if err != nil {
		return nil, err
	}
	if st.State != payments.StateRepairRequired {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("payment is not awaiting repair"))
	}
	if err := s.tc.SignalWorkflow(ctx, m.PaymentId, "", payments.SignalRepair, payments.Repair{Settled: m.Settled, Operator: m.Operator}); err != nil {
		return nil, signalErr(err)
	}
	return connect.NewResponse(&paymentsv1.ResolveRepairResponse{}), nil
}

// status reads the workflow's state with its query handler.
func (s *Service) status(ctx context.Context, id string) (payments.Status, error) {
	val, err := s.tc.QueryWorkflow(ctx, id, "", payments.QueryStatus)
	if err != nil {
		return payments.Status{}, queryErr(err)
	}
	var st payments.Status
	if err := val.Get(&st); err != nil {
		return payments.Status{}, connect.NewError(connect.CodeInternal, errors.New("status unreadable"))
	}
	return st, nil
}

func queryErr(err error) error {
	var notFound *serviceerror.NotFound
	if errors.As(err, &notFound) {
		return connect.NewError(connect.CodeNotFound, errors.New("payment not found"))
	}
	return connect.NewError(connect.CodeUnavailable, errors.New("workflow service unavailable"))
}

func signalErr(err error) error {
	var notFound *serviceerror.NotFound
	if errors.As(err, &notFound) {
		return connect.NewError(connect.CodeNotFound, errors.New("payment not found"))
	}
	return connect.NewError(connect.CodeUnavailable, errors.New("workflow service unavailable"))
}

func toProto(st payments.Status) *paymentsv1.PaymentStatus {
	return &paymentsv1.PaymentStatus{
		PaymentId:   st.PaymentID,
		State:       stateOf(st.State),
		Reason:      st.Reason,
		RailStatus:  st.RailStatus,
		InitiatedBy: st.InitiatedBy,
		Steps:       st.Steps,
		AmountMinor: st.AmountMinor,
		Asset:       st.Asset,
	}
}

func stateOf(s string) paymentsv1.PaymentState {
	return paymentsv1.PaymentState(paymentsv1.PaymentState_value["PAYMENT_STATE_"+s])
}

func requestHash(m proto.Message) (string, error) {
	b, err := proto.MarshalOptions{Deterministic: true}.Marshal(m)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

func atoi(s string) int64 {
	var n int64
	for _, c := range s {
		n = n*10 + int64(c-'0')
		if n > 1<<40 {
			return n
		}
	}
	return n
}

func invalid(msg string) error { return connect.NewError(connect.CodeInvalidArgument, errors.New(msg)) }
