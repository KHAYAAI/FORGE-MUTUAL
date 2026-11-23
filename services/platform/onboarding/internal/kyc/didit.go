package kyc

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/khayaai/forge-mutual/pkg/uuid"
)

// Didit is the Didit (https://docs.didit.me) verification provider.
//
// It starts a verification by creating a session (POST /v3/session/) and gives back the hosted page where the person
// completes it. It authenticates results from the webhook Didit sends when a session changes status, and turns the
// three statuses that are verdicts into APPROVED, DECLINED and REVIEW. Everything else is acknowledged and ignored.
//
// Built from Didit's published documentation. It has not yet been run against a live Didit account (VERIFY-DIDIT):
// the first live checks are listed in docs/PROVIDERS.md.
type Didit struct {
	cfg     DiditConfig
	base    string
	client  *http.Client
	now     func() time.Time
	secrets []string
}

// DiditConfig configures the adapter. Secrets come from the environment and are never logged.
type DiditConfig struct {
	APIKey        string // sent as x-api-key
	KYCWorkflowID string // the workflow for a person: the UUID from the Didit console
	KYBWorkflowID string // the workflow for a business
	WebhookSecret string // the destination's secret_shared_key
	// PreviousWebhookSecret is accepted as well while the secret is being rotated. Optional.
	PreviousWebhookSecret string
	// Environment is "sandbox" or "live". Only events from that environment are accepted, so a sandbox application
	// (which mocks its providers and can approve anyone) can never approve a person in a live deployment.
	Environment string
	BaseURL     string // default https://verification.didit.me
	// CallbackURL is where Didit sends the person when they finish. Optional.
	CallbackURL string
	HTTPClient  *http.Client // default: 15 second timeout, no redirects
	Now         func() time.Time
}

const (
	diditDefaultBase = "https://verification.didit.me"
	// diditMaxSkew is how far X-Timestamp may be from the clock. Didit signs every attempt, retries included, with a
	// fresh timestamp, so this never rejects a genuine retry.
	diditMaxSkew = 300 * time.Second
)

// NewDidit checks the configuration and returns the provider. It refuses a configuration that could approve people
// it should not, or that would send the API key somewhere unprotected.
func NewDidit(cfg DiditConfig) (*Didit, error) {
	if strings.TrimSpace(cfg.APIKey) == "" {
		return nil, errors.New("kyc: didit: an API key is required")
	}
	if _, err := uuid.Parse(cfg.KYCWorkflowID); err != nil {
		return nil, errors.New("kyc: didit: the KYC workflow ID must be a UUID")
	}
	if _, err := uuid.Parse(cfg.KYBWorkflowID); err != nil {
		return nil, errors.New("kyc: didit: the KYB workflow ID must be a UUID")
	}
	if len(cfg.WebhookSecret) < 16 {
		return nil, errors.New("kyc: didit: the webhook secret is required (at least 16 characters)")
	}
	if cfg.PreviousWebhookSecret != "" && len(cfg.PreviousWebhookSecret) < 16 {
		return nil, errors.New("kyc: didit: the previous webhook secret must be at least 16 characters")
	}
	if cfg.Environment != "sandbox" && cfg.Environment != "live" {
		return nil, errors.New(`kyc: didit: the environment must be "sandbox" or "live"`)
	}
	base := strings.TrimRight(cfg.BaseURL, "/")
	if base == "" {
		base = diditDefaultBase
	}
	if err := requireSecureURL(base, "base URL"); err != nil {
		return nil, err
	}
	if cfg.CallbackURL != "" {
		if u, err := url.Parse(cfg.CallbackURL); err != nil || u.Scheme != "https" || u.Host == "" {
			return nil, errors.New("kyc: didit: the callback URL must be an https URL")
		}
	}
	client := cfg.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	// Never follow a redirect: Go would forward the x-api-key header to wherever it points.
	copied := *client
	copied.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	now := cfg.Now
	if now == nil {
		now = time.Now
	}
	secrets := []string{cfg.WebhookSecret}
	if cfg.PreviousWebhookSecret != "" {
		secrets = append(secrets, cfg.PreviousWebhookSecret)
	}
	return &Didit{cfg: cfg, base: base, client: &copied, now: now, secrets: secrets}, nil
}

// requireSecureURL accepts an https URL, or an http URL on the loopback interface (for tests against a local fake).
func requireSecureURL(raw, what string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return fmt.Errorf("kyc: didit: the %s is not a URL", what)
	}
	if u.Scheme == "https" {
		return nil
	}
	if u.Scheme == "http" {
		if ip := net.ParseIP(u.Hostname()); (ip != nil && ip.IsLoopback()) || u.Hostname() == "localhost" {
			return nil
		}
	}
	return fmt.Errorf("kyc: didit: the %s must be https", what)
}

// StartPersonVerification implements Provider.
func (d *Didit) StartPersonVerification(ctx context.Context, onboardingID, subjectID string) (Session, error) {
	return d.start(ctx, d.cfg.KYCWorkflowID, "person", onboardingID, subjectID)
}

// StartBusinessVerification implements Provider.
func (d *Didit) StartBusinessVerification(ctx context.Context, onboardingID, subjectID string) (Session, error) {
	return d.start(ctx, d.cfg.KYBWorkflowID, "business", onboardingID, subjectID)
}

// start creates a Didit session. vendor_data is the subject's party ID, which groups a party's sessions in Didit, and
// the metadata (echoed in every webhook) says which onboarding the session belongs to. Neither holds a name or an ID
// number: Didit learns those from the person.
func (d *Didit) start(ctx context.Context, workflowID, kind, onboardingID, subjectID string) (Session, error) {
	if onboardingID == "" || subjectID == "" {
		return Session{}, fmt.Errorf("%w: an onboarding and a subject are required", ErrRejected)
	}
	body := map[string]any{
		"workflow_id": workflowID,
		"vendor_data": subjectID,
		"metadata": map[string]string{
			"onboarding_id":    onboardingID,
			"subject_party_id": subjectID,
			"subject_kind":     kind,
		},
	}
	if d.cfg.CallbackURL != "" {
		body["callback"] = d.cfg.CallbackURL
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return Session{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, d.base+"/v3/session/", bytes.NewReader(payload))
	if err != nil {
		return Session{}, err
	}
	req.Header.Set("x-api-key", d.cfg.APIKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := d.client.Do(req)
	if err != nil {
		// A transport error can be retried. It does not carry the request headers, so it holds no secret.
		return Session{}, fmt.Errorf("kyc: didit: create session: %w", err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))

	switch {
	case resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated:
	case resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500:
		return Session{}, fmt.Errorf("kyc: didit: create session: %d (will be retried)", resp.StatusCode)
	default:
		return Session{}, fmt.Errorf("%w: didit answered %d: %s", ErrRejected, resp.StatusCode, snippet(data))
	}
	var out struct {
		SessionID string `json:"session_id"`
		URL       string `json:"url"`
	}
	if err := json.Unmarshal(data, &out); err != nil || out.SessionID == "" {
		return Session{}, errors.New("kyc: didit: the create-session reply has no session_id")
	}
	if u, err := url.Parse(out.URL); err != nil || u.Scheme != "https" || u.Host == "" {
		return Session{}, errors.New("kyc: didit: the create-session reply has no https verification URL")
	}
	return Session{ID: out.SessionID, URL: out.URL}, nil
}

// snippet is the start of a response body, on one line, for an error message.
func snippet(b []byte) string {
	s := strings.Join(strings.Fields(string(b)), " ")
	if len(s) > 200 {
		s = s[:200] + "..."
	}
	return s
}

// VerifyWebhook implements Provider. A delivery authenticates when X-Signature-V2 is the hex HMAC-SHA256, under the
// destination's secret, of the body's canonical JSON, and X-Timestamp is within five minutes of the clock. Only then
// is the body read. Anything that does not authenticate returns ErrSignature with a short reason, for the operator's
// log: a wrong or rotated secret, a drifting clock and a missing header look the same to the sender, and must not to
// whoever is setting the destination up. The reason never holds a secret, a signature or any of the body.
func (d *Didit) VerifyWebhook(body []byte, header http.Header) (Event, error) {
	signature := strings.ToLower(strings.TrimSpace(header.Get("X-Signature-V2")))
	stamp := strings.TrimSpace(header.Get("X-Timestamp"))
	if signature == "" || stamp == "" {
		return Event{}, refused("X-Signature-V2 or X-Timestamp is missing")
	}
	sent, err := strconv.ParseInt(stamp, 10, 64)
	if err != nil || stamp[0] == '+' || stamp[0] == '-' {
		return Event{}, refused("X-Timestamp is not a whole number of seconds")
	}
	// Old points at a delay or a replay; ahead points at this service's clock running slow.
	limit := int64(diditMaxSkew / time.Second)
	if age := d.now().Unix() - sent; age > limit {
		return Event{}, refused(fmt.Sprintf("X-Timestamp is %ds old, and %ds is allowed", age, limit))
	} else if age < -limit {
		return Event{}, refused(fmt.Sprintf("X-Timestamp is %ds ahead of this service's clock, and %ds is allowed", -age, limit))
	}
	canonical, err := canonicalJSON(body)
	if err != nil {
		return Event{}, refused("the body is not a JSON document")
	}
	valid := false
	for _, secret := range d.secrets { // every secret is tried, so the time taken does not say which one matched
		if hmac.Equal([]byte(signV2(secret, canonical)), []byte(signature)) {
			valid = true
		}
	}
	if !valid {
		return Event{}, refused("X-Signature-V2 does not match the body under any configured secret")
	}
	return d.decode(body)
}

// refused is ErrSignature with the reason a delivery did not authenticate. The reason is for the operator's log. The
// sender is told only 401.
func refused(reason string) error { return fmt.Errorf("%w: %s", ErrSignature, reason) }

// SignDidit produces the X-Signature-V2 value Didit would send for a body. Tests and local tooling use it to send
// deliveries, as Sign does for the fake provider.
func SignDidit(secret string, body []byte) (string, error) {
	canonical, err := canonicalJSON(body)
	if err != nil {
		return "", err
	}
	return signV2(secret, canonical), nil
}

// signV2 is the X-Signature-V2 value: the hex HMAC-SHA256 of the canonical JSON, keyed with the secret as given.
func signV2(secret string, canonical []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(canonical)
	return hex.EncodeToString(mac.Sum(nil))
}

// diditVerdicts maps the session statuses that are verdicts. The strings are exact and case-sensitive
// (https://docs.didit.me/integration/verification-statuses). Every other status ("Not Started", "In Progress",
// "Awaiting User", "Resubmitted", "Expired", "Abandoned", "Kyc Expired") is not a verdict.
var diditVerdicts = map[string]string{
	"Approved":  "APPROVED",
	"Declined":  "DECLINED",
	"In Review": "REVIEW",
}

// decode reads an authenticated delivery.
func (d *Didit) decode(body []byte) (Event, error) {
	var p struct {
		EventID     string          `json:"event_id"`
		WebhookType string          `json:"webhook_type"`
		Environment string          `json:"environment"`
		SessionID   string          `json:"session_id"`
		Status      string          `json:"status"`
		VendorData  string          `json:"vendor_data"`
		Metadata    json.RawMessage `json:"metadata"`
	}
	if err := json.Unmarshal(body, &p); err != nil {
		return Event{}, fmt.Errorf("kyc: didit webhook: %w", err)
	}
	if p.WebhookType != "status.updated" {
		return Event{}, ErrIgnored
	}
	if p.Environment != d.cfg.Environment {
		return Event{}, fmt.Errorf("kyc: didit webhook is from the %q environment, this service accepts %q", p.Environment, d.cfg.Environment)
	}
	verdict, ok := diditVerdicts[p.Status]
	if !ok {
		return Event{}, ErrIgnored
	}
	var meta struct {
		OnboardingID   string `json:"onboarding_id"`
		SubjectPartyID string `json:"subject_party_id"`
	}
	if len(p.Metadata) > 0 && string(p.Metadata) != "null" {
		if err := json.Unmarshal(p.Metadata, &meta); err != nil {
			return Event{}, errors.New("kyc: didit webhook metadata is not an object")
		}
	}
	// The session must be one this service started: it names its onboarding and its subject, and the two ways it
	// names the subject must agree.
	if p.SessionID == "" || meta.OnboardingID == "" || p.VendorData == "" ||
		len(meta.OnboardingID) > 128 || len(p.VendorData) > 128 {
		return Event{}, errors.New("kyc: didit webhook does not name an onboarding and a subject")
	}
	if meta.SubjectPartyID != "" && meta.SubjectPartyID != p.VendorData {
		return Event{}, errors.New("kyc: didit webhook names two different subjects")
	}
	return Event{EventID: p.EventID, SessionID: p.SessionID, OnboardingID: meta.OnboardingID, SubjectID: p.VendorData, Verdict: verdict}, nil
}
