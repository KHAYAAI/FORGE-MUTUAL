package workflow

import (
	"errors"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/mock"
	"go.temporal.io/sdk/testsuite"

	"github.com/khayaai/forge-mutual/services/bank/payouts-service/internal/store"
)

// These tests run the workflows in Temporal's test environment, with the activities mocked. They need no server.

func newEnv(t *testing.T, acts *Activities) *testsuite.TestWorkflowEnvironment {
	t.Helper()
	var s testsuite.WorkflowTestSuite
	env := s.NewTestWorkflowEnvironment()
	env.RegisterWorkflow(BulkPayout)
	env.RegisterWorkflow(PayoutItem)
	env.RegisterActivity(acts)
	return env
}

func requireState(t *testing.T, env *testsuite.TestWorkflowEnvironment, want string) {
	t.Helper()
	if !env.IsWorkflowCompleted() {
		t.Fatal("workflow did not complete")
	}
	if err := env.GetWorkflowError(); err != nil {
		t.Fatalf("workflow error: %v", err)
	}
	var got string
	if err := env.GetWorkflowResult(&got); err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("batch state = %s, want %s", got, want)
	}
}

func input() BatchInput {
	return BatchInput{BatchID: "b1", Run: 1, Concurrency: 2, PollInterval: time.Second, MaxPolls: 3}
}

// Every item settles, and the batch completes.
func TestEveryItemSettles(t *testing.T) {
	acts := &Activities{}
	env := newEnv(t, acts)
	env.OnActivity(acts.OpenItems, mock.Anything, "b1").Return([]string{"k1", "k2"}, nil).Once()
	env.OnActivity(acts.SendItem, mock.Anything, SendInput{BatchID: "b1", ItemKey: "k1"}).
		Return(SendResult{Outcome: "sent", PaymentID: "p1"}, nil).Once()
	env.OnActivity(acts.SendItem, mock.Anything, SendInput{BatchID: "b1", ItemKey: "k2"}).
		Return(SendResult{Outcome: "sent", PaymentID: "p2"}, nil).Once()
	env.OnActivity(acts.PollItem, mock.Anything, PollInput{BatchID: "b1", ItemKey: "k1", PaymentID: "p1"}).
		Return(PollResult{Outcome: "settled"}, nil).Once()
	env.OnActivity(acts.PollItem, mock.Anything, PollInput{BatchID: "b1", ItemKey: "k2", PaymentID: "p2"}).
		Return(PollResult{Outcome: "settled"}, nil).Once()
	env.OnActivity(acts.FinishRun, mock.Anything, "b1", uint32(1)).Return(store.StateCompleted, nil).Once()

	env.ExecuteWorkflow(BulkPayout, input())
	requireState(t, env, store.StateCompleted)
	env.AssertExpectations(t)
}

// A refused item is final and is not polled. The other items still go out, and the batch ends with exceptions.
func TestRefusedItemDoesNotStopTheRest(t *testing.T) {
	acts := &Activities{}
	env := newEnv(t, acts)
	env.OnActivity(acts.OpenItems, mock.Anything, "b1").Return([]string{"k1", "k2"}, nil).Once()
	env.OnActivity(acts.SendItem, mock.Anything, SendInput{BatchID: "b1", ItemKey: "k1"}).
		Return(SendResult{Outcome: "refused", Reason: "invalid_argument: bad account"}, nil).Once()
	env.OnActivity(acts.SendItem, mock.Anything, SendInput{BatchID: "b1", ItemKey: "k2"}).
		Return(SendResult{Outcome: "sent", PaymentID: "p2"}, nil).Once()
	env.OnActivity(acts.PollItem, mock.Anything, PollInput{BatchID: "b1", ItemKey: "k2", PaymentID: "p2"}).
		Return(PollResult{Outcome: "settled"}, nil).Once()
	env.OnActivity(acts.FinishRun, mock.Anything, "b1", uint32(1)).Return(store.StateExceptions, nil).Once()

	env.ExecuteWorkflow(BulkPayout, input())
	requireState(t, env, store.StateExceptions)
	env.AssertExpectations(t)
}

// When payments cannot be reached after the retries, the item is marked uncertain and the batch carries on. Nothing
// polls an item whose send never got an answer.
func TestUnreachablePaymentsMarksTheItemUncertain(t *testing.T) {
	acts := &Activities{}
	env := newEnv(t, acts)
	env.OnActivity(acts.OpenItems, mock.Anything, "b1").Return([]string{"k1"}, nil).Once()
	env.OnActivity(acts.SendItem, mock.Anything, SendInput{BatchID: "b1", ItemKey: "k1"}).
		Return(SendResult{}, errors.New("connection refused"))
	env.OnActivity(acts.MarkUncertain, mock.Anything,
		MarkInput{BatchID: "b1", ItemKey: "k1", Reason: "payments could not be reached"}).Return(nil).Once()
	env.OnActivity(acts.FinishRun, mock.Anything, "b1", uint32(1)).Return(store.StateExceptions, nil).Once()

	env.ExecuteWorkflow(BulkPayout, input())
	requireState(t, env, store.StateExceptions)
	env.AssertExpectations(t)
}

// An item still in progress after its polls is left sent, not failed. A resume follows it again.
func TestPendingItemIsLeftForAResume(t *testing.T) {
	acts := &Activities{}
	env := newEnv(t, acts)
	env.OnActivity(acts.OpenItems, mock.Anything, "b1").Return([]string{"k1"}, nil).Once()
	env.OnActivity(acts.SendItem, mock.Anything, SendInput{BatchID: "b1", ItemKey: "k1"}).
		Return(SendResult{Outcome: "sent", PaymentID: "p1"}, nil).Once()
	env.OnActivity(acts.PollItem, mock.Anything, PollInput{BatchID: "b1", ItemKey: "k1", PaymentID: "p1"}).
		Return(PollResult{Outcome: "pending"}, nil).Times(3)
	env.OnActivity(acts.FinishRun, mock.Anything, "b1", uint32(1)).Return(store.StateExceptions, nil).Once()

	env.ExecuteWorkflow(BulkPayout, input())
	requireState(t, env, store.StateExceptions)
	env.AssertExpectations(t)
}

// An item that is already settled is not sent again: the child stops at once.
func TestSettledItemIsNotSentAgain(t *testing.T) {
	acts := &Activities{}
	env := newEnv(t, acts)
	env.OnActivity(acts.OpenItems, mock.Anything, "b1").Return([]string{"k1"}, nil).Once()
	env.OnActivity(acts.SendItem, mock.Anything, SendInput{BatchID: "b1", ItemKey: "k1"}).
		Return(SendResult{Outcome: "settled"}, nil).Once()
	env.OnActivity(acts.FinishRun, mock.Anything, "b1", uint32(1)).Return(store.StateCompleted, nil).Once()

	env.ExecuteWorkflow(BulkPayout, input())
	requireState(t, env, store.StateCompleted)
	env.AssertExpectations(t)
}

func TestRefusedIsFinalAndOtherCodesAreNot(t *testing.T) {
	for _, code := range []connect.Code{connect.CodeInvalidArgument, connect.CodeAlreadyExists,
		connect.CodeFailedPrecondition, connect.CodePermissionDenied, connect.CodeNotFound} {
		if !Refused(connect.NewError(code, errors.New("x"))) {
			t.Errorf("code %v should be a final refusal", code)
		}
	}
	for _, code := range []connect.Code{connect.CodeUnavailable, connect.CodeDeadlineExceeded, connect.CodeInternal} {
		if Refused(connect.NewError(code, errors.New("x"))) {
			t.Errorf("code %v must not be treated as a refusal: the outcome is unknown", code)
		}
	}
	if Refused(nil) {
		t.Error("nil is not a refusal")
	}
}

func TestPaymentKeyIsStablePerItem(t *testing.T) {
	a := PaymentKey("b1", "k1")
	if a != PaymentKey("b1", "k1") {
		t.Fatal("the key must be the same on every call")
	}
	if a == PaymentKey("b1", "k2") || a == PaymentKey("b2", "k1") {
		t.Fatal("different items must get different keys")
	}
}
