// Package payments is the domestic payment workflow (ARCHITECTURE.md 9.1 and 9.2). It is deterministic:
// every side effect is an activity, and every activity is idempotent with respect to its inputs.
package payments

import (
	railsv1 "github.com/khayaai/forge-mutual/pkg/api/forge/rails/v1"
)

// Signals and the query name.
const (
	SignalApproval = "approval"
	SignalRepair   = "repair"
	QueryStatus    = "status"
)

// Workflow states, reported in Status.State.
const (
	StateRunning          = "RUNNING"
	StateAwaitingApproval = "AWAITING_APPROVAL"
	StateReserved         = "RESERVED"
	StateCompleted        = "COMPLETED"
	StateDeclined         = "DECLINED"
	StateFailed           = "FAILED"
	StateCancelled        = "CANCELLED"
	StateRepairRequired   = "REPAIR_REQUIRED"
)

// Defaults applied by Input.WithDefaults. They are part of the workflow input, so a replay sees the same values.
const (
	DefaultPollSeconds  = 60
	DefaultApprovalSecs = 24 * 60 * 60
	DefaultMaxPolls     = 30
	// DefaultReserveSeconds is the ledger's maximum pending timeout (ledger gateway MaxPendingTimeout). A hold
	// that outlives the rail's outcome window is released by the ledger; SPEC section 6 records the risk.
	DefaultReserveSeconds = 7 * 24 * 60 * 60
)

// Input starts one payment.
type Input struct {
	PaymentID             string // canonical UUID; also the business transaction id in the ledger
	RequestHash           string
	PartyID               string
	FromAccountID         string // the customer's ledger account
	AmountMinor           string
	Asset                 string
	BeneficiaryName       string
	AccountNumber         string
	BankCode              string
	Reference             string
	Actor                 string
	Scenario              string // rail simulation only; empty on a production rail
	PollSeconds           int64
	ApprovalSeconds       int64
	MaxPolls              int
	ReserveTimeoutSeconds int64
}

// WithDefaults fills the zero fields.
func (in Input) WithDefaults() Input {
	if in.PollSeconds <= 0 {
		in.PollSeconds = DefaultPollSeconds
	}
	if in.ApprovalSeconds <= 0 {
		in.ApprovalSeconds = DefaultApprovalSecs
	}
	if in.MaxPolls <= 0 {
		in.MaxPolls = DefaultMaxPolls
	}
	if in.ReserveTimeoutSeconds <= 0 {
		in.ReserveTimeoutSeconds = DefaultReserveSeconds
	}
	return in
}

// Approval is a checker's decision on a payment that needs step-up.
type Approval struct {
	Approve  bool
	Approver string
}

// Repair is an operator's resolution of a payment whose rail outcome was unknown.
type Repair struct {
	Settled  bool
	Operator string
}

// Status is the workflow's state. It is the query result and the final result.
type Status struct {
	PaymentID   string
	RequestHash string
	State       string
	Reason      string
	RailStatus  string
	InitiatedBy string
	AmountMinor string
	Asset       string
	Steps       []string
}

// PartyInfo is what policy needs about the paying party.
type PartyInfo struct {
	Status string // protobuf enum name, for example PARTY_STATUS_ACTIVE
	Tier   string
}

// Decision is a policy outcome.
type Decision struct {
	Allow  bool
	StepUp bool
	Reason string
}

// ReserveResult says whether the ledger placed the hold.
type ReserveResult struct {
	OK         bool
	TransferID string
	Reason     string
}

// RailResult is the rail's answer about the payment.
type RailResult struct {
	Status    railsv1.RailStatus
	Reference string
}

func isFinal(s railsv1.RailStatus) bool {
	return s == railsv1.RailStatus_RAIL_STATUS_SETTLED || s == railsv1.RailStatus_RAIL_STATUS_REJECTED ||
		s == railsv1.RailStatus_RAIL_STATUS_RETURNED
}
