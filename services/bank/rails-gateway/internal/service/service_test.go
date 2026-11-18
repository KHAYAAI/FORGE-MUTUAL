package service

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"

	railsv1 "github.com/khayaai/forge-mutual/pkg/api/forge/rails/v1"
	"github.com/khayaai/forge-mutual/services/bank/rails-gateway/internal/rail"
)

func TestOverTheWireErrorCodes(t *testing.T) {
	s := New(rail.NewSim(false))
	ctx := context.Background()
	in := &railsv1.InitiateRequest{InstructionId: "10000000-0001", Rail: "EFT", AmountMinor: "100", Asset: "ZAR",
		BeneficiaryName: "A B", AccountNumber: "1234567", BankCode: "250655"}
	if _, err := s.Initiate(ctx, connect.NewRequest(in)); err != nil {
		t.Fatal(err)
	}
	changed := proto.Clone(in).(*railsv1.InitiateRequest)
	changed.AmountMinor = "101"
	if _, err := s.Initiate(ctx, connect.NewRequest(changed)); connect.CodeOf(err) != connect.CodeAlreadyExists {
		t.Fatalf("changed instruction: %v", err)
	}
	withScenario := proto.Clone(in).(*railsv1.InitiateRequest)
	withScenario.InstructionId = "10000000-0002"
	withScenario.Scenario = "stall"
	if _, err := s.Initiate(ctx, connect.NewRequest(withScenario)); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("scenario on a production rail: %v", err)
	}
	if _, err := s.Status(ctx, connect.NewRequest(&railsv1.StatusRequest{InstructionId: "missing"})); connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("unknown status: %v", err)
	}
}
