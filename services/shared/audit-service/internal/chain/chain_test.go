package chain

import (
	"errors"
	"testing"
	"time"
)

var t0 = time.Date(2026, 10, 31, 12, 0, 0, 123456000, time.UTC)

func build(n int) []Entry {
	var out []Entry
	prev := Genesis
	for i := 0; i < n; i++ {
		e := Entry{
			Seq: uint64(i + 1), At: t0.Add(time.Duration(i) * time.Second), Actor: "svc-payments",
			Action: "payment.approved", Entity: "BANK", Payload: []byte(`{"n":1}`), PrevHash: prev,
		}
		Seal(&e)
		out = append(out, e)
		prev = e.Hash
	}
	return out
}

func TestIntactChainVerifies(t *testing.T) {
	if _, err := Verify(Genesis, 1, build(5)); err != nil {
		t.Fatalf("intact chain: %v", err)
	}
}

// Each kind of change must be caught, at the sequence number that changed.
func TestEveryKindOfChangeIsCaught(t *testing.T) {
	cases := map[string]func([]Entry){
		"payload": func(es []Entry) { es[2].Payload = []byte(`{"n":2}`) },
		"actor":   func(es []Entry) { es[2].Actor = "svc-attacker" },
		"action":  func(es []Entry) { es[2].Action = "payment.none" },
		"time":    func(es []Entry) { es[2].At = es[2].At.Add(time.Microsecond) },
		"removed": func(es []Entry) { es[1] = es[2]; es[1].Seq = 2 },
	}
	for name, mutate := range cases {
		es := build(4)
		mutate(es)
		_, err := Verify(Genesis, 1, es)
		if !errors.Is(err, ErrBroken) {
			t.Fatalf("%s: change not caught: %v", name, err)
		}
	}
}

// Recomputing a single entry's hash is not enough to hide a change, because the next entry commits to the old hash.
func TestResealingOneEntryBreaksTheNext(t *testing.T) {
	es := build(4)
	es[1].Action = "payment.none"
	Seal(&es[1])
	if _, err := Verify(Genesis, 1, es); !errors.Is(err, ErrBroken) {
		t.Fatalf("a resealed entry must still break the link to the next: %v", err)
	}
}

// Length prefixes stop two different entries encoding to the same bytes.
func TestFieldBoundariesAreUnambiguous(t *testing.T) {
	a := Hash(Genesis, 1, t0, "ab", "c", "BANK", [32]byte{})
	b := Hash(Genesis, 1, t0, "a", "bc", "BANK", [32]byte{})
	if a == b {
		t.Fatal("moving a boundary between fields must change the hash")
	}
}

func TestValidateRefusesBadInput(t *testing.T) {
	good := Input{Actor: "svc", Action: "a", Entity: "BANK", Payload: []byte("{}")}
	if err := good.Validate(); err != nil {
		t.Fatal(err)
	}
	bad := []Input{
		{Action: "a", Entity: "BANK", Payload: []byte("{}")},
		{Actor: "svc", Entity: "BANK", Payload: []byte("{}")},
		{Actor: "svc", Action: "a", Payload: []byte("{}")},
		{Actor: "svc", Action: "a", Entity: "BANK"},
		{Actor: "svc", Action: "a", Entity: "BANK", Payload: make([]byte, MaxPayload+1)},
	}
	for i, in := range bad {
		if in.Validate() == nil {
			t.Fatalf("case %d accepted", i)
		}
	}
}

func TestMicrosTruncatesToStoredPrecision(t *testing.T) {
	ns := time.Date(2026, 1, 1, 0, 0, 0, 1500, time.UTC)
	if got := Micros(ns); got.Nanosecond() != 1000 {
		t.Fatalf("Micros = %v", got)
	}
}
