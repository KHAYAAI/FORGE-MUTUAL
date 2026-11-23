package store

import (
	"encoding/json"

	"github.com/khayaai/forge-mutual/services/bank/reconciliation/internal/recon"
)

type checkJSON struct {
	Check      string `json:"check"`
	Matched    bool   `json:"matched"`
	Difference int64  `json:"difference_minor"`
	Detail     string `json:"detail"`
}

func encodeChecks(rs []recon.Result) []byte {
	out := make([]checkJSON, 0, len(rs))
	for _, r := range rs {
		out = append(out, checkJSON{Check: r.Check, Matched: r.Matched, Difference: r.Difference, Detail: r.Detail})
	}
	b, err := json.Marshal(out)
	if err != nil {
		// A struct of strings, bools and integers always marshals.
		panic(err)
	}
	return b
}
