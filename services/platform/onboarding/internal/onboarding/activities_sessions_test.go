package onboarding

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"go.temporal.io/sdk/temporal"

	"github.com/khayaai/forge-mutual/services/platform/onboarding/internal/kyc"
	"github.com/khayaai/forge-mutual/services/platform/onboarding/internal/store"
)

// countingProvider starts sessions and counts how many it was asked for. A real provider bills for each in live mode.
type countingProvider struct {
	kyc.Provider
	started []string
	err     error
}

func (c *countingProvider) StartPersonVerification(_ context.Context, onboardingID, subjectID string) (kyc.Session, error) {
	return c.start("person", subjectID)
}

func (c *countingProvider) StartBusinessVerification(_ context.Context, onboardingID, subjectID string) (kyc.Session, error) {
	return c.start("business", subjectID)
}

func (c *countingProvider) start(kind, subjectID string) (kyc.Session, error) {
	if c.err != nil {
		return kyc.Session{}, c.err
	}
	c.started = append(c.started, kind+":"+subjectID)
	return kyc.Session{ID: fmt.Sprintf("session-%d", len(c.started)), URL: "https://verify.example/" + subjectID}, nil
}

type memSessions struct {
	rows      map[string]store.Session
	readErr   error
	recordErr error
}

func newMemSessions() *memSessions { return &memSessions{rows: map[string]store.Session{}} }

func (m *memSessions) Session(_ context.Context, onboardingID, subjectID string) (store.Session, bool, error) {
	if m.readErr != nil {
		return store.Session{}, false, m.readErr
	}
	s, ok := m.rows[onboardingID+"/"+subjectID]
	return s, ok, nil
}

func (m *memSessions) RecordSession(_ context.Context, s store.Session) error {
	if m.recordErr != nil {
		return m.recordErr
	}
	key := s.OnboardingID + "/" + s.SubjectPartyID
	if _, ok := m.rows[key]; !ok {
		m.rows[key] = s
	}
	return nil
}

func TestAVerificationIsStartedOnceAndItsLinkIsRecorded(t *testing.T) {
	p, sessions := &countingProvider{}, newMemSessions()
	a := &Activities{KYC: p, Sessions: sessions}
	ctx := context.Background()

	if err := a.startVerification(ctx, "onb-1", "company-1", "business"); err != nil {
		t.Fatal(err)
	}
	if err := a.startVerification(ctx, "onb-1", "person-1", "person"); err != nil {
		t.Fatal(err)
	}
	co := sessions.rows["onb-1/company-1"]
	if co.Kind != "business" || co.SessionID != "session-1" || co.URL != "https://verify.example/company-1" || co.CreatedAt.IsZero() {
		t.Fatalf("company session = %+v", co)
	}
	if sessions.rows["onb-1/person-1"].Kind != "person" {
		t.Fatalf("person session = %+v", sessions.rows["onb-1/person-1"])
	}

	// The activity is retried, as Temporal does after a failure later in the same activity. No session is started
	// again, so nothing is billed twice, and the recorded one stands.
	if err := a.startVerification(ctx, "onb-1", "company-1", "business"); err != nil {
		t.Fatal(err)
	}
	if err := a.startVerification(ctx, "onb-1", "person-1", "person"); err != nil {
		t.Fatal(err)
	}
	if len(p.started) != 2 {
		t.Fatalf("the provider was asked to start %d sessions, want 2: %v", len(p.started), p.started)
	}
	if sessions.rows["onb-1/company-1"].SessionID != "session-1" {
		t.Fatal("the first recorded session must stand")
	}
}

// A refusal that a retry cannot change stops the activity at once. A transient failure is retried.
func TestAProviderRefusalIsNotRetriedButAnOutageIs(t *testing.T) {
	a := &Activities{KYC: &countingProvider{err: fmt.Errorf("%w: didit answered 401", kyc.ErrRejected)}, Sessions: newMemSessions()}
	err := a.startVerification(context.Background(), "onb-1", "person-1", "person")
	var app *temporal.ApplicationError
	if !errors.As(err, &app) || !app.NonRetryable() || app.Type() != ErrTypeProviderRejected {
		t.Fatalf("err = %v; a refusal must be a non-retryable %s", err, ErrTypeProviderRejected)
	}

	a = &Activities{KYC: &countingProvider{err: errors.New("connection reset")}, Sessions: newMemSessions()}
	err = a.startVerification(context.Background(), "onb-1", "person-1", "person")
	if err == nil || (errors.As(err, &app) && app.NonRetryable()) {
		t.Fatalf("err = %v; an outage must be retryable", err)
	}

	a = &Activities{KYC: &countingProvider{err: kyc.ErrNotConfigured}, Sessions: newMemSessions()}
	err = a.startVerification(context.Background(), "onb-1", "person-1", "person")
	if !errors.As(err, &app) || app.Type() != ErrTypeNotConfigured {
		t.Fatalf("err = %v; an unconfigured provider must stay non-retryable", err)
	}
}

func TestAStoreFailureIsRetriedAndStartsNothingItCannotRecord(t *testing.T) {
	// If the store cannot be read, nothing is started: the retry would not know what already exists.
	p := &countingProvider{}
	a := &Activities{KYC: p, Sessions: &memSessions{rows: map[string]store.Session{}, readErr: errors.New("database down")}}
	if err := a.startVerification(context.Background(), "onb-1", "person-1", "person"); err == nil {
		t.Fatal("want an error")
	}
	if len(p.started) != 0 {
		t.Fatalf("a session was started without knowing whether one existed: %v", p.started)
	}

	// If the session was started but cannot be recorded, the activity fails and is retried. The retry will start another
	// session: the small window the code documents. It costs one extra session, and no session is lost.
	p = &countingProvider{}
	sessions := newMemSessions()
	sessions.recordErr = errors.New("database down")
	a = &Activities{KYC: p, Sessions: sessions}
	if err := a.startVerification(context.Background(), "onb-1", "person-1", "person"); err == nil {
		t.Fatal("want an error when the session cannot be recorded")
	}
	sessions.recordErr = nil
	if err := a.startVerification(context.Background(), "onb-1", "person-1", "person"); err != nil {
		t.Fatal(err)
	}
	if len(p.started) != 2 || sessions.rows["onb-1/person-1"].SessionID != "session-2" {
		t.Fatalf("started = %v, recorded = %+v", p.started, sessions.rows)
	}
}

// Tests with no provider that issues links have no store: the verification is still started.
func TestWithoutASessionStoreTheVerificationStillStarts(t *testing.T) {
	p := &countingProvider{}
	a := &Activities{KYC: p}
	if err := a.startVerification(context.Background(), "onb-1", "person-1", "person"); err != nil {
		t.Fatal(err)
	}
	if len(p.started) != 1 {
		t.Fatalf("started = %v", p.started)
	}
}
