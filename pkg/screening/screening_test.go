package screening

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestHitsAreFoundDespiteCaseAndSpacing(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	f, err := NewFixture([]string{"Jane  Doe"}, now, time.Hour, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	res, err := f.Screen(context.Background(), []string{"jane doe", "Thandi Dube"})
	if err != nil || len(res.Hits) != 1 || res.Hits[0] != "jane doe" {
		t.Fatalf("result = %+v, err = %v", res, err)
	}
}

func TestStaleListFailsClosed(t *testing.T) {
	asOf := time.Unix(1_800_000_000, 0)
	later := asOf.Add(2 * time.Hour)
	f, _ := NewFixture(nil, asOf, time.Hour, func() time.Time { return later })
	if _, err := f.Screen(context.Background(), []string{"Anyone"}); !errors.Is(err, ErrStale) {
		t.Fatalf("stale list must fail closed: %v", err)
	}
}

func TestMaxAgeMustBePositive(t *testing.T) {
	if _, err := NewFixture(nil, time.Now(), 0, time.Now); err == nil {
		t.Fatal("zero max age must be refused")
	}
}
