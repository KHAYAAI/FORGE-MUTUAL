// Package service is the onboarding API: it starts and reports workflows, and receives KYC webhooks.
package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"time"

	"connectrpc.com/connect"
	enumspb "go.temporal.io/api/enums/v1"
	"go.temporal.io/api/serviceerror"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/converter"
	"google.golang.org/protobuf/proto"

	onboardingv1 "github.com/khayaai/forge-mutual/pkg/api/forge/onboarding/v1"
	"github.com/khayaai/forge-mutual/pkg/api/forge/onboarding/v1/onboardingv1connect"
	"github.com/khayaai/forge-mutual/pkg/serve"
	"github.com/khayaai/forge-mutual/pkg/uuid"
	"github.com/khayaai/forge-mutual/services/platform/onboarding/internal/kyc"
	"github.com/khayaai/forge-mutual/services/platform/onboarding/internal/onboarding"
	"github.com/khayaai/forge-mutual/services/platform/onboarding/internal/store"
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
	reviews   ReviewQueue
	sessions  SessionLister
	log       *slog.Logger
	accounts  AccountOpener
}

// SessionLister returns the verification sessions started for an onboarding.
type SessionLister interface {
	Sessions(ctx context.Context, onboardingID string) ([]store.Session, error)
}

// WithSessions sets where GetOnboarding reads the verification links from.
func (s *Service) WithSessions(l SessionLister) *Service {
	s.sessions = l
	return s
}

// WithLogger sets where the webhook handler says what it did with a delivery. Without one it says nothing.
func (s *Service) WithLogger(l *slog.Logger) *Service {
	if l != nil {
		s.log = l
	}
	return s
}

var (
	_ onboardingv1connect.OnboardingServiceHandler = (*Service)(nil)
	_ Temporal                                     = (client.Client)(nil) // the real client must satisfy the interface
)

// New returns the service.
func New(tc Temporal, provider kyc.Provider, taskQueue string) *Service {
	return &Service{tc: tc, kyc: provider, taskQueue: taskQueue, log: slog.New(slog.DiscardHandler)}
}

// StartBusinessOnboarding implements OnboardingServiceHandler. The onboarding ID is derived from the key,
// so a repeated request names the same workflow. The same key with a different request is refused.
func (s *Service) StartBusinessOnboarding(ctx context.Context, req *connect.Request[onboardingv1.StartBusinessOnboardingRequest]) (*connect.Response[onboardingv1.StartBusinessOnboardingResponse], error) {
	m := req.Msg
	if m.ApplicantSubject != "" {
		if !validSubject(m.ApplicantSubject) {
			return nil, invalid("applicant_subject must be 1 to 200 printable characters")
		}
		m.IdempotencyKey = applicantKey("business", m.ApplicantSubject)
	}
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
	if err := s.start(ctx, id, hash, onboarding.OnboardBusiness, input); err != nil {
		return nil, err
	}
	return connect.NewResponse(&onboardingv1.StartBusinessOnboardingResponse{OnboardingId: id}), nil
}

// start runs a workflow under the onboarding ID. A second start with the same request answers as the first did; a
// different request under the same ID is refused.
func (s *Service) start(ctx context.Context, id, hash string, wf, input any) error {
	// WorkflowExecutionErrorWhenAlreadyStarted is required. Without it the SDK swallows the
	// already-started error and returns a handle to the existing run, so the duplicate-key check
	// below would never run and a changed request under the same key would be accepted silently.
	_, err := s.tc.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
		ID:                                       id,
		TaskQueue:                                s.taskQueue,
		WorkflowIDReusePolicy:                    enumspb.WORKFLOW_ID_REUSE_POLICY_REJECT_DUPLICATE,
		WorkflowExecutionErrorWhenAlreadyStarted: true,
	}, wf, input)
	var started *serviceerror.WorkflowExecutionAlreadyStarted
	if errors.As(err, &started) {
		prev, err := s.status(ctx, id)
		if err != nil {
			return err
		}
		if prev.RequestHash != hash {
			return connect.NewError(connect.CodeAlreadyExists, errors.New("idempotency key reused with a different request"))
		}
		return nil
	}
	if err != nil {
		return connect.NewError(connect.CodeUnavailable, errors.New("workflow service unavailable"))
	}
	return nil
}

// StartPersonOnboarding implements OnboardingServiceHandler.
func (s *Service) StartPersonOnboarding(ctx context.Context, req *connect.Request[onboardingv1.StartPersonOnboardingRequest]) (*connect.Response[onboardingv1.StartPersonOnboardingResponse], error) {
	m := req.Msg
	if m.ApplicantSubject != "" {
		if !validSubject(m.ApplicantSubject) {
			return nil, invalid("applicant_subject must be 1 to 200 printable characters")
		}
		m.IdempotencyKey = applicantKey("person", m.ApplicantSubject)
	}
	if m.IdempotencyKey == "" || len(m.IdempotencyKey) > 256 {
		return nil, invalid("idempotency_key must be 1 to 256 characters")
	}
	if strings.TrimSpace(m.LegalName) == "" || len(m.LegalName) > 200 {
		return nil, invalid("legal_name must be 1 to 200 characters")
	}
	if !countryRe.MatchString(m.Country) {
		return nil, invalid("country must be an ISO 3166-1 alpha-2 code")
	}
	nid := strings.ReplaceAll(m.NationalId, " ", "")
	if !nationalRe.MatchString(nid) || (m.Country == "ZA" && !validSAID(nid)) {
		return nil, invalid("national_id is not a valid identity number")
	}
	if m.Actor == "" {
		return nil, invalid("actor is required")
	}
	id, err := uuid.NewV5(onboardingNamespace, m.IdempotencyKey)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	hash, err := requestHash(m)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	input := onboarding.PersonInput{
		OnboardingID: id, RequestHash: hash, LegalName: m.LegalName, NationalID: nid, Country: m.Country,
		Actor: m.Actor, ApplicantSubject: m.ApplicantSubject,
	}
	if err := s.start(ctx, id, hash, onboarding.OnboardPerson, input); err != nil {
		return nil, err
	}
	return connect.NewResponse(&onboardingv1.StartPersonOnboardingResponse{OnboardingId: id}), nil
}

// FindApplication implements OnboardingServiceHandler. An applicant's application is keyed by their subject and its
// kind, so it is found without the applicant keeping any ID.
func (s *Service) FindApplication(ctx context.Context, req *connect.Request[onboardingv1.FindApplicationRequest]) (*connect.Response[onboardingv1.FindApplicationResponse], error) {
	if req.Msg.Kind != "business" && req.Msg.Kind != "person" {
		return nil, invalid(`kind must be "business" or "person"`)
	}
	if !validSubject(req.Msg.ApplicantSubject) {
		return nil, invalid("applicant_subject must be 1 to 200 printable characters")
	}
	id, err := uuid.NewV5(onboardingNamespace, applicantKey(req.Msg.Kind, req.Msg.ApplicantSubject))
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out, err := s.statusWithLinks(ctx, id)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&onboardingv1.FindApplicationResponse{Status: out}), nil
}

// applicantKey is the idempotency key of an applicant's application of a kind: one each.
func applicantKey(kind, subject string) string { return "applicant/" + kind + "/" + subject }

func validSubject(s string) bool {
	if s == "" || len(s) > 200 {
		return false
	}
	for _, r := range s {
		if r < 0x21 || r == 0x7f {
			return false
		}
	}
	return true
}

// validSAID checks a South African identity number: 13 digits, a real date of birth, and the Luhn check digit.
func validSAID(id string) bool {
	if len(id) != 13 {
		return false
	}
	sum := 0
	for i := 0; i < 13; i++ {
		d := int(id[i] - '0')
		if d < 0 || d > 9 {
			return false
		}
		if i%2 == 1 {
			d *= 2
			if d > 9 {
				d -= 9
			}
		}
		sum += d
	}
	if sum%10 != 0 {
		return false
	}
	if _, err := time.Parse("060102", id[:6]); err != nil {
		return false
	}
	return true
}

// GetOnboarding implements OnboardingServiceHandler.
func (s *Service) GetOnboarding(ctx context.Context, req *connect.Request[onboardingv1.GetOnboardingRequest]) (*connect.Response[onboardingv1.GetOnboardingResponse], error) {
	if _, err := uuid.Parse(req.Msg.OnboardingId); err != nil {
		return nil, invalid("onboarding_id must be a canonical UUID")
	}
	out, err := s.statusWithLinks(ctx, req.Msg.OnboardingId)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&onboardingv1.GetOnboardingResponse{Status: out}), nil
}

// statusWithLinks is the workflow's state with each subject's verification link.
func (s *Service) statusWithLinks(ctx context.Context, id string) (*onboardingv1.OnboardingStatus, error) {
	st, err := s.status(ctx, id)
	if err != nil {
		return nil, err
	}
	out := toProto(st)
	if s.sessions != nil {
		links, err := s.sessions.Sessions(ctx, id)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, errors.New("internal error"))
		}
		for _, l := range links {
			out.VerificationLinks = append(out.VerificationLinks, &onboardingv1.VerificationLink{
				SubjectPartyId: l.SubjectPartyID, Kind: l.Kind, SessionId: l.SessionID, Url: l.URL,
			})
		}
	}
	return out, nil
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

// maxWebhookBody bounds what a delivery may carry. A decision with every check's detail can be large.
const maxWebhookBody = 1 << 20

// webhookDeadline is how long the handler may spend. Didit gives up after five seconds, and retries a timeout, so
// answering inside that window with an error is better than being cut off.
const webhookDeadline = 4 * time.Second

// WebhookHandler receives a provider's verification result. It authenticates the delivery with the provider's own
// signature headers before it signals the workflow. Unsigned or altered deliveries are refused.
//
// What it answers matters to the provider, which retries some answers and gives up on others:
//
//   - 401: the delivery does not authenticate. Never signals the workflow.
//   - 200: acknowledged and nothing to do. This is a delivery that is not a verdict (a session starting), and a
//     verdict for an onboarding that has ended or does not exist, which no retry can change.
//   - 202: a verdict, delivered to the workflow.
//   - 400: authentic, but not about a session this service started. Not retried.
//   - 503: the workflow service could not be reached. The provider retries.
func (s *Service) WebhookHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		body, err := io.ReadAll(io.LimitReader(r.Body, maxWebhookBody+1))
		if err != nil || len(body) > maxWebhookBody {
			http.Error(w, "unreadable body", http.StatusBadRequest)
			return
		}
		// The sender is told as little as possible. The operator is told why, because a wrong or rotated secret, a clock
		// that has drifted and a destination pointed at the wrong URL all look like the same 401 from outside.
		refuse := func(status int, text string, err error) {
			s.log.Warn("kyc webhook refused", "status", status, "reason", clip(err.Error(), 300), "request_id", serve.RequestIDFrom(r.Context()))
			http.Error(w, text, status)
		}
		ev, err := s.kyc.VerifyWebhook(body, r.Header)
		switch {
		case errors.Is(err, kyc.ErrSignature):
			refuse(http.StatusUnauthorized, "signature", err)
			return
		case errors.Is(err, kyc.ErrNotConfigured):
			refuse(http.StatusServiceUnavailable, "provider not configured", err)
			return
		case errors.Is(err, kyc.ErrIgnored):
			w.WriteHeader(http.StatusOK)
			return
		case err != nil:
			refuse(http.StatusBadRequest, "invalid webhook", err)
			return
		}
		about := []any{
			"onboarding_id", ev.OnboardingID, "subject_party_id", ev.SubjectID, "verdict", ev.Verdict,
			"session_id", ev.SessionID, "event_id", ev.EventID, "request_id", serve.RequestIDFrom(r.Context()),
		}
		ctx, cancel := context.WithTimeout(r.Context(), webhookDeadline)
		defer cancel()
		err = s.tc.SignalWorkflow(ctx, ev.OnboardingID, "", onboarding.SignalKYCResult, onboarding.KYCSignal{
			SubjectPartyID: ev.SubjectID, Verdict: ev.Verdict, SessionID: ev.SessionID,
		})
		var notFound *serviceerror.NotFound
		if errors.As(err, &notFound) {
			// The onboarding has ended or never existed. The verdict is authentic and moot, and a provider that is told
			// 404 retries it for nothing.
			s.log.Info("kyc verdict for an onboarding that is not running", about...)
			w.WriteHeader(http.StatusOK)
			return
		}
		if err != nil {
			s.log.Error("kyc verdict not delivered, the provider will retry", append(about, "err", clip(err.Error(), 300))...)
			http.Error(w, "workflow service unavailable", http.StatusServiceUnavailable)
			return
		}
		s.log.Info("kyc verdict delivered", about...)
		w.WriteHeader(http.StatusAccepted)
	})
}

// ---- helpers ----

// clip shortens text for a log line.
func clip(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

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
		Kind:           st.Kind,
	}
	if out.Kind == "" {
		out.Kind = "business" // runs started before the kind was recorded
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
