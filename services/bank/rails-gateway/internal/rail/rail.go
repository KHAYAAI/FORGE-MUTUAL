// Package rail is the payment rail boundary (ARCHITECTURE.md 12.1). RailSim is the simulator used until a
// sponsor adapter is certified. Its scenarios are deterministic, so tests can drive every outcome.
package rail

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"

	railsv1 "github.com/khayaai/forge-mutual/pkg/api/forge/rails/v1"
	"github.com/khayaai/forge-mutual/pkg/money"
)

// Errors returned by a rail.
var (
	ErrConflict        = errors.New("rail: instruction id reused with a different instruction")
	ErrNotFound        = errors.New("rail: unknown instruction")
	ErrScenarioRefused = errors.New("rail: scenarios are refused on this rail")
	ErrBadScenario     = errors.New("rail: unknown scenario")
)

// Instruction is one payment sent to the rail.
type Instruction struct {
	ID              string
	Rail            string
	AmountMinor     string
	Asset           string
	BeneficiaryName string
	AccountNumber   string
	BankCode        string
	Scenario        string
}

// Result is the rail's answer about an instruction.
type Result struct {
	Status    railsv1.RailStatus
	Reference string
}

// Rail sends instructions and reports their status. Initiate is idempotent by instruction ID.
type Rail interface {
	Initiate(in Instruction) (Result, error)
	Status(id string) (Result, error)
}

var (
	accountRe = regexp.MustCompile(`^[0-9]{6,16}$`)
	bankRe    = regexp.MustCompile(`^[A-Za-z0-9]{1,12}$`)
)

// plan is a parsed scenario.
type plan struct {
	kind  string // settle, reject, return, stall, reject_at_initiation
	after int    // status polls before a final outcome
}

// ParseScenario reads a scenario name. "" means the default, settle after one poll.
//
//	settle_after:N   settled on the (N+1)th status poll
//	reject_after:N   rejected on the (N+1)th status poll
//	return_after:N   returned on the (N+1)th status poll
//	stall            never final; stays PENDING
//	reject_at_initiation  refused by the rail on submission
func ParseScenario(s string) (plan, error) {
	if s == "" {
		return plan{kind: "settle", after: 1}, nil
	}
	if s == "stall" {
		return plan{kind: "stall"}, nil
	}
	if s == "reject_at_initiation" {
		return plan{kind: "reject_at_initiation"}, nil
	}
	name, count, ok := strings.Cut(s, ":")
	if !ok {
		return plan{}, fmt.Errorf("%w: %q", ErrBadScenario, s)
	}
	n, err := strconv.Atoi(count)
	if err != nil || n < 0 || n > 10 {
		return plan{}, fmt.Errorf("%w: %q", ErrBadScenario, s)
	}
	switch name {
	case "settle_after":
		return plan{kind: "settle", after: n}, nil
	case "reject_after":
		return plan{kind: "reject", after: n}, nil
	case "return_after":
		return plan{kind: "return", after: n}, nil
	}
	return plan{}, fmt.Errorf("%w: %q", ErrBadScenario, s)
}

type simInstruction struct {
	hash      [32]byte
	plan      plan
	reference string
	polls     int
	final     *railsv1.RailStatus
}

// Sim is the rail simulator.
type Sim struct {
	mu             sync.Mutex
	allowScenarios bool
	instructions   map[string]*simInstruction
}

var _ Rail = (*Sim)(nil)

// NewSim returns a simulator. Scenarios are refused unless allowScenarios is set, so a scripted outcome can
// never reach a production rail.
func NewSim(allowScenarios bool) *Sim {
	return &Sim{allowScenarios: allowScenarios, instructions: map[string]*simInstruction{}}
}

func hashOf(in Instruction) [32]byte {
	return sha256.Sum256([]byte(strings.Join([]string{in.ID, in.Rail, in.AmountMinor, in.Asset,
		in.BeneficiaryName, in.AccountNumber, in.BankCode, in.Scenario}, "\x00")))
}

// validate reports why the rail would refuse an instruction, or "" when it is valid.
func validate(in Instruction) string {
	switch {
	case in.Rail != "EFT":
		return "unsupported rail"
	case in.Asset != "ZAR":
		return "EFT carries ZAR only"
	case strings.TrimSpace(in.BeneficiaryName) == "" || len(in.BeneficiaryName) > 100:
		return "beneficiary name required, up to 100 characters"
	case !accountRe.MatchString(in.AccountNumber):
		return "account number must be 6 to 16 digits"
	case !bankRe.MatchString(in.BankCode):
		return "bank code required"
	}
	m, err := money.Parse(in.Asset, in.AmountMinor)
	if err != nil || m.Minor() <= 0 {
		return "amount must be a positive integer of minor units"
	}
	return ""
}

// Initiate implements Rail.
func (s *Sim) Initiate(in Instruction) (Result, error) {
	if in.Scenario != "" && !s.allowScenarios {
		return Result{}, ErrScenarioRefused
	}
	p, err := ParseScenario(in.Scenario)
	if err != nil {
		return Result{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	h := hashOf(in)
	if prev, ok := s.instructions[in.ID]; ok {
		if prev.hash != h {
			return Result{}, ErrConflict
		}
		return s.resultLocked(prev), nil
	}
	rec := &simInstruction{hash: h, plan: p, reference: "RS-" + in.ID[:8]}
	s.instructions[in.ID] = rec
	if reason := validate(in); reason != "" || p.kind == "reject_at_initiation" {
		final := railsv1.RailStatus_RAIL_STATUS_REJECTED
		rec.final = &final
		rec.reference = ""
		return Result{Status: final}, nil
	}
	return s.resultLocked(rec), nil
}

// Status implements Rail. Each call is one poll: a scenario reaches its final outcome after the planned
// number of polls.
func (s *Sim) Status(id string) (Result, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, ok := s.instructions[id]
	if !ok {
		return Result{}, ErrNotFound
	}
	if rec.final == nil && rec.plan.kind != "stall" {
		rec.polls++
		if rec.polls > rec.plan.after {
			var final railsv1.RailStatus
			switch rec.plan.kind {
			case "settle":
				final = railsv1.RailStatus_RAIL_STATUS_SETTLED
			case "reject":
				final = railsv1.RailStatus_RAIL_STATUS_REJECTED
			case "return":
				final = railsv1.RailStatus_RAIL_STATUS_RETURNED
			}
			rec.final = &final
		}
	}
	return s.resultLocked(rec), nil
}

func (s *Sim) resultLocked(rec *simInstruction) Result {
	if rec.final != nil {
		return Result{Status: *rec.final, Reference: rec.reference}
	}
	if rec.polls == 0 && rec.plan.kind != "stall" {
		return Result{Status: railsv1.RailStatus_RAIL_STATUS_ACCEPTED, Reference: rec.reference}
	}
	return Result{Status: railsv1.RailStatus_RAIL_STATUS_PENDING, Reference: rec.reference}
}
