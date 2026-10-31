// Package evidence builds the immutable evidence pack for each regulatory run (ARCHITECTURE.md 15.2). A pack records
// what went in, the versions that produced it, and who prepared and approved it. Its digest covers every byte, so a
// changed pack fails verification.
package evidence

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"
)

// Input is one named source the report was built from, with the digest of what was read.
type Input struct {
	Name   string `json:"name"`
	Digest string `json:"digest"` // hex SHA-256 of the bytes read
	Count  int64  `json:"count"`
}

// Pack is the evidence for one run.
type Pack struct {
	RunID         string    `json:"run_id"`
	Report        string    `json:"report"`
	Period        string    `json:"period"`
	CodeVersion   string    `json:"code_version"`
	PolicyVersion string    `json:"policy_version"`
	Inputs        []Input   `json:"inputs"`
	Queries       []string  `json:"queries"`
	PreparedBy    string    `json:"prepared_by"`
	PreparedAt    time.Time `json:"prepared_at"`
	ApprovedBy    string    `json:"approved_by,omitempty"`
	ApprovedAt    time.Time `json:"approved_at,omitempty"`
}

// Errors.
var (
	ErrSameApprover = errors.New("evidence: the preparer cannot approve their own pack")
	ErrNotApproved  = errors.New("evidence: the pack has no approval")
	ErrAltered      = errors.New("evidence: the pack no longer matches its digest")
)

// New starts a pack. Inputs are sorted by name, so the same inputs always produce the same bytes.
func New(runID, report, period, codeVersion, policyVersion, preparedBy string, inputs []Input, queries []string, at time.Time) (Pack, error) {
	if runID == "" || report == "" || period == "" || preparedBy == "" {
		return Pack{}, errors.New("evidence: run, report, period and preparer are required")
	}
	in := append([]Input(nil), inputs...)
	sort.Slice(in, func(i, j int) bool { return in[i].Name < in[j].Name })
	qs := append([]string(nil), queries...)
	return Pack{
		RunID: runID, Report: report, Period: period, CodeVersion: codeVersion, PolicyVersion: policyVersion,
		Inputs: in, Queries: qs, PreparedBy: preparedBy, PreparedAt: at.UTC(),
	}, nil
}

// Approve records a second person's approval. The preparer cannot approve.
func (p *Pack) Approve(by string, at time.Time) error {
	if by == "" {
		return errors.New("evidence: approver is required")
	}
	if by == p.PreparedBy {
		return ErrSameApprover
	}
	p.ApprovedBy = by
	p.ApprovedAt = at.UTC()
	return nil
}

// Canonical returns the pack's JSON encoding. encoding/json writes struct fields in declaration order and sorts map keys,
// so the bytes are stable for the same pack.
func (p Pack) Canonical() ([]byte, error) {
	return json.Marshal(p)
}

// Digest is the hex SHA-256 of the canonical encoding.
func (p Pack) Digest() (string, error) {
	b, err := p.Canonical()
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

// Verify checks stored bytes against an expected digest. It refuses a pack that changed, and a pack without approval
// when approval is required.
func Verify(stored []byte, expectedDigest string, requireApproval bool) (Pack, error) {
	sum := sha256.Sum256(stored)
	if hex.EncodeToString(sum[:]) != expectedDigest {
		return Pack{}, ErrAltered
	}
	var p Pack
	dec := json.NewDecoder(bytes.NewReader(stored))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&p); err != nil {
		return Pack{}, fmt.Errorf("evidence: decode: %w", err)
	}
	if requireApproval && p.ApprovedBy == "" {
		return p, ErrNotApproved
	}
	return p, nil
}
