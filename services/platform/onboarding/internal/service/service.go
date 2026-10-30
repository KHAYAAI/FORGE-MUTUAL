// Package service is the onboarding API: it starts and reports workflows, and receives KYC webhooks.
package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"regexp"
	"strings"

	"connectrpc.com/connect"
	enumspb "go.temporal.io/api/enums/v1"
	"go.temporal.io/api/serviceerror"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/converter"
	"google.golang.org/protobuf/proto"

	onboardingv1 "github.com/khayaai/forge-mutual/pkg/api/forge/onboarding/v1"
	"github.com/khayaai/forge-mutual/pkg/api/forge/onboarding/v1/onboardingv1connect"
	"github.com/khayaai/forge-mutual/pkg/uuid"
	"github.com/khayaai/forge-mutual/services/platform/onboarding/internal/kyc"
	"github.com/khayaai/forge-mutual/services/platform/onboarding/internal/onboarding"
)

// onboardingNamespace names the UUIDs derived from idempotency keys (a fixed, project-specific value).
const onboardingNamespace = "2c1f8d3e-6b7a-5f4e-9a0b-7d6c5b4a3e2f"

const maxDirectors = 20

var (
	countryRe  = regexp.MustCompile(`^[A-Z]{2}$`)
	nationalRe = regexp.MustCompile(`^[A-Za-z0-9]{6,20}$`)
)

// Temporal is the part of the Temporal client the service uses. client.Client satisfies it.
type Temporal interface {
	ExecuteWorkflow(ctx context.Context, options client.StartWorkflowOptions, workflow interface{}, args ...interface{}) (client.WorkflowRun, error)
	SignalWorkflow(ctx context.Context, workflowID, runID, signalName string, arg interface{}) error
	QueryWorkflow(ctx context.Context, workflowID, runID, queryType string, args ...interface{}) (converter.EncodedValue, error)
}

// Service implements OnboardingService and serves the KYC webhook.
type Service struct {
	tc        Temporal
	kyc       kyc.Provider
	taskQueue string
}

var (
	_ onboardingv1connect.OnboardingServiceHandler = (*Service)(nil)
	_ Temporal                                     = (client.Client)(nil) // the real client must satisfy the interface
)

// New returns the service.
func New(tc Temporal, provider kyc.Provider, taskQueue string) *Service {
	return &Service{tc: tc, kyc: provider, taskQueue: taskQueue}
}

// StartBusinessOnboarding implements OnboardingServiceHandler. The onboarding ID is derived from the key,
// so a repeated request names the same workflow. The same key with a different request is refused.
func (s *Service) StartBusinessOnboarding(ctx context.Context, req *connect.Request[onboardingv1.StartBusinessOnboardingRequest]) (*connect.Response[onboardingv1.StartBusinessOnboardingResponse], error) {
	m := req.Msg
	if m.IdempotencyKey == "" || len(m.IdempotencyKey) > 128 {
		return nil, invalid("idempotency_key must be 1 to 128 characters")
	}
	if strings.TrimSpace(m.LegalName) == "" || len(m.LegalName) > 200 {
		return nil, invalid("legal_name must be 1 to 200 characters")
	}
	if strings.TrimSpace(m.RegistrationNumber) == "" || len(m.RegistrationNumber) > 32 {
		return nil, invalid("registration_number is required, up to 32 characters")
	}
	if !countryRe.MatchString(m.Country) {
		return nil, invalid("country must be an ISO 3166-1 alpha-2 code")
	}
	if m.SectorCode == "" || len(m.SectorCode) > 64 {
		return nil, invalid("sector_code is required, up to 64 characters")
	}
	if len(m.Directors) == 0 || len(m.Directors) > maxDirectors {
		return nil, invalid("directors must hold 1 to 20 entries")
	}
	if m.Actor == "" {
		return nil, invalid("actor is required")
	}
	var directors []onboarding.DirectorInput
	for _, d := range m.Directors {
		if strings.TrimSpace(d.LegalName) == "" || !nationalRe.MatchString(strings.ReplaceAll(d.NationalId, " ", "")) {
			return nil, invalid("each director needs a legal name and a national ID of 6 to 20 characters")
		}
		if d.OwnershipBps > 10000 {
			return nil, invalid("ownership_bps must be 0 to 10000")
		}
		if d.IsUbo && d.OwnershipBps == 0 {
			return nil, invalid("a UBO must hold a share (ownership_bps > 0)")
		}
		directors = append(directors, onboarding.DirectorInput{
			LegalName: d.LegalName, NationalID: d.NationalId, OwnershipBps: d.OwnershipBps, IsUBO: d.IsUbo,
		})
	}

	id, err := uuid.NewV5(onboardingNamespace, m.IdempotencyKey)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	hash, err := requestHash(m)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	input := onboarding.Input{
		OnboardingID: id, RequestHash: hash, LegalName: m.LegalName, RegistrationNumber: m.RegistrationNumber,
		Country: m.Country, SectorCode: m.SectorCode, Directors: directors, Actor: m.Actor,
	}
	// WorkflowExecutionErrorWhenAlreadyStarted is required. Without it the SDK swallows the
	// already-started error and returns a handle to the existing run, so the duplicate-key check
	// below would never run and a changed request under the same key would be accepted silently.
	_, err = s.tc.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
		ID:                                       id,
		TaskQueue:                                s.taskQueue,
		WorkflowIDReusePolicy:                    enumspb.WORKFLOW_ID_REUSE_POLICY_REJECT_DUPLICATE,
		WorkflowExecutionErrorWhenAlreadyStarted: true,
	}, onboarding.OnboardBusiness, input)
	var started *serviceerror.WorkflowExecutionAlreadyStarted
	if errors.As(err, &started) {
		// The key was used before. The same request gets the same answer; a different one is refused.
		prev, err := s.status(ctx, id)
		if err != nil {
			return nil, err
		}
		if prev.RequestHash != hash {
			return nil, connect.NewError(connect.CodeAlreadyExists, errors.New("idempotency key reused with a different request"))
		}
		return connect.NewResponse(&onboardingv1.StartBusinessOnboardingResponse{OnboardingId: id}), nil
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeUnavailable, errors.New("workflow service unavailable"))
	}
	return connect.NewResponse(&onboardingv1.StartBusinessOnboardingResponse{OnboardingId: id}), nil
}

// GetOnboarding implements OnboardingServiceHandler.
func (s *Service) GetOnboarding(ctx context.Context, req *connect.Request[onboardingv1.GetOnboardingRequest]) (*connect.Response[onboardingv1.GetOnboardingResponse], error) {
	if _, err := uuid.Parse(req.Msg.OnboardingId); err != nil {
		return nil, invalid("onboarding_id must be a canonical UUID")
	}
	st, err := s.status(ctx, req.Msg.OnboardingId)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&onboardingv1.GetOnboardingResponse{Status: toProto(st)}), nil
}

// status reads the workflow's state with its query handler.
func (s *Service) status(ctx context.Context, id string) (onboarding.Status, error) {
	val, err := s.tc.QueryWorkflow(ctx, id, "", onboarding.QueryStatus)
	var notFound *serviceerror.NotFound
	if errors.As(err, &notFound) {
		return onboarding.Status{}, connect.NewError(connect.CodeNotFound, errors.New("onboarding not found"))
	}
	if err != nil {
		return onboarding.Status{}, connect.NewError(connect.CodeUnavailable, errors.New("workflow service unavailable"))
	}
	var st onboarding.Status
	if err := val.Get(&st); err != nil {
		return onboarding.Status{}, connect.NewError(connect.CodeInternal, errors.New("status unreadable"))
	}
	return st, nil
}

// WebhookHandler receives a provider's verification result. It authenticates the body with the
// provider's signature before it signals the workflow. Unsigned or altered bodies are refused.
func (s *Service) WebhookHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		body, err := io.ReadAll(io.LimitReader(r.Body, 64<<10))
		if err != nil {
			http.Error(w, "unreadable body", http.StatusBadRequest)
			return
		}
		ev, err := s.kyc.VerifyWebhook(body, r.Header.Get("X-Forge-Signature"))
		switch {
		case errors.Is(err, kyc.ErrSignature):
			http.Error(w, "signature", http.StatusUnauthorized)
			return
		case errors.Is(err, kyc.ErrNotConfigured):
			http.Error(w, "provider not configured", http.StatusServiceUnavailable)
			return
		case err != nil:
			http.Error(w, "invalid webhook", http.StatusBadRequest)
			return
		}
		err = s.tc.SignalWorkflow(r.Context(), ev.OnboardingID, "", onboarding.SignalKYCResult, onboarding.KYCSignal{
			SubjectPartyID: ev.SubjectID, Verdict: ev.Verdict, SessionID: ev.SessionID,
		})
		var notFound *serviceerror.NotFound
		if errors.As(err, &notFound) {
			http.Error(w, "unknown onboarding", http.StatusNotFound)
			return
		}
		if err != nil {
			http.Error(w, "workflow service unavailable", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusAccepted)
	})
}

// ---- helpers ----

func toProto(st onboarding.Status) *onboardingv1.OnboardingStatus {
	out := &onboardingv1.OnboardingStatus{
		OnboardingId:   st.OnboardingID,
		State:          stateOf(st.State),
		CompanyPartyId: st.CompanyPartyID,
		PersonPartyIds: st.PersonPartyIDs,
		AccountIds:     st.AccountIDs,
		Tier:           st.Tier,
		DeclineReason:  st.DeclineReason,
		Steps:          st.Steps,
	}
	return out
}

func stateOf(s string) onboardingv1.OnboardingState {
	return onboardingv1.OnboardingState(onboardingv1.OnboardingState_value["ONBOARDING_STATE_"+s])
}

func requestHash(m proto.Message) (string, error) {
	b, err := proto.MarshalOptions{Deterministic: true}.Marshal(m)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

func invalid(msg string) error { return connect.NewError(connect.CodeInvalidArgument, errors.New(msg)) }
