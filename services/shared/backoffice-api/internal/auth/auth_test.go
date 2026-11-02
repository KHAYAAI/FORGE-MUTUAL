package auth

import (
	"net/http/httptest"
	"testing"

	"github.com/khayaai/forge-mutual/services/shared/backoffice-api/internal/api"
)

func request(name, roles string) (api.Session, error) {
	r := httptest.NewRequest("GET", "/v1/cases", nil)
	r.Header.Set("X-Forge-Staff", name)
	r.Header.Set("X-Forge-Roles", roles)
	return DevStaff{}.Session(r)
}

func TestRolesAreReadFromTheHeader(t *testing.T) {
	s, err := request("Naledi", "compliance, operations")
	if err != nil {
		t.Fatal(err)
	}
	if s.Actor != "Naledi" || !s.Roles[api.RoleCompliance] || !s.Roles[api.RoleOperations] {
		t.Fatalf("session = %+v", s)
	}
}

// A person with no known role has no session, and an unknown role is refused rather than ignored, so a typo cannot
// quietly grant or withhold access.
func TestNoRoleOrAnUnknownRoleIsNoSession(t *testing.T) {
	if _, err := request("Naledi", ""); err == nil {
		t.Fatal("a staff member with no role must have no session")
	}
	if _, err := request("Naledi", "admin"); err == nil {
		t.Fatal("an unknown role must be refused")
	}
	if _, err := request("", "compliance"); err == nil {
		t.Fatal("a missing name must be refused")
	}
}

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
}
