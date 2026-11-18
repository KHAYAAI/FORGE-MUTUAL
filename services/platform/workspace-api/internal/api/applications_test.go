package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"

	onboardingv1 "github.com/khayaai/forge-mutual/pkg/api/forge/onboarding/v1"
	"github.com/khayaai/forge-mutual/pkg/api/forge/onboarding/v1/onboardingv1connect"
)

// fakeOnboarding holds one status per applicant and kind, as the real service keys them.
type fakeOnboarding struct {
	onboardingv1connect.OnboardingServiceClient
	persons  []*onboardingv1.StartPersonOnboardingRequest
	business []*onboardingv1.StartBusinessOnboardingRequest
	status   map[string]*onboardingv1.OnboardingStatus // subject + "/" + kind
	startErr error
}

func (f *fakeOnboarding) StartPersonOnboarding(_ context.Context, r *connect.Request[onboardingv1.StartPersonOnboardingRequest]) (*connect.Response[onboardingv1.StartPersonOnboardingResponse], error) {
	if f.startErr != nil {
		return nil, f.startErr
	}
	f.persons = append(f.persons, r.Msg)
	if _, ok := f.status[r.Msg.ApplicantSubject+"/person"]; !ok {
		f.status[r.Msg.ApplicantSubject+"/person"] = &onboardingv1.OnboardingStatus{OnboardingId: "onb-p", State: onboardingv1.OnboardingState_ONBOARDING_STATE_RUNNING, Kind: "person"}
	}
	return connect.NewResponse(&onboardingv1.StartPersonOnboardingResponse{OnboardingId: "onb-p"}), nil
}

func (f *fakeOnboarding) StartBusinessOnboarding(_ context.Context, r *connect.Request[onboardingv1.StartBusinessOnboardingRequest]) (*connect.Response[onboardingv1.StartBusinessOnboardingResponse], error) {
	f.business = append(f.business, r.Msg)
	f.status[r.Msg.ApplicantSubject+"/business"] = &onboardingv1.OnboardingStatus{OnboardingId: "onb-b", State: onboardingv1.OnboardingState_ONBOARDING_STATE_RUNNING, Kind: "business"}
	return connect.NewResponse(&onboardingv1.StartBusinessOnboardingResponse{OnboardingId: "onb-b"}), nil
}

func (f *fakeOnboarding) FindApplication(_ context.Context, r *connect.Request[onboardingv1.FindApplicationRequest]) (*connect.Response[onboardingv1.FindApplicationResponse], error) {
	st, ok := f.status[r.Msg.ApplicantSubject+"/"+r.Msg.Kind]
	if !ok {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("not found"))
	}
	return connect.NewResponse(&onboardingv1.FindApplicationResponse{Status: st}), nil
}

func appRig(t *testing.T) (http.Handler, *fakeOnboarding, *API) {
	t.Helper()
	ob := &fakeOnboarding{status: map[string]*onboardingv1.OnboardingStatus{}}
	a := &API{Auth: fakeAuth{}, Services: Services{Onboarding: ob}}
	return a.Handler(), ob, a
}

func call(t *testing.T, h http.Handler, method, path, applicant string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	if applicant != "" {
		req.Header.Set("X-Test-Applicant", applicant)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w
}

func TestAnApplicantAppliesAndSeesTheirOwnApplication(t *testing.T) {
	h, ob, _ := appRig(t)
	w := call(t, h, "POST", "/v1/applications/person", "user_01", map[string]string{"legal_name": " Naledi Mokoena ", "national_id": "9001015009086"})
	if w.Code != http.StatusAccepted {
		t.Fatalf("apply: %d %s", w.Code, w.Body)
	}
	if len(ob.persons) != 1 || ob.persons[0].ApplicantSubject != "user_01" || ob.persons[0].LegalName != "Naledi Mokoena" || ob.persons[0].Country != "ZA" {
		t.Fatalf("forwarded %+v", ob.persons)
	}
	// The applicant is who the session says, whatever the body says.
	w = call(t, h, "POST", "/v1/applications/person", "user_01", map[string]string{"legal_name": "X", "national_id": "1", "applicant_subject": "user_02"})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("a body naming an applicant must be refused: %d", w.Code)
	}
	if got := call(t, h, "GET", "/v1/applications/person", "user_01", nil); got.Code != http.StatusOK || !strings.Contains(got.Body.String(), `"state":"in_progress"`) {
		t.Fatalf("own application: %d %s", got.Code, got.Body)
	}
	if got := call(t, h, "GET", "/v1/applications/person", "user_02", nil); got.Code != http.StatusNotFound {
		t.Fatalf("someone else's: %d", got.Code)
	}
	if got := call(t, h, "GET", "/v1/applications/person", "", nil); got.Code != http.StatusUnauthorized {
		t.Fatalf("not signed in: %d", got.Code)
	}
}

func TestTheApplicantIsSentToVerifyAndThenSeesTheOutcome(t *testing.T) {
	h, ob, _ := appRig(t)
	ob.status["user_01/person"] = &onboardingv1.OnboardingStatus{
		State: onboardingv1.OnboardingState_ONBOARDING_STATE_RUNNING, Steps: []string{"prospects_created", "verifications_started"},
		VerificationLinks: []*onboardingv1.VerificationLink{{Kind: "person", Url: "https://verify.example/s1"}},
	}
	var out applicationJSON
	_ = json.Unmarshal(call(t, h, "GET", "/v1/applications/person", "user_01", nil).Body.Bytes(), &out)
	if out.Next != "verify" || len(out.Verify) != 1 || out.Verify[0].URL != "https://verify.example/s1" {
		t.Fatalf("before verification: %+v", out)
	}
	ob.status["user_01/person"] = &onboardingv1.OnboardingStatus{
		State: onboardingv1.OnboardingState_ONBOARDING_STATE_COMPLETED, PersonPartyIds: []string{partyA},
		VerificationLinks: []*onboardingv1.VerificationLink{{Kind: "person", Url: "https://verify.example/s1"}},
	}
	out = applicationJSON{}
	_ = json.Unmarshal(call(t, h, "GET", "/v1/applications/person", "user_01", nil).Body.Bytes(), &out)
	if out.State != "approved" || out.PartyID != partyA || len(out.Verify) != 0 {
		t.Fatalf("approved: %+v", out)
	}
}

// A declined applicant is told only that they were declined. Telling them they matched a screen would be tipping-off.
func TestADeclineNeverShowsItsReason(t *testing.T) {
	h, ob, _ := appRig(t)
	ob.status["user_01/person"] = &onboardingv1.OnboardingStatus{State: onboardingv1.OnboardingState_ONBOARDING_STATE_DECLINED, DeclineReason: "screening_match"}
	w := call(t, h, "GET", "/v1/applications/person", "user_01", nil)
	if !strings.Contains(w.Body.String(), `"state":"declined"`) || strings.Contains(w.Body.String(), "screening") {
		t.Fatalf("decline: %s", w.Body)
	}
}

func TestApplicationsAreRateLimitedPerApplicant(t *testing.T) {
	h, _, a := appRig(t)
	now := time.Unix(1_800_000_000, 0)
	a.Limit = &RateLimit{Max: 3, Window: time.Hour, Now: func() time.Time { return now }}
	body := map[string]string{"legal_name": "N", "national_id": "9001015009086"}
	for i := 0; i < 3; i++ {
		if w := call(t, h, "POST", "/v1/applications/person", "user_01", body); w.Code != http.StatusAccepted {
			t.Fatalf("attempt %d: %d", i, w.Code)
		}
	}
	if w := call(t, h, "POST", "/v1/applications/person", "user_01", body); w.Code != http.StatusTooManyRequests || w.Header().Get("Retry-After") == "" {
		t.Fatalf("fourth attempt: %d", w.Code)
	}
	if w := call(t, h, "POST", "/v1/applications/person", "user_02", body); w.Code != http.StatusAccepted {
		t.Fatalf("another applicant is not limited by the first: %d", w.Code)
	}
	now = now.Add(time.Hour + time.Second)
	if w := call(t, h, "POST", "/v1/applications/person", "user_01", body); w.Code != http.StatusAccepted {
		t.Fatalf("after the window: %d", w.Code)
	}
}

func TestABusinessApplicationIsForwardedWithItsDirectors(t *testing.T) {
	h, ob, _ := appRig(t)
	w := call(t, h, "POST", "/v1/applications/business", "user_05", map[string]any{
		"legal_name": "Forge Pilot (Pty) Ltd", "registration_number": "2026/123456/07", "sector_code": "software_saas",
		"directors": []map[string]any{{"legal_name": "Thandi Dube", "national_id": "8001015009087", "ownership_bps": 6000, "is_ubo": true}},
	})
	if w.Code != http.StatusAccepted || len(ob.business) != 1 || ob.business[0].ApplicantSubject != "user_05" || len(ob.business[0].Directors) != 1 {
		t.Fatalf("business: %d %s %+v", w.Code, w.Body, ob.business)
	}
	if got := call(t, h, "GET", "/v1/applications/trust", "user_05", nil); got.Code != http.StatusNotFound {
		t.Fatalf("unknown kind: %d", got.Code)
	}
}
