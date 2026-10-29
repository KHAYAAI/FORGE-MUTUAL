// Package kyc is the identity-verification boundary (ARCHITECTURE.md 11.2). Implementations: FakeProvider
// for development and tests, and Didit, which is not configured in this build (VERIFY-DIDIT).
package kyc

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
)

// ErrNotConfigured is returned by a provider that has no credentials. Callers must fail closed.
var ErrNotConfigured = errors.New("kyc: provider not configured")

// ErrSignature is returned when a webhook's signature does not verify.
var ErrSignature = errors.New("kyc: webhook signature does not verify")

// Session is a started verification.
type Session struct {
	ID string
}

// Event is a verified result delivered by webhook.
type Event struct {
	SessionID    string `json:"session_id"`
	OnboardingID string `json:"onboarding_id"`
	SubjectID    string `json:"subject_party_id"`
	Verdict      string `json:"verdict"` // "APPROVED" or "DECLINED"
}

// Provider starts verifications and authenticates their results.
type Provider interface {
	// StartPersonVerification starts an identity check for a person party.
	StartPersonVerification(ctx context.Context, onboardingID, subjectID string) (Session, error)
	// StartBusinessVerification starts a business check for a company party.
	StartBusinessVerification(ctx context.Context, onboardingID, subjectID string) (Session, error)
	// VerifyWebhook authenticates a webhook body against its signature and decodes it.
	VerifyWebhook(body []byte, signature string) (Event, error)
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
	return Session{ID: "fake-person-" + subjectID}, nil
}

// StartBusinessVerification implements Provider.
func (f *FakeProvider) StartBusinessVerification(_ context.Context, onboardingID, subjectID string) (Session, error) {
	return Session{ID: "fake-business-" + subjectID}, nil
}

// VerifyWebhook implements Provider.
func (f *FakeProvider) VerifyWebhook(body []byte, signature string) (Event, error) {
	return verify(f.secret, body, signature)
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
	if ev.Verdict != "APPROVED" && ev.Verdict != "DECLINED" {
		return Event{}, fmt.Errorf("kyc: unknown verdict %q", ev.Verdict)
	}
	return ev, nil
}

// Didit is the production adapter. It is not configured in this build: every call returns
// ErrNotConfigured, so onboarding cannot approve anyone. The sandbox contract (session creation, the
// webhook signature scheme and the result schema) must be confirmed against Didit's documentation
// before this is implemented (VERIFY-DIDIT).
type Didit struct{}

// StartPersonVerification implements Provider.
func (Didit) StartPersonVerification(context.Context, string, string) (Session, error) {
	return Session{}, ErrNotConfigured
}

// StartBusinessVerification implements Provider.
func (Didit) StartBusinessVerification(context.Context, string, string) (Session, error) {
	return Session{}, ErrNotConfigured
}

// VerifyWebhook implements Provider.
func (Didit) VerifyWebhook([]byte, string) (Event, error) { return Event{}, ErrNotConfigured }
