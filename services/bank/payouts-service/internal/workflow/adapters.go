package workflow

import (
	"context"
	"errors"
	"fmt"
	"time"

	"connectrpc.com/connect"
	"go.temporal.io/api/enums/v1"
	"go.temporal.io/api/serviceerror"
	"go.temporal.io/sdk/client"

	paymentsv1 "github.com/khayaai/forge-mutual/pkg/api/forge/payments/v1"
	"github.com/khayaai/forge-mutual/pkg/api/forge/payments/v1/paymentsv1connect"
)

// ConnectPayments adapts the payments ConnectRPC client to the Payments interface.
type ConnectPayments struct {
	Client paymentsv1connect.PaymentsServiceClient
}

// CreatePayment implements Payments.
func (p ConnectPayments) CreatePayment(ctx context.Context, req *paymentsv1.CreatePaymentRequest) (string, error) {
	res, err := p.Client.CreatePayment(ctx, connect.NewRequest(req))
	if err != nil {
		return "", err
	}
	return res.Msg.PaymentId, nil
}

// GetPayment implements Payments.
func (p ConnectPayments) GetPayment(ctx context.Context, paymentID string) (paymentsv1.PaymentState, error) {
	res, err := p.Client.GetPayment(ctx, connect.NewRequest(&paymentsv1.GetPaymentRequest{PaymentId: paymentID}))
	if err != nil {
		return paymentsv1.PaymentState_PAYMENT_STATE_UNSPECIFIED, err
	}
	return res.Msg.Status.State, nil
}

// RunID is the workflow id of one run of a batch. A run number is never reused, so a new run never collides with an
// old one.
func RunID(batchID string, run uint32) string {
	return fmt.Sprintf("bulk-payout:%s:%d", batchID, run)
}

// Starter starts runs of BulkPayout on Temporal.
type Starter struct {
	Client       client.Client
	TaskQueue    string
	Concurrency  int
	PollInterval time.Duration
	MaxPolls     int
}

// StartRun starts a run. A run that is already started is left as it is, so starting again is safe.
func (s Starter) StartRun(ctx context.Context, batchID string, run uint32) error {
	_, err := s.Client.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
		ID:                                       RunID(batchID, run),
		TaskQueue:                                s.TaskQueue,
		WorkflowIDReusePolicy:                    enums.WORKFLOW_ID_REUSE_POLICY_REJECT_DUPLICATE,
		WorkflowExecutionErrorWhenAlreadyStarted: true,
	}, BulkPayout, BatchInput{
		BatchID: batchID, Run: run, Concurrency: s.Concurrency, PollInterval: s.PollInterval, MaxPolls: s.MaxPolls,
	})
	var started *serviceerror.WorkflowExecutionAlreadyStarted
	if errors.As(err, &started) {
		return nil
	}
	return err
}
