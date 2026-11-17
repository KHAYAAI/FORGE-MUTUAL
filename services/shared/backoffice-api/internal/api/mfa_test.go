package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type mfaAuth struct{ Authenticator }

func (mfaAuth) Session(*http.Request) (Session, error) { return Session{}, ErrMFARequired }

// A valid sign-in without a second factor is told so (403, mfa_required), not sent round the sign-in loop.
func TestASignInWithoutASecondFactorIsToldSo(t *testing.T) {
	h := (&API{Auth: mfaAuth{}}).Handler()
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/v1/cases", nil))
	if w.Code != http.StatusForbidden || !strings.Contains(w.Body.String(), "mfa_required") {
		t.Fatalf("got %d %s", w.Code, w.Body)
	}
}
