package crypt

import (
	"bytes"
	"encoding/base64"
	"testing"
)

func key(t *testing.T) Key {
	t.Helper()
	k, err := ParseKey(base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{7}, 32)))
	if err != nil {
		t.Fatal(err)
	}
	return k
}

func TestSealOpenRoundTripAndTamperDetection(t *testing.T) {
	k := key(t)
	sealed, err := k.Seal([]byte("thandi@example.com"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(sealed, []byte("thandi")) {
		t.Fatal("the address must not appear in the sealed value")
	}
	pt, err := k.Open(sealed)
	if err != nil || string(pt) != "thandi@example.com" {
		t.Fatalf("open: %q, %v", pt, err)
	}
	sealed[len(sealed)-1] ^= 1
	if _, err := k.Open(sealed); err == nil {
		t.Fatal("a tampered value must not open")
	}
}

// Two seals of the same address differ, so an observer cannot tell which contacts are the same.
func TestSealIsRandomised(t *testing.T) {
	k := key(t)
	a, _ := k.Seal([]byte("x"))
	b, _ := k.Seal([]byte("x"))
	if bytes.Equal(a, b) {
		t.Fatal("two seals of the same plaintext must differ")
	}
}

func TestParseKeyRefusesWrongLength(t *testing.T) {
	if _, err := ParseKey(base64.StdEncoding.EncodeToString([]byte("short"))); err == nil {
		t.Fatal("a short key must be refused")
	}
}
