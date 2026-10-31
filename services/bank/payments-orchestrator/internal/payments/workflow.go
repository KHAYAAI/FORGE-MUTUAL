package payments

import (
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	railsv1 "github.com/khayaai/forge-mutual/pkg/api/forge/rails/v1"
)

var activityRetry = temporal.RetryPolicy{
	InitialInterval:    time.Second,
	BackoffCoefficient: 2,
	MaximumInterval:    time.Minute,
	MaximumAttempts:    5,
	NonRetryableErrorTypes: []string{
		ErrTypeValidation,
	},
}

// DomesticPayment moves money out of a customer's account by EFT. Its order is the one in ARCHITECTURE.md 9.2:
// validate, check policy and screening, reserve, execute on the rail, wait for the outcome, then settle or
// release. Two rules keep money safe:
//
//   - A failure before the hold is placed declines the payment. Nothing has moved.
//   - A failure after the hold, or a rail outcome that never arrives, never releases the hold on its own.
//     The payment goes to REPAIR_REQUIRED and waits for an operator. The rail may already have moved the money.
func DomesticPayment(ctx workflow.Context, in Input) (Status, error) {
	in = in.WithDefaults()
	st := Status{
		PaymentID: in.PaymentID, RequestHash: in.RequestHash, State: StateRunning,
		InitiatedBy: in.Actor, AmountMinor: in.AmountMinor, Asset: in.Asset,
	}
	if err := workflow.SetQueryHandler(ctx, QueryStatus, func() (Status, error) { return st, nil }); err != nil {
		return st, err
	}
	actx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy:         &activityRetry,
	})
	var acts *Activities

	var transferID string // the ledger hold, once placed

	record := func(state, reason string) error {
		return workflow.ExecuteActivity(actx, acts.Record, in, state, reason, st.RailStatus).Get(actx, nil)
	}
	// decline ends a payment before any money is held.
	decline := func(state, reason string) (Status, error) {
		st.State, st.Reason = state, reason
		if err := record(state, reason); err != nil {
			return st, err
		}
		return st, nil
	}
	// repair parks a payment whose hold exists but whose outcome is unknown or whose last step failed.
	repair := func(reason string) (Status, error) {
		st.State, st.Reason = StateRepairRequired, reason
		if err := record(StateRepairRequired, reason); err != nil {
			return st, err
		}
		ch := workflow.GetSignalChannel(ctx, SignalRepair)
		var r Repair
		ch.Receive(ctx, &r)
		st.Steps = append(st.Steps, "repair_resolved")
		if r.Settled {
			return settle(ctx, actx, acts, &st, in, transferID)
		}
		return release(actx, acts, &st, in, transferID, StateFailed, "repair_not_settled")
	}

	// 1. The party must be active.
	var party PartyInfo
	if err := workflow.ExecuteActivity(actx, acts.LoadParty, in.PartyID).Get(actx, &party); err != nil {
		return decline(StateFailed, "party_unavailable")
	}
	st.Steps = append(st.Steps, "party_loaded")
	if party.Status != "PARTY_STATUS_ACTIVE" {
		return decline(StateDeclined, "party_not_active")
	}

	// 2. Beneficiary screening. An unavailable list declines: it never passes.
	var hits []string
	if err := workflow.ExecuteActivity(actx, acts.Screen, in).Get(actx, &hits); err != nil {
		return decline(StateDeclined, "screening_unavailable")
	}
	if len(hits) > 0 {
		return decline(StateDeclined, "beneficiary_screening_match")
	}
	st.Steps = append(st.Steps, "screened")

	// 3. Asset and entity policy.
	var ae Decision
	if err := workflow.ExecuteActivity(actx, acts.CheckAssetEntity, in).Get(actx, &ae); err != nil {
		return decline(StateFailed, "policy_unavailable")
	}
	if !ae.Allow {
		return decline(StateDeclined, "policy:"+ae.Reason)
	}

	// 4. Limits, including the day's velocity and the reconciliation break.
	var lim Decision
	if err := workflow.ExecuteActivity(actx, acts.CheckLimits, in, party).Get(actx, &lim); err != nil {
		return decline(StateFailed, "policy_unavailable")
	}
	if !lim.Allow {
		return decline(StateDeclined, "limits:"+lim.Reason)
	}
	st.Steps = append(st.Steps, "policy_allowed")

	// 5. Step-up: a checker other than the initiator must approve.
	if lim.StepUp {
		st.State = StateAwaitingApproval
		if err := record(StateAwaitingApproval, "step_up"); err != nil {
			return st, err
		}
		approved, reason := waitForApproval(ctx, in.Actor, in.ApprovalSeconds)
		if !approved {
			return decline(StateCancelled, reason)
		}
		st.Steps = append(st.Steps, "approved")
		st.State = StateRunning
	}

	// 6. Reserve: place the hold.
	var res ReserveResult
	if err := workflow.ExecuteActivity(actx, acts.Reserve, in).Get(actx, &res); err != nil {
		return decline(StateFailed, "reserve_unavailable")
	}
	if !res.OK {
		return decline(StateDeclined, res.Reason)
	}
	transferID = res.TransferID
	st.State = StateReserved
	st.Steps = append(st.Steps, "reserved")

	// 7. Send the instruction to the rail. If the call fails, the instruction may have reached the rail,
	// so the outcome is unknown and the payment goes to repair rather than release.
	var rr RailResult
	if err := workflow.ExecuteActivity(actx, acts.RailInitiate, in).Get(actx, &rr); err != nil {
		return repair("rail_initiation_unknown")
	}
	st.RailStatus = rr.Status.String()
	st.Steps = append(st.Steps, "rail_initiated")
	if rr.Status == railsv1.RailStatus_RAIL_STATUS_REJECTED {
		return release(actx, acts, &st, in, transferID, StateFailed, "rail_rejected_at_initiation")
	}

	// 8. Wait for a final outcome: poll the rail until it reaches one, or give up after MaxPolls.
	for polls := 0; !isFinal(rr.Status); polls++ {
		if polls >= in.MaxPolls {
			return repair("rail_outcome_not_received")
		}
		if err := workflow.Sleep(actx, time.Duration(in.PollSeconds)*time.Second); err != nil {
			return st, err
		}
		if err := workflow.ExecuteActivity(actx, acts.RailStatus, in).Get(actx, &rr); err != nil {
			continue // a failed poll is retried by the next one; the hold is untouched
		}
		st.RailStatus = rr.Status.String()
	}
	st.Steps = append(st.Steps, "rail_final")

	// 9. Settle or release, from the final outcome.
	if rr.Status == railsv1.RailStatus_RAIL_STATUS_SETTLED {
		return settle(ctx, actx, acts, &st, in, transferID)
	}
	return release(actx, acts, &st, in, transferID, StateFailed, "rail_"+railWord(rr.Status))
}

// settle posts the hold. If the posting fails, the payment goes to repair: the rail has settled, so the
// money must not be released.
func settle(ctx workflow.Context, actx workflow.Context, acts *Activities, st *Status, in Input, transferID string) (Status, error) {
	if err := workflow.ExecuteActivity(actx, acts.Settle, in, transferID, st.RailStatus).Get(actx, nil); err != nil {
		st.State, st.Reason = StateRepairRequired, "settlement_posting_failed"
		return *st, nil
	}
	st.State, st.Reason = StateCompleted, ""
	st.Steps = append(st.Steps, "settled")
	return *st, nil
}

// release voids the hold and records the outcome.
func release(actx workflow.Context, acts *Activities, st *Status, in Input, transferID, state, reason string) (Status, error) {
	if err := workflow.ExecuteActivity(actx, acts.Release, in, transferID, state, reason, st.RailStatus).Get(actx, nil); err != nil {
		st.State, st.Reason = StateRepairRequired, "release_failed:"+reason
		return *st, nil
	}
	st.State, st.Reason = state, reason
	st.Steps = append(st.Steps, "released")
	return *st, nil
}

// waitForApproval waits for a checker's decision. A decision by the initiator is ignored: nobody approves
// their own payment.
func waitForApproval(ctx workflow.Context, initiator string, seconds int64) (bool, string) {
	ch := workflow.GetSignalChannel(ctx, SignalApproval)
	timer := workflow.NewTimer(ctx, time.Duration(seconds)*time.Second)
	for {
		var a Approval
		timedOut := false
		sel := workflow.NewSelector(ctx)
		sel.AddReceive(ch, func(c workflow.ReceiveChannel, _ bool) { c.Receive(ctx, &a) })
		sel.AddFuture(timer, func(workflow.Future) { timedOut = true })
		sel.Select(ctx)
		if timedOut {
			return false, "approval_timeout"
		}
		if a.Approver == "" || a.Approver == initiator {
			continue
		}
		if !a.Approve {
			return false, "approval_refused"
		}
		return true, ""
	}
}

func railWord(s railsv1.RailStatus) string {
	switch s {
	case railsv1.RailStatus_RAIL_STATUS_REJECTED:
		return "rejected"
	case railsv1.RailStatus_RAIL_STATUS_RETURNED:
		return "returned"
	}
	return "unknown"
}

// ErrTypeValidation marks errors that retrying cannot fix.
const ErrTypeValidation = "Validation"
