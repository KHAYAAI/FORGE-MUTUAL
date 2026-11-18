// Package returns is the framework for prudential returns to the Prudential Authority (BA 100 to BA 950 for a bank, DI
// forms for a mutual bank). It is the frame, not the forms: a return is a set of line items, each computed from named
// inputs by a named rule, then certified by two different people and tied to an evidence pack.
//
// What this package does NOT do: it holds no line-item definitions. The line numbers, definitions and aggregation rules come
// from the form instructions in the PA's returns guidance, which have not been read for this repository (VERIFY-FORMS). A
// Spec is written by someone who has read them. Nothing here submits a return to the PA (VERIFY-PA-CHANNEL).
//
// The standard library only, so it builds and tests without the rest of the workspace.
package returns

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

var (
	ErrUnverified  = errors.New("returns: line definitions are not verified against the form instructions")
	ErrIncomplete  = errors.New("returns: return is incomplete")
	ErrSameSigner  = errors.New("returns: the approver must differ from the preparer")
	ErrNotApproved = errors.New("returns: return is not approved")
	ErrBreak       = errors.New("returns: reconciliation break")
	ErrNotAudited  = errors.New("returns: the external auditor has not reported on this return")
)

// Cents are minor units of the return's currency (rand cents). Integers only: no floating point near money.
type Cents int64

// Line is one line of a form. Compute reads named inputs and returns a value. Inputs are the only data it may use.
type Line struct {
	Ref       string // the form's own line reference, for example "BA100 12" (to be filled from the form instructions)
	Label     string
	Inputs    []string // names of inputs this line reads
	Compute   func(in map[string]Cents) (Cents, error)
	Verified  bool // true only once checked against the form instructions by a named person
	CheckedBy string
}

// Total says that one line must equal the sum of others: the cross-checks a form's own arithmetic gives.
type Total struct {
	Line string
	Sum  []string
}

// Spec defines a return.
type Spec struct {
	Form      string // for example "BA 900"
	Name      string
	Frequency string // "monthly", "quarterly", "annual"
	// Audited is true for returns the external auditor must report on before submission (MR-05; which forms, VERIFY).
	Audited bool
	Lines   []Line
	Totals  []Total
	// Reconcile ties return lines to other sources of truth, such as the ledger. Each returns the figure from the other source.
	Reconcile map[string]func(in map[string]Cents) (Cents, error)
}

// Result is a computed return.
type Result struct {
	Form, Period string
	Audited      bool
	Values       map[string]Cents
	Unverified   []string // lines not yet verified against the form instructions
	Breaks       []string
	ComputedAt   time.Time
}

// Compute evaluates every line from inputs. It fails if an input a line needs is missing. It lists unverified lines and
// arithmetic or reconciliation breaks in the result rather than hiding them.
func (s Spec) Compute(period string, in map[string]Cents, at time.Time) (Result, error) {
	r := Result{Form: s.Form, Period: period, Audited: s.Audited, Values: map[string]Cents{}, ComputedAt: at}
	for _, l := range s.Lines {
		for _, name := range l.Inputs {
			if _, ok := in[name]; !ok {
				return Result{}, fmt.Errorf("%w: %s needs input %q", ErrIncomplete, l.Ref, name)
			}
		}
		v, err := l.Compute(in)
		if err != nil {
			return Result{}, fmt.Errorf("%s: %w", l.Ref, err)
		}
		r.Values[l.Ref] = v
		if !l.Verified || strings.TrimSpace(l.CheckedBy) == "" {
			r.Unverified = append(r.Unverified, l.Ref)
		}
	}
	for _, t := range s.Totals {
		var sum Cents
		for _, ref := range t.Sum {
			v, ok := r.Values[ref]
			if !ok {
				return Result{}, fmt.Errorf("%w: total %s refers to unknown line %s", ErrIncomplete, t.Line, ref)
			}
			sum += v
		}
		got, ok := r.Values[t.Line]
		if !ok {
			return Result{}, fmt.Errorf("%w: total refers to unknown line %s", ErrIncomplete, t.Line)
		}
		if got != sum {
			r.Breaks = append(r.Breaks, fmt.Sprintf("%s is %d but its parts sum to %d", t.Line, got, sum))
		}
	}
	refs := make([]string, 0, len(s.Reconcile))
	for ref := range s.Reconcile {
		refs = append(refs, ref)
	}
	sort.Strings(refs)
	for _, ref := range refs {
		other, err := s.Reconcile[ref](in)
		if err != nil {
			return Result{}, fmt.Errorf("reconcile %s: %w", ref, err)
		}
		v, ok := r.Values[ref]
		if !ok {
			return Result{}, fmt.Errorf("%w: reconcile refers to unknown line %s", ErrIncomplete, ref)
		}
		if v != other {
			r.Breaks = append(r.Breaks, fmt.Sprintf("%s is %d but the other source says %d", ref, v, other))
		}
	}
	return r, nil
}

// Certification is the sign-off: a preparer and a different approver, tied to the evidence pack digest the figures came from.
type Certification struct {
	Result       Result
	PreparedBy   string
	ApprovedBy   string
	ApprovedAt   time.Time
	EvidenceHash string
	// The external auditor's report, for audited returns: the firm, the signing partner and when.
	AuditFirm, AuditPartner string
	AuditedAt               time.Time
}

// Prepare starts a certification. It refuses a result that has breaks or arithmetic errors.
func Prepare(r Result, preparedBy, evidenceDigest string) (Certification, error) {
	if len(r.Breaks) > 0 {
		return Certification{}, fmt.Errorf("%w: %s", ErrBreak, strings.Join(r.Breaks, "; "))
	}
	if strings.TrimSpace(preparedBy) == "" || strings.TrimSpace(evidenceDigest) == "" {
		return Certification{}, fmt.Errorf("%w: preparer and evidence digest are required", ErrIncomplete)
	}
	return Certification{Result: r, PreparedBy: preparedBy, EvidenceHash: evidenceDigest}, nil
}

// Approve records the approver. The approver must differ from the preparer.
func (c *Certification) Approve(by string, at time.Time) error {
	by = strings.TrimSpace(by)
	if by == "" || strings.EqualFold(by, strings.TrimSpace(c.PreparedBy)) {
		return ErrSameSigner
	}
	c.ApprovedBy, c.ApprovedAt = by, at
	return nil
}

// Submittable reports whether the return could go to the PA: approved, no breaks, and every line verified against the form
// instructions. Until the lines are verified it returns ErrUnverified, which is the honest state of every return today.
func (c Certification) Submittable() error {
	if c.ApprovedBy == "" {
		return ErrNotApproved
	}
	if c.Result.Audited && c.AuditPartner == "" {
		return ErrNotAudited
	}
	if len(c.Result.Unverified) > 0 {
		return fmt.Errorf("%w: %s", ErrUnverified, strings.Join(c.Result.Unverified, ", "))
	}
	return nil
}

// AuditorReport records the external auditor's report on an approved return. The auditor must be outside Forge: the
// partner may not be the preparer or the approver.
func (c *Certification) AuditorReport(firm, partner string, at time.Time) error {
	firm, partner = strings.TrimSpace(firm), strings.TrimSpace(partner)
	if c.ApprovedBy == "" {
		return ErrNotApproved
	}
	if firm == "" || partner == "" || strings.EqualFold(partner, c.PreparedBy) || strings.EqualFold(partner, c.ApprovedBy) {
		return fmt.Errorf("%w: an independent audit firm and partner are required", ErrSameSigner)
	}
	c.AuditFirm, c.AuditPartner, c.AuditedAt = firm, partner, at
	return nil
}
