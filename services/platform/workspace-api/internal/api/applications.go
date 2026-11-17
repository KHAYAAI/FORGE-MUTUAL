package api

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"connectrpc.com/connect"

	onboardingv1 "github.com/khayaai/forge-mutual/pkg/api/forge/onboarding/v1"
)

// Applications: the customer front door. A signed-in person applies for a personal or a business account. Each
// applicant has at most one application of each kind, keyed by their identity provider subject, so applying twice
// is the same application, and they find it again by signing in.

type applicantHandler func(w http.ResponseWriter, r *http.Request, ap Applicant)

func (a *API) applicant(h applicantHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ap, err := a.Auth.Applicant(r)
		if err != nil {
			writeJSON(w, http.StatusUnauthorized, errBody("unauthenticated", "sign in to apply"))
			return
		}
		h(w, r, ap)
	}
}

type personApplication struct {
	LegalName  string `json:"legal_name"`
	NationalID string `json:"national_id"`
}

type directorJSON struct {
	LegalName    string `json:"legal_name"`
	NationalID   string `json:"national_id"`
	OwnershipBps uint32 `json:"ownership_bps"`
	IsUBO        bool   `json:"is_ubo"`
}

type businessApplication struct {
	LegalName          string         `json:"legal_name"`
	RegistrationNumber string         `json:"registration_number"`
	SectorCode         string         `json:"sector_code"`
	Directors          []directorJSON `json:"directors"`
}

func (a *API) applyPerson(w http.ResponseWriter, r *http.Request, ap Applicant) {
	var in personApplication
	if !decode(w, r, &in) || !a.allow(w, ap) {
		return
	}
	_, err := a.Services.Onboarding.StartPersonOnboarding(r.Context(), connect.NewRequest(&onboardingv1.StartPersonOnboardingRequest{
		LegalName: strings.TrimSpace(in.LegalName), NationalId: in.NationalID, Country: "ZA",
		Actor: ap.Actor, ApplicantSubject: ap.Subject,
	}))
	if err != nil {
		writeErr(w, err)
		return
	}
	a.respondApplication(w, r, ap, "person", http.StatusAccepted)
}

func (a *API) applyBusiness(w http.ResponseWriter, r *http.Request, ap Applicant) {
	var in businessApplication
	if !decode(w, r, &in) || !a.allow(w, ap) {
		return
	}
	req := &onboardingv1.StartBusinessOnboardingRequest{
		LegalName: strings.TrimSpace(in.LegalName), RegistrationNumber: strings.TrimSpace(in.RegistrationNumber),
		Country: "ZA", SectorCode: in.SectorCode, Actor: ap.Actor, ApplicantSubject: ap.Subject,
	}
	for _, d := range in.Directors {
		req.Directors = append(req.Directors, &onboardingv1.Director{
			LegalName: strings.TrimSpace(d.LegalName), NationalId: d.NationalID, OwnershipBps: d.OwnershipBps, IsUbo: d.IsUBO,
		})
	}
	if _, err := a.Services.Onboarding.StartBusinessOnboarding(r.Context(), connect.NewRequest(req)); err != nil {
		writeErr(w, err)
		return
	}
	a.respondApplication(w, r, ap, "business", http.StatusAccepted)
}

func (a *API) getApplication(w http.ResponseWriter, r *http.Request, ap Applicant) {
	kind := r.PathValue("kind")
	if kind != "person" && kind != "business" {
		writeJSON(w, http.StatusNotFound, errBody("not_found", "no such application kind"))
		return
	}
	a.respondApplication(w, r, ap, kind, http.StatusOK)
}

type applicationJSON struct {
	Kind  string `json:"kind"`
	State string `json:"state"` // in_progress, approved, declined, or failed
	// Next is what the applicant must do now: "verify" (open each verification link), or "wait".
	Next   string     `json:"next"`
	Steps  []string   `json:"steps"`
	Verify []linkJSON `json:"verify,omitempty"`
	// PartyID is the applicant's customer record, once approved.
	PartyID string `json:"party_id,omitempty"`
}

type linkJSON struct {
	Kind string `json:"kind"` // "person" or "business"
	URL  string `json:"url"`
}

// respondApplication answers with the applicant's own application of that kind. The decline reason is never shown:
// telling an applicant that they matched a sanctions or PEP screen would be tipping-off (FIC Act section 29), and the
// other reasons are for staff.
func (a *API) respondApplication(w http.ResponseWriter, r *http.Request, ap Applicant, kind string, status int) {
	res, err := a.Services.Onboarding.FindApplication(r.Context(), connect.NewRequest(&onboardingv1.FindApplicationRequest{
		ApplicantSubject: ap.Subject, Kind: kind,
	}))
	if err != nil {
		if connect.CodeOf(err) == connect.CodeNotFound {
			writeJSON(w, http.StatusNotFound, errBody("not_found", "you have not applied for this yet"))
			return
		}
		writeErr(w, err)
		return
	}
	st := res.Msg.Status
	out := applicationJSON{Kind: kind, Steps: st.Steps, Next: "wait"}
	if out.Steps == nil {
		out.Steps = []string{}
	}
	switch st.State {
	case onboardingv1.OnboardingState_ONBOARDING_STATE_COMPLETED:
		out.State = "approved"
		if kind == "person" && len(st.PersonPartyIds) == 1 {
			out.PartyID = st.PersonPartyIds[0]
		} else if kind == "business" {
			out.PartyID = st.CompanyPartyId
		}
		out.Next = ""
	case onboardingv1.OnboardingState_ONBOARDING_STATE_DECLINED:
		out.State, out.Next = "declined", ""
	case onboardingv1.OnboardingState_ONBOARDING_STATE_FAILED:
		out.State = "failed"
	default:
		out.State = "in_progress"
		verified := false
		for _, s := range st.Steps {
			if s == "kyc_approved" {
				verified = true
			}
		}
		if !verified {
			for _, l := range st.VerificationLinks {
				if l.Url != "" {
					out.Verify = append(out.Verify, linkJSON{Kind: l.Kind, URL: l.Url})
				}
			}
			if len(out.Verify) > 0 {
				out.Next = "verify"
			}
		}
	}
	writeJSON(w, status, out)
}

// ---- rate limit ----

// RateLimit allows each applicant a number of submissions per window. It is per process: with several replicas the
// effective limit is that many times higher, which is acceptable because an application is idempotent per applicant
// and starts at most one verification each. The limit stops a signed-in account from hammering the endpoint.
type RateLimit struct {
	Max    int
	Window time.Duration
	Now    func() time.Time

	mu   sync.Mutex
	seen map[string][]time.Time
}

func (a *API) allow(w http.ResponseWriter, ap Applicant) bool {
	if a.Limit == nil {
		a.Limit = &RateLimit{Max: 10, Window: time.Hour}
	}
	if a.Limit.take(ap.Subject) {
		return true
	}
	w.Header().Set("Retry-After", "3600")
	writeJSON(w, http.StatusTooManyRequests, errBody("rate_limited", "too many applications from this account; try again later"))
	return false
}

func (l *RateLimit) take(key string) bool {
	now := time.Now
	if l.Now != nil {
		now = l.Now
	}
	t := now()
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.seen == nil {
		l.seen = map[string][]time.Time{}
	}
	kept := l.seen[key][:0]
	for _, at := range l.seen[key] {
		if t.Sub(at) < l.Window {
			kept = append(kept, at)
		}
	}
	if len(kept) >= l.Max {
		l.seen[key] = kept
		return false
	}
	l.seen[key] = append(kept, t)
	return true
}
