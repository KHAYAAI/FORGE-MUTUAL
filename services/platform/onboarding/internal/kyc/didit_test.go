package kyc

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const (
	diditSecret = "whsec_unit_test_secret_not_real_0123456789"
	kycWorkflow = "11111111-1111-4111-8111-111111111111"
	kybWorkflow = "22222222-2222-4222-8222-222222222222"
	apiKey      = "test-api-key-that-must-never-appear-in-an-error"
)

// The clock the tests run on. Didit's timestamps are whole seconds since the epoch.
var testNow = time.Unix(1774970000, 0)

func newDidit(t *testing.T, mod func(*DiditConfig)) *Didit {
	t.Helper()
	cfg := DiditConfig{
		APIKey: apiKey, KYCWorkflowID: kycWorkflow, KYBWorkflowID: kybWorkflow, WebhookSecret: diditSecret,
		Environment: "live", Now: func() time.Time { return testNow },
	}
	if mod != nil {
		mod(&cfg)
	}
	d, err := NewDidit(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

// ---- the signature scheme, against Didit's own reference code ----------------------------------------------------

type vectorFile struct {
	Secret  string `json:"secret"`
	Vectors []struct {
		Name      string `json:"name"`
		Body      string `json:"body"`
		Canonical string `json:"canonical"`
		Signature string `json:"signature"`
	} `json:"vectors"`
}

// testdata/didit_v2_vectors.json is written by testdata/gen_didit_vectors.py, which runs Didit's published Python code
// over edge cases and random documents. The Go code must give the same canonical form and the same signature for every
// one, however the body was formatted: compact, indented, with Unicode escaped or not.
func TestCanonicalJSONAndSignatureMatchDiditsReference(t *testing.T) {
	raw, err := os.ReadFile("testdata/didit_v2_vectors.json")
	if err != nil {
		t.Fatal(err)
	}
	var f vectorFile
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatal(err)
	}
	if len(f.Vectors) < 100 {
		t.Fatalf("only %d vectors: the file is not the one the generator writes", len(f.Vectors))
	}
	for _, v := range f.Vectors {
		got, err := canonicalJSON([]byte(v.Body))
		if err != nil {
			t.Errorf("%s: %v", v.Name, err)
			continue
		}
		if string(got) != v.Canonical {
			t.Errorf("%s: canonical form differs\n  got:  %s\n  want: %s", v.Name, got, v.Canonical)
			continue
		}
		if sig := signV2(f.Secret, got); sig != v.Signature {
			t.Errorf("%s: signature %s, want %s", v.Name, sig, v.Signature)
		}
	}
}

func TestCanonicalJSONRefusesWhatIsNotOneJSONDocument(t *testing.T) {
	for name, body := range map[string]string{
		"empty": "", "text": "not json", "two documents": `{"a":1}{"b":2}`, "trailing text": `{"a":1} x`,
		"truncated": `{"a":`, "number out of range": `{"a":1e999}`,
	} {
		if _, err := canonicalJSON([]byte(body)); err == nil {
			t.Errorf("%s: want an error", name)
		}
	}
}

// ---- verifying a delivery ----------------------------------------------------------------------------------------

// delivery builds a signed webhook the way Didit sends one, with the body formatted by Go's encoder (which escapes <, >
// and & and sorts keys), so that the canonical form has to be reproduced and is not simply the bytes sent.
func delivery(t *testing.T, secret string, payload map[string]any, stamp int64) ([]byte, http.Header) {
	t.Helper()
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	canon, err := canonicalJSON(body)
	if err != nil {
		t.Fatal(err)
	}
	h := http.Header{}
	h.Set("X-Signature-V2", signV2(secret, canon))
	h.Set("X-Timestamp", strconv.FormatInt(stamp, 10))
	return body, h
}

func statusPayload(status string) map[string]any {
	return map[string]any{
		"event_id": "9c0c8b8a-1111-4222-9333-444444444444", "webhook_type": "status.updated", "timestamp": 1774970000,
		"environment": "live", "session_id": "sess-1", "status": status, "vendor_data": "party-1",
		"metadata": map[string]any{"onboarding_id": "onb-1", "subject_party_id": "party-1", "subject_kind": "person"},
		"decision": map[string]any{
			"status": status, "score": 98.0, "ratio": 0.9876,
			"id_verifications": []any{map[string]any{"full_name": "José <Ramírez> & Sons", "confidence": 0.5}},
		},
	}
}

func TestDiditVerdictsMapToOurs(t *testing.T) {
	d := newDidit(t, nil)
	for status, want := range map[string]string{"Approved": "APPROVED", "Declined": "DECLINED", "In Review": "REVIEW"} {
		body, h := delivery(t, diditSecret, statusPayload(status), testNow.Unix())
		ev, err := d.VerifyWebhook(body, h)
		if err != nil {
			t.Fatalf("%s: %v", status, err)
		}
		if ev.Verdict != want || ev.OnboardingID != "onb-1" || ev.SubjectID != "party-1" || ev.SessionID != "sess-1" ||
			ev.EventID != "9c0c8b8a-1111-4222-9333-444444444444" {
			t.Errorf("%s: event = %+v", status, ev)
		}
	}
}

// Only three statuses are verdicts. The others, and other event types, are authentic but ignored. They are checked
// for a signature first, so an unsigned one is a signature failure and not an ignored event.
func TestDiditIgnoresWhatIsNotAVerdict(t *testing.T) {
	d := newDidit(t, nil)
	for _, status := range []string{"Not Started", "In Progress", "Awaiting User", "Resubmitted", "Expired", "Abandoned", "Kyc Expired", "approved", "APPROVED", ""} {
		body, h := delivery(t, diditSecret, statusPayload(status), testNow.Unix())
		if _, err := d.VerifyWebhook(body, h); !errors.Is(err, ErrIgnored) {
			t.Errorf("status %q: err = %v, want ErrIgnored (matching is exact and case-sensitive)", status, err)
		}
	}
	for _, kind := range []string{"data.updated", "user.created", "business.status.updated", ""} {
		p := statusPayload("Approved")
		p["webhook_type"] = kind
		body, h := delivery(t, diditSecret, p, testNow.Unix())
		if _, err := d.VerifyWebhook(body, h); !errors.Is(err, ErrIgnored) {
			t.Errorf("type %q: err = %v, want ErrIgnored", kind, err)
		}
	}
	body, _ := delivery(t, diditSecret, statusPayload("In Progress"), testNow.Unix())
	if _, err := d.VerifyWebhook(body, http.Header{}); !errors.Is(err, ErrSignature) {
		t.Errorf("an unsigned delivery of an ignorable status must fail the signature, got %v", err)
	}
}

func TestDiditRefusesWhatDoesNotAuthenticate(t *testing.T) {
	d := newDidit(t, nil)
	good, goodH := delivery(t, diditSecret, statusPayload("Approved"), testNow.Unix())
	if _, err := d.VerifyWebhook(good, goodH); err != nil {
		t.Fatalf("the control delivery must verify: %v", err)
	}
	clone := func(mod func(http.Header)) http.Header {
		h := goodH.Clone()
		mod(h)
		return h
	}
	cases := map[string]struct {
		body []byte
		h    http.Header
	}{
		"no signature":       {good, clone(func(h http.Header) { h.Del("X-Signature-V2") })},
		"no timestamp":       {good, clone(func(h http.Header) { h.Del("X-Timestamp") })},
		"empty signature":    {good, clone(func(h http.Header) { h.Set("X-Signature-V2", "") })},
		"not hex":            {good, clone(func(h http.Header) { h.Set("X-Signature-V2", "zz") })},
		"another digest":     {good, clone(func(h http.Header) { h.Set("X-Signature-V2", strings.Repeat("0", 64)) })},
		"truncated digest":   {good, clone(func(h http.Header) { h.Set("X-Signature-V2", goodH.Get("X-Signature-V2")[:32]) })},
		"timestamp text":     {good, clone(func(h http.Header) { h.Set("X-Timestamp", "now") })},
		"timestamp signed":   {good, clone(func(h http.Header) { h.Set("X-Timestamp", "+"+strconv.FormatInt(testNow.Unix(), 10)) })},
		"timestamp decimal":  {good, clone(func(h http.Header) { h.Set("X-Timestamp", strconv.FormatInt(testNow.Unix(), 10)+".5") })},
		"timestamp negative": {good, clone(func(h http.Header) { h.Set("X-Timestamp", "-1") })},
		"body not json":      {[]byte("verdict: approved"), goodH},
		"two documents":      {append(append([]byte{}, good...), good...), goodH},
	}
	for name, c := range cases {
		if _, err := d.VerifyWebhook(c.body, c.h); !errors.Is(err, ErrSignature) {
			t.Errorf("%s: err = %v, want ErrSignature", name, err)
		}
	}

	// A different secret.
	body, h := delivery(t, "whsec_another_secret_entirely_0123456789", statusPayload("Approved"), testNow.Unix())
	if _, err := d.VerifyWebhook(body, h); !errors.Is(err, ErrSignature) {
		t.Errorf("wrong secret: %v", err)
	}

	// Altered after signing: a top-level field, and a field deep inside the decision. The signature covers all of it.
	for name, edit := range map[string]func(map[string]any){
		"vendor_data": func(p map[string]any) { p["vendor_data"] = "party-2" },
		"status":      func(p map[string]any) { p["status"] = "Declined" },
		"decision":    func(p map[string]any) { p["decision"].(map[string]any)["ratio"] = 0.9877 },
		"nested": func(p map[string]any) {
			p["decision"].(map[string]any)["id_verifications"].([]any)[0].(map[string]any)["full_name"] = "X"
		},
	} {
		signed := statusPayload("Approved")
		_, sh := delivery(t, diditSecret, signed, testNow.Unix())
		edit(signed)
		altered, _ := json.Marshal(signed)
		if _, err := d.VerifyWebhook(altered, sh); !errors.Is(err, ErrSignature) {
			t.Errorf("altered %s: err = %v, want ErrSignature", name, err)
		}
	}
}

// Didit's window is 300 seconds either side of the clock. Exactly 300 passes; 301 does not.
func TestDiditTimestampWindow(t *testing.T) {
	d := newDidit(t, nil)
	for offset, want := range map[int64]bool{0: true, 299: true, 300: true, 301: false, -299: true, -300: true, -301: false, 86400: false, -86400: false} {
		body, h := delivery(t, diditSecret, statusPayload("Approved"), testNow.Unix()-offset)
		_, err := d.VerifyWebhook(body, h)
		if (err == nil) != want {
			t.Errorf("timestamp %ds old: err = %v, want accepted = %v", offset, err, want)
		}
		if err != nil && !errors.Is(err, ErrSignature) {
			t.Errorf("timestamp %ds old: err = %v, want ErrSignature", offset, err)
		}
	}
}

func TestDiditAcceptsThePreviousSecretWhileRotating(t *testing.T) {
	previous := "whsec_the_previous_secret_0123456789ab"
	d := newDidit(t, func(c *DiditConfig) { c.PreviousWebhookSecret = previous })
	for _, secret := range []string{diditSecret, previous} {
		body, h := delivery(t, secret, statusPayload("Approved"), testNow.Unix())
		if _, err := d.VerifyWebhook(body, h); err != nil {
			t.Errorf("secret %q: %v", secret[:12], err)
		}
	}
	// Without a previous secret configured, the old one is refused.
	d = newDidit(t, nil)
	body, h := delivery(t, previous, statusPayload("Approved"), testNow.Unix())
	if _, err := d.VerifyWebhook(body, h); !errors.Is(err, ErrSignature) {
		t.Errorf("an old secret after rotation must be refused: %v", err)
	}
}

// A sandbox application mocks its providers and can approve anyone. Its events must never be accepted by a live
// deployment, even with a valid signature.
func TestDiditAcceptsOnlyItsOwnEnvironment(t *testing.T) {
	sandbox := statusPayload("Approved")
	sandbox["environment"] = "sandbox"
	sandbox["sandbox_scenario"] = "approve"
	body, h := delivery(t, diditSecret, sandbox, testNow.Unix())

	live := newDidit(t, nil)
	_, err := live.VerifyWebhook(body, h)
	if err == nil || errors.Is(err, ErrSignature) || errors.Is(err, ErrIgnored) {
		t.Fatalf("a sandbox event at a live service must be refused loudly, got %v", err)
	}
	if _, err := newDidit(t, func(c *DiditConfig) { c.Environment = "sandbox" }).VerifyWebhook(body, h); err != nil {
		t.Errorf("a sandbox service accepts a sandbox event: %v", err)
	}
	// And a live event at a sandbox service.
	liveBody, liveH := delivery(t, diditSecret, statusPayload("Approved"), testNow.Unix())
	if _, err := newDidit(t, func(c *DiditConfig) { c.Environment = "sandbox" }).VerifyWebhook(liveBody, liveH); err == nil {
		t.Error("a live event at a sandbox service must be refused too")
	}
}

// A verdict is accepted only for a session this service started: it names its onboarding and its subject.
func TestDiditRefusesASessionThatIsNotOurs(t *testing.T) {
	d := newDidit(t, nil)
	cases := map[string]func(map[string]any){
		"no metadata":            func(p map[string]any) { delete(p, "metadata") },
		"null metadata":          func(p map[string]any) { p["metadata"] = nil },
		"metadata not an object": func(p map[string]any) { p["metadata"] = "onb-1" },
		"no onboarding":          func(p map[string]any) { p["metadata"] = map[string]any{"subject_party_id": "party-1"} },
		"no vendor_data":         func(p map[string]any) { delete(p, "vendor_data") },
		"no session":             func(p map[string]any) { delete(p, "session_id") },
		"two subjects":           func(p map[string]any) { p["metadata"].(map[string]any)["subject_party_id"] = "party-2" },
		"onboarding too long":    func(p map[string]any) { p["metadata"].(map[string]any)["onboarding_id"] = strings.Repeat("x", 129) },
	}
	for name, edit := range cases {
		p := statusPayload("Approved")
		edit(p)
		body, h := delivery(t, diditSecret, p, testNow.Unix())
		_, err := d.VerifyWebhook(body, h)
		if err == nil || errors.Is(err, ErrSignature) || errors.Is(err, ErrIgnored) {
			t.Errorf("%s: err = %v, want a refusal that is neither a signature failure nor ignored", name, err)
		}
	}
	// The subject is named in vendor_data alone, with metadata that does not repeat it: accepted.
	p := statusPayload("Approved")
	p["metadata"] = map[string]any{"onboarding_id": "onb-1"}
	body, h := delivery(t, diditSecret, p, testNow.Unix())
	if ev, err := d.VerifyWebhook(body, h); err != nil || ev.SubjectID != "party-1" {
		t.Errorf("vendor_data alone: %+v, %v", ev, err)
	}
}

// ---- starting a session ------------------------------------------------------------------------------------------

type seen struct {
	method, path, key, contentType string
	body                           map[string]any
}

func fakeDidit(t *testing.T, status int, reply string, got *seen) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got != nil {
			got.method, got.path, got.key, got.contentType = r.Method, r.URL.Path, r.Header.Get("x-api-key"), r.Header.Get("Content-Type")
			_ = json.NewDecoder(r.Body).Decode(&got.body)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(reply))
	}))
	t.Cleanup(srv.Close)
	return srv
}

const createdReply = `{"session_id":"4c5c7f3a-1f82-4f3b-8d8e-1a8d2d2f9b7a","session_kind":"user","status":"Not Started","url":"https://verify.didit.me/session/abc","session_token":"tok"}`

func TestDiditStartsASessionTheDocumentedWay(t *testing.T) {
	var got seen
	srv := fakeDidit(t, http.StatusCreated, createdReply, &got)
	d := newDidit(t, func(c *DiditConfig) { c.BaseURL = srv.URL + "/"; c.CallbackURL = "https://app.example/verified" })

	s, err := d.StartPersonVerification(t.Context(), "onb-1", "party-1")
	if err != nil {
		t.Fatal(err)
	}
	if s.ID != "4c5c7f3a-1f82-4f3b-8d8e-1a8d2d2f9b7a" || s.URL != "https://verify.didit.me/session/abc" {
		t.Fatalf("session = %+v", s)
	}
	if got.method != "POST" || got.path != "/v3/session/" || got.key != apiKey || got.contentType != "application/json" {
		t.Fatalf("request = %+v", got)
	}
	meta, _ := got.body["metadata"].(map[string]any)
	if got.body["workflow_id"] != kycWorkflow || got.body["vendor_data"] != "party-1" || got.body["callback"] != "https://app.example/verified" ||
		meta["onboarding_id"] != "onb-1" || meta["subject_party_id"] != "party-1" || meta["subject_kind"] != "person" {
		t.Fatalf("body = %+v", got.body)
	}
	// Nothing that identifies the person goes to Didit in the request: only IDs.
	if len(got.body) != 4 || len(meta) != 3 {
		t.Fatalf("the request carries more than the workflow, the subject and the onboarding: %+v", got.body)
	}

	if _, err := d.StartBusinessVerification(t.Context(), "onb-1", "company-1"); err != nil {
		t.Fatal(err)
	}
	meta, _ = got.body["metadata"].(map[string]any)
	if got.body["workflow_id"] != kybWorkflow || meta["subject_kind"] != "business" {
		t.Fatalf("a business uses the KYB workflow: %+v", got.body)
	}
}

func TestDiditOmitsTheCallbackWhenNoneIsConfigured(t *testing.T) {
	var got seen
	srv := fakeDidit(t, http.StatusCreated, createdReply, &got)
	d := newDidit(t, func(c *DiditConfig) { c.BaseURL = srv.URL })
	if _, err := d.StartPersonVerification(t.Context(), "onb-1", "party-1"); err != nil {
		t.Fatal(err)
	}
	if _, present := got.body["callback"]; present {
		t.Fatalf("no callback was configured: %+v", got.body)
	}
}

// A refusal that a retry cannot change is ErrRejected. A busy or failing Didit is retried. Neither error carries the
// API key.
func TestDiditStartFailures(t *testing.T) {
	cases := []struct {
		name     string
		status   int
		reply    string
		rejected bool
	}{
		{"bad key", 401, `{"detail":"Invalid API key"}`, true},
		{"bad request", 400, `{"workflow_id":["not a valid UUID"]}`, true},
		{"forbidden", 403, `{"detail":"no credits"}`, true},
		{"not found", 404, `{"detail":"workflow not found"}`, true},
		{"redirect", 302, ``, true},
		{"rate limited", 429, `{"detail":"slow down"}`, false},
		{"server error", 500, `oops`, false},
		{"bad gateway", 502, ``, false},
		{"no session id", 201, `{"url":"https://verify.didit.me/s"}`, false},
		{"no url", 201, `{"session_id":"s1"}`, false},
		{"insecure url", 201, `{"session_id":"s1","url":"http://verify.didit.me/s"}`, false},
		{"not json", 201, `<html>`, false},
	}
	for _, c := range cases {
		srv := fakeDidit(t, c.status, c.reply, nil)
		d := newDidit(t, func(cfg *DiditConfig) { cfg.BaseURL = srv.URL })
		_, err := d.StartPersonVerification(t.Context(), "onb-1", "party-1")
		if err == nil {
			t.Errorf("%s: want an error", c.name)
			continue
		}
		if errors.Is(err, ErrRejected) != c.rejected {
			t.Errorf("%s: rejected = %v, want %v (%v)", c.name, errors.Is(err, ErrRejected), c.rejected, err)
		}
		if strings.Contains(err.Error(), apiKey) {
			t.Errorf("%s: the error carries the API key: %v", c.name, err)
		}
	}
	// An unreachable Didit is a retryable error, not a rejection.
	srv := fakeDidit(t, 201, createdReply, nil)
	d := newDidit(t, func(cfg *DiditConfig) { cfg.BaseURL = srv.URL })
	srv.Close()
	if _, err := d.StartPersonVerification(t.Context(), "onb-1", "party-1"); err == nil || errors.Is(err, ErrRejected) {
		t.Errorf("a connection failure must be retryable: %v", err)
	}
}

// Go forwards custom headers across a redirect, so the API key would follow it. The client must not follow.
func TestDiditDoesNotFollowRedirectsWithTheKey(t *testing.T) {
	var leaked atomic.Bool
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-api-key") != "" {
			leaked.Store(true)
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(createdReply))
	}))
	t.Cleanup(other.Close)
	redirecting := httptest.NewServer(http.RedirectHandler(other.URL+"/v3/session/", http.StatusTemporaryRedirect))
	t.Cleanup(redirecting.Close)

	d := newDidit(t, func(c *DiditConfig) { c.BaseURL = redirecting.URL })
	if _, err := d.StartPersonVerification(t.Context(), "onb-1", "party-1"); err == nil {
		t.Fatal("a redirect must be an error")
	}
	if leaked.Load() {
		t.Fatal("the API key followed a redirect to another host")
	}
}

func TestDiditRefusesAnUnsafeOrIncompleteConfiguration(t *testing.T) {
	ok := DiditConfig{APIKey: apiKey, KYCWorkflowID: kycWorkflow, KYBWorkflowID: kybWorkflow, WebhookSecret: diditSecret, Environment: "live"}
	if _, err := NewDidit(ok); err != nil {
		t.Fatalf("the control configuration must pass: %v", err)
	}
	bad := map[string]func(*DiditConfig){
		"no key":                func(c *DiditConfig) { c.APIKey = " " },
		"kyc workflow":          func(c *DiditConfig) { c.KYCWorkflowID = "not-a-uuid" },
		"kyb workflow":          func(c *DiditConfig) { c.KYBWorkflowID = "" },
		"short secret":          func(c *DiditConfig) { c.WebhookSecret = "short" },
		"short previous secret": func(c *DiditConfig) { c.PreviousWebhookSecret = "short" },
		"no environment":        func(c *DiditConfig) { c.Environment = "" },
		"unknown environment":   func(c *DiditConfig) { c.Environment = "production" },
		"plain http base":       func(c *DiditConfig) { c.BaseURL = "http://verification.didit.me" },
		"not a url":             func(c *DiditConfig) { c.BaseURL = "verification.didit.me" },
		"plain http callback":   func(c *DiditConfig) { c.CallbackURL = "http://app.example/back" },
		"not a callback url":    func(c *DiditConfig) { c.CallbackURL = "/back" },
	}
	for name, mod := range bad {
		c := ok
		mod(&c)
		if _, err := NewDidit(c); err == nil {
			t.Errorf("%s: want an error", name)
		} else if strings.Contains(err.Error(), apiKey) || strings.Contains(err.Error(), diditSecret) {
			t.Errorf("%s: the error carries a secret: %v", name, err)
		}
	}
	// A local fake over plain http is for tests only: loopback is allowed, nothing else.
	c := ok
	c.BaseURL = "http://127.0.0.1:9999"
	if _, err := NewDidit(c); err != nil {
		t.Errorf("loopback http: %v", err)
	}
}
