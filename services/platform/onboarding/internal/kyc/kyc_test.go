package kyc

import (
	"errors"
	"net/http"
	"testing"
)

// forgeHeader is the header the fake provider reads its signature from.
func forgeHeader(sig string) http.Header {
	h := http.Header{}
	if sig != "" {
		h.Set("X-Forge-Signature", sig)
	}
	return h
}

func fake(t *testing.T) *FakeProvider {
	t.Helper()
	p, err := NewFake([]byte("0123456789abcdef-test-secret"))
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestSignedWebhookIsAccepted(t *testing.T) {
	p := fake(t)
	body := []byte(`{"session_id":"s1","onboarding_id":"o1","subject_party_id":"p1","verdict":"APPROVED"}`)
	ev, err := p.VerifyWebhook(body, forgeHeader(Sign([]byte("0123456789abcdef-test-secret"), body)))
	if err != nil || ev.Verdict != "APPROVED" || ev.SubjectID != "p1" {
		t.Fatalf("event = %+v, err = %v", ev, err)
	}
}

func TestBadSignatureIsRefused(t *testing.T) {
	p := fake(t)
	body := []byte(`{"session_id":"s1","onboarding_id":"o1","subject_party_id":"p1","verdict":"APPROVED"}`)
	if _, err := p.VerifyWebhook(body, forgeHeader("00")); !errors.Is(err, ErrSignature) {
		t.Fatalf("forged signature: %v", err)
	}
	tampered := []byte(`{"session_id":"s1","onboarding_id":"o1","subject_party_id":"p2","verdict":"APPROVED"}`)
	sig := Sign([]byte("0123456789abcdef-test-secret"), body)
	if _, err := p.VerifyWebhook(tampered, forgeHeader(sig)); !errors.Is(err, ErrSignature) {
		t.Fatalf("altered body must not verify: %v", err)
	}
}

func TestUnknownVerdictIsRefused(t *testing.T) {
	p := fake(t)
	body := []byte(`{"verdict":"MAYBE"}`)
	if _, err := p.VerifyWebhook(body, forgeHeader(Sign([]byte("0123456789abcdef-test-secret"), body))); err == nil {
		t.Fatal("unknown verdict must be refused")
	}
}

func TestShortSecretRefused(t *testing.T) {
	if _, err := NewFake([]byte("short")); err == nil {
		t.Fatal("short secret must be refused")
	}
}

func TestMissingSignatureIsRefused(t *testing.T) {
	body := []byte(`{"session_id":"s1","onboarding_id":"o1","subject_party_id":"p1","verdict":"APPROVED"}`)
	if _, err := fake(t).VerifyWebhook(body, http.Header{}); !errors.Is(err, ErrSignature) {
		t.Fatalf("an unsigned delivery must be refused: %v", err)
	}
}
