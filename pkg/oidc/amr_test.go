package oidc

import (
	"errors"
	"reflect"
	"testing"
)

func TestCheckAMR(t *testing.T) {
	cases := []struct {
		name          string
		have, require []string
		want          error
	}{
		{"no requirement passes", nil, nil, nil},
		{"no requirement passes with methods", []string{"pwd"}, nil, nil},
		{"password only is refused", []string{"pwd"}, []string{"mfa"}, ErrMFARequired},
		{"nothing shown is refused", nil, []string{"mfa"}, ErrMFARequired},
		{"mfa shown passes", []string{"pwd", "mfa"}, []string{"mfa"}, nil},
		{"any one of several passes", []string{"hwk"}, []string{"mfa", "hwk"}, nil},
		{"match is exact, not a prefix", []string{"mfa-lite"}, []string{"mfa"}, ErrMFARequired},
	}
	for _, c := range cases {
		if got := checkAMR(c.have, c.require); !errors.Is(got, c.want) && got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}

func TestAMROfAcceptsListOrString(t *testing.T) {
	if got := amrOf([]any{"pwd", "mfa", 7}); !reflect.DeepEqual(got, []string{"pwd", "mfa"}) {
		t.Errorf("list: %v", got)
	}
	if got := amrOf("pwd  mfa"); !reflect.DeepEqual(got, []string{"pwd", "mfa"}) {
		t.Errorf("string: %v", got)
	}
	if got := amrOf(nil); got != nil {
		t.Errorf("nil: %v", got)
	}
	if got := amrOf(42); got != nil {
		t.Errorf("number: %v", got)
	}
}

func TestParseAMRList(t *testing.T) {
	if got := ParseAMRList("mfa, hwk  otp,"); !reflect.DeepEqual(got, []string{"mfa", "hwk", "otp"}) {
		t.Errorf("got %v", got)
	}
	if got := ParseAMRList(""); len(got) != 0 {
		t.Errorf("empty must mean no requirement, got %v", got)
	}
}
