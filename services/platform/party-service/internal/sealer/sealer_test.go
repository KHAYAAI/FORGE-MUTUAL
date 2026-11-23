package sealer

import (
	"bytes"
	"testing"
)

func newSealer(t *testing.T) *Sealer {
	t.Helper()
	enc := bytes.Repeat([]byte{1}, 32)
	idx := bytes.Repeat([]byte{2}, 32)
	s, err := New(enc, idx)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestRoundTripAndNoPlaintextInCiphertext(t *testing.T) {
	s := newSealer(t)
	sealed, err := s.Seal("8001 0100 1234 5")
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(sealed, []byte("8001")) {
		t.Fatal("plaintext visible in sealed value")
	}
	got, err := s.Open(sealed)
	if err != nil || got != "8001010012345" {
		t.Fatalf("open = %q, %v", got, err)
	}
}

func TestTamperingIsDetected(t *testing.T) {
	s := newSealer(t)
	sealed, _ := s.Seal("8001010012345")
	sealed[len(sealed)-1] ^= 1
	if _, err := s.Open(sealed); err == nil {
		t.Fatal("tampered value must not open")
	}
}

func TestSealingIsNonDeterministic(t *testing.T) {
	s := newSealer(t)
	a, _ := s.Seal("8001010012345")
	b, _ := s.Seal("8001010012345")
	if bytes.Equal(a, b) {
		t.Fatal("two seals of the same value must differ")
	}
}

func TestIndexIsDeterministicAndNormalised(t *testing.T) {
	s := newSealer(t)
	if !bytes.Equal(s.Index("8001 0100 12345"), s.Index("8001010012345")) {
		t.Fatal("index must ignore spacing")
	}
	if bytes.Equal(s.Index("8001010012345"), s.Index("8001010012346")) {
		t.Fatal("different IDs must not share an index")
	}
}

func TestMask(t *testing.T) {
	if got := Mask("8001010012345"); got != "*********2345" {
		t.Fatalf("mask = %q", got)
	}
	if got := Mask("12"); got != "**" {
		t.Fatalf("short mask = %q", got)
	}
}

func TestKeyLengthsChecked(t *testing.T) {
	if _, err := New(make([]byte, 16), make([]byte, 32)); err == nil {
		t.Fatal("16-byte encryption key must be refused")
	}
	if _, err := New(make([]byte, 32), make([]byte, 8)); err == nil {
		t.Fatal("short index key must be refused")
	}
}
