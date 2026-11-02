package auth

import (
	"net/http/httptest"
	"testing"
)

const party = "11111111-1111-4111-8111-111111111111"

func TestDevHeadersNeedAValidPartyAndActor(t *testing.T) {
	cases := map[string]struct{ party, actor string }{
		"no party":      {"", "alice"},
		"bad party":     {"not-a-uuid", "alice"},
		"no actor":      {party, ""},
		"actor too big": {party, string(make([]byte, 65))},
	}
	for name, c := range cases {
		r := httptest.NewRequest("GET", "/v1/accounts", nil)
		r.Header.Set("X-Forge-Party", c.party)
		r.Header.Set("X-Forge-Actor", c.actor)
		if _, err := (DevHeaders{}).Session(r); err == nil {
			t.Errorf("%s: a session was granted", name)
		}
	}
	r := httptest.NewRequest("GET", "/v1/accounts", nil)
	r.Header.Set("X-Forge-Party", party)
	r.Header.Set("X-Forge-Actor", "alice")
	s, err := (DevHeaders{}).Session(r)
	if err != nil || s.Party != party || s.Actor != "alice" {
		t.Fatalf("session = %+v, %v", s, err)
	}
}

// Dev headers are refused in production, and a production authenticator is not built, so the service does not start
// without an explicit, allowed mode.
func TestSelectRefusesDevInProduction(t *testing.T) {
	if _, err := Select("dev", "production"); err == nil {
		t.Fatal("dev authentication must be refused in production")
	}
	if _, err := Select("dev", "development"); err != nil {
		t.Fatalf("dev in development: %v", err)
	}
	if _, err := Select("", "development"); err == nil {
		t.Fatal("no mode configured must refuse to start")
	}
	if _, err := Select("oidc", "production"); err == nil {
		t.Fatal("an unbuilt mode must refuse to start")
	}
}
