package goaml

import (
	"errors"
	"testing"
)

// Submission must refuse every payload until the schema is verified, including an empty one.
func TestSubmissionAlwaysRefuses(t *testing.T) {
	for _, payload := range [][]byte{nil, []byte("<report/>")} {
		if err := Submit(payload); !errors.Is(err, ErrSchemaUnverified) {
			t.Fatalf("submission must be refused: %v", err)
		}
	}
}
