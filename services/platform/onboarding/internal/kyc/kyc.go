// Package kyc is the identity-verification boundary (ARCHITECTURE.md 11.2). Implementations: FakeProvider
// for development and tests, and Didit (didit.go), which is built from Didit's documentation and has not yet been run
// against a live account (VERIFY-DIDIT).
package kyc

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

// ErrNotConfigured is returned by a provider that has no credentials. Callers must fail closed.
var ErrNotConfigured = errors.New("kyc: provider not configured")

// ErrSignature is returned when a webhook's signature does not verify, or its timestamp is missing or stale.
var ErrSignature = errors.New("kyc: webhook signature does not verify")

// ErrIgnored is returned for an authentic delivery that is not a verdict, such as a session moving to "In Progress".
// The caller acknowledges it and does nothing.
var ErrIgnored = errors.New("kyc: delivery is not a verdict")

// ErrRejected is returned when the provider refuses a request in a way a retry will not change: bad credentials, an
// unknown workflow, a malformed request. Callers must not retry it.
var ErrRejected = errors.New("kyc: provider rejected the request")

// Session is a started verification.
type Session struct {
	ID string
	// URL is the hosted page where the person completes the verification. Empty for a provider with none.
	URL string
}

// Event is a verified result delivered by webhook.
type Event struct {
	// EventID names the delivery, where the provider gives one. A retry keeps it.
	EventID      string `json:"event_id"`
	SessionID    string `json:"session_id"`
	OnboardingID string `json:"onboarding_id"`
	SubjectID    string `json:"subject_party_id"`
	Verdict      string `json:"verdict"` // "APPROVED", "DECLINED" or "REVIEW"
}

// Provider starts verifications and authenticates their results.
type Provider interface {
	// StartPersonVerification starts an identity check for a person party.
	StartPersonVerification(ctx context.Context, onboardingID, subjectID string) (Session, error)
	// StartBusinessVerification starts a business check for a company party.
	StartBusinessVerification(ctx context.Context, onboardingID, subjectID string) (Session, error)
	// VerifyWebhook authenticates a delivery from its body and headers, and decodes it. Each provider names its own
	// signature headers. It returns ErrSignature for a delivery that does not authenticate, and ErrIgnored for an
	// authentic one that is not a verdict.
	VerifyWebhook(body []byte, header http.Header) (Event, error)
}

// FakeProvider signs webhooks with HMAC-SHA256 and never contacts a vendor. It is for development and
// tests; the signature check is the same one a real provider must pass.
type FakeProvider struct {
	secret []byte
}

// NewFake returns a fake provider with the given webhook secret.
func NewFake(secret []byte) (*FakeProvider, error) {
	if len(secret) < 16 {
		return nil, errors.New("kyc: webhook secret must be at least 16 bytes")
	}
	return &FakeProvider{secret: append([]byte(nil), secret...)}, nil
}

// StartPersonVerification implements Provider.
func (f *FakeProvider) StartPersonVerification(_ context.Context, onboardingID, subjectID string) (Session, error) {
	return fakeSession("fake-person-" + subjectID), nil
}

// StartBusinessVerification implements Provider.
func (f *FakeProvider) StartBusinessVerification(_ context.Context, onboardingID, subjectID string) (Session, error) {
	return fakeSession("fake-business-" + subjectID), nil
}

// fakeSession has a hosted link like a real provider's, so the customer journey is the same. The link is on the
// reserved .invalid domain: it can never resolve, and never reaches anyone.
func fakeSession(id string) Session {
	return Session{ID: id, URL: "https://verify.invalid/session/" + id}
}

// VerifyWebhook implements Provider. The fake's signature is in X-Forge-Signature.
func (f *FakeProvider) VerifyWebhook(body []byte, header http.Header) (Event, error) {
	return verify(f.secret, body, header.Get("X-Forge-Signature"))
}

// Sign produces the signature the fake expects. Tests and local tooling use it to send webhooks.
func Sign(secret, body []byte) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

func verify(secret, body []byte, signature string) (Event, error) {
	want := Sign(secret, body)
	if !hmac.Equal([]byte(want), []byte(signature)) {
		return Event{}, ErrSignature
	}
	var ev Event
	if err := json.Unmarshal(body, &ev); err != nil {
		return Event{}, fmt.Errorf("kyc: webhook body: %w", err)
	}
	// REVIEW means the provider cannot decide: a person must (see the onboarding workflow).
	if ev.Verdict != "APPROVED" && ev.Verdict != "DECLINED" && ev.Verdict != "REVIEW" {
		return Event{}, fmt.Errorf("kyc: unknown verdict %q", ev.Verdict)
	}
	return ev, nil
}
