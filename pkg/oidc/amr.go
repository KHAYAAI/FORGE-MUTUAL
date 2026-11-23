package oidc

import (
	"errors"
	"strings"
)

// ErrMFARequired is returned when a token does not show a second factor. Joint Standard 2 of 2024, paragraph 8.3.1,
// requires multi-factor authentication for users of critical system functions, for every administrative and privileged
// account, and for every account that reaches sensitive information over the internet.
var ErrMFARequired = errors.New("oidc: token does not show multi-factor authentication")

// ClaimAMR is the authentication methods reference claim (RFC 8176): the ways the holder proved who they are.
const ClaimAMR = "amr"

// DefaultRequireAMR is what the platform looks for when a deployment sets nothing more specific. "mfa" is the value
// RFC 8176 defines for "multiple-factor authentication".
//
// VERIFY-WORKOS: whether WorkOS AuthKit access tokens carry an "amr" claim, and with what values, has not been
// confirmed against a real token. Until it has, a deployment can set OIDC_REQUIRE_AMR to whatever its provider
// actually sends, and the check stays the same.
var DefaultRequireAMR = []string{"mfa"}

// ParseAMRList reads a comma- or space-separated list from configuration. An empty list means "no requirement".
func ParseAMRList(s string) []string {
	var out []string
	for _, f := range strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == ' ' || r == '\t' }) {
		if f != "" {
			out = append(out, f)
		}
	}
	return out
}

// amrOf reads the amr claim, which may be a list or a space-separated string.
func amrOf(v any) []string {
	switch a := v.(type) {
	case []any:
		var out []string
		for _, x := range a {
			if s, ok := x.(string); ok {
				out = append(out, s)
			}
		}
		return out
	case string:
		return strings.Fields(a)
	}
	return nil
}

// checkAMR passes when no requirement is set, or when the token shows at least one of the required methods.
func checkAMR(have, require []string) error {
	if len(require) == 0 {
		return nil
	}
	for _, h := range have {
		for _, r := range require {
			if h == r {
				return nil
			}
		}
	}
	return ErrMFARequired
}
