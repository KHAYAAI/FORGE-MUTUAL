// Package goaml is the submission boundary for goAML reports. It refuses to submit until the official schema and the
// submission process are verified (VERIFY-GOAML). A report that cannot be validated must not reach the regulator.
package goaml

import "errors"

// ErrSchemaUnverified is returned on every submission while VERIFY-GOAML is open.
var ErrSchemaUnverified = errors.New("goaml: submission refused: the official schema and process are not yet verified (VERIFY-GOAML)")

// Submit always refuses. It exists so no caller can route around the check, and so the refusal is in one place.
func Submit(_ []byte) error { return ErrSchemaUnverified }
