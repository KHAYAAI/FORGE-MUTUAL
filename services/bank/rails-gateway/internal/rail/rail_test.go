package rail

import (
	"errors"
	"testing"

	railsv1 "github.com/khayaai/forge-mutual/pkg/api/forge/rails/v1"
)

func instr(id, scenario string) Instruction {
	return Instruction{ID: id, Rail: "EFT", AmountMinor: "15000", Asset: "ZAR", BeneficiaryName: "Sipho Ndlovu",
		AccountNumber: "1234567890", BankCode: "250655", Scenario: scenario}
}

func TestDefaultScenarioSettlesOnTheSecondPoll(t *testing.T) {
	s := NewSim(false)
	r, err := s.Initiate(instr("aaaaaaaa-0001", ""))
	if err != nil || r.Status != railsv1.RailStatus_RAIL_STATUS_ACCEPTED {
		t.Fatalf("initiate = %+v, %v", r, err)
	}
	if r, _ := s.Status("aaaaaaaa-0001"); r.Status != railsv1.RailStatus_RAIL_STATUS_PENDING {
		t.Fatalf("first poll = %v, want PENDING", r.Status)
	}
	if r, _ := s.Status("aaaaaaaa-0001"); r.Status != railsv1.RailStatus_RAIL_STATUS_SETTLED {
		t.Fatalf("second poll = %v, want SETTLED", r.Status)
	}
	if r, _ := s.Status("aaaaaaaa-0001"); r.Status != railsv1.RailStatus_RAIL_STATUS_SETTLED {
		t.Fatal("a final outcome must not change")
	}
}

func TestScenariosAreRefusedUnlessEnabled(t *testing.T) {
	if _, err := NewSim(false).Initiate(instr("bbbbbbbb-0001", "stall")); !errors.Is(err, ErrScenarioRefused) {
		t.Fatalf("scenario on a production rail must be refused: %v", err)
	}
}

func TestScenarioGrammar(t *testing.T) {
	good := []string{"", "settle_after:0", "reject_after:3", "return_after:1", "stall", "reject_at_initiation"}
	for _, g := range good {
		if _, err := ParseScenario(g); err != nil {
			t.Errorf("%q: %v", g, err)
		}
	}
	for _, bad := range []string{"settle", "settle_after:-1", "settle_after:11", "explode:2", "stall:1"} {
		if _, err := ParseScenario(bad); !errors.Is(err, ErrBadScenario) {
			t.Errorf("%q: want ErrBadScenario, got %v", bad, err)
		}
	}
}

func TestRejectAfterAndReturnAfter(t *testing.T) {
	s := NewSim(true)
	_, _ = s.Initiate(instr("cccccccc-0001", "reject_after:1"))
	s.Status("cccccccc-0001")
	if r, _ := s.Status("cccccccc-0001"); r.Status != railsv1.RailStatus_RAIL_STATUS_REJECTED {
		t.Fatalf("reject_after:1 = %v", r.Status)
	}
	_, _ = s.Initiate(instr("cccccccc-0002", "return_after:0"))
	if r, _ := s.Status("cccccccc-0002"); r.Status != railsv1.RailStatus_RAIL_STATUS_RETURNED {
		t.Fatalf("return_after:0 = %v", r.Status)
	}
}

func TestStallNeverReachesAFinalOutcome(t *testing.T) {
	s := NewSim(true)
	_, _ = s.Initiate(instr("dddddddd-0001", "stall"))
	for i := 0; i < 5; i++ {
		if r, _ := s.Status("dddddddd-0001"); r.Status != railsv1.RailStatus_RAIL_STATUS_PENDING {
			t.Fatalf("poll %d = %v, want PENDING", i, r.Status)
		}
	}
}

func TestInitiateIsIdempotentAndRefusesChangedInstructions(t *testing.T) {
	s := NewSim(true)
	first, _ := s.Initiate(instr("eeeeeeee-0001", "settle_after:0"))
	again, err := s.Initiate(instr("eeeeeeee-0001", "settle_after:0"))
	if err != nil || again.Reference != first.Reference {
		t.Fatalf("replay = %+v, %v", again, err)
	}
	changed := instr("eeeeeeee-0001", "settle_after:0")
	changed.AmountMinor = "15001"
	if _, err := s.Initiate(changed); !errors.Is(err, ErrConflict) {
		t.Fatalf("changed amount under the same id: want ErrConflict, got %v", err)
	}
}

func TestInvalidInstructionsAreRejectedByTheRail(t *testing.T) {
	cases := map[string]func(*Instruction){
		"dollars":      func(i *Instruction) { i.Asset = "USD" },
		"bad account":  func(i *Instruction) { i.AccountNumber = "12ab" },
		"zero amount":  func(i *Instruction) { i.AmountMinor = "0" },
		"decimal":      func(i *Instruction) { i.AmountMinor = "150.00" },
		"no name":      func(i *Instruction) { i.BeneficiaryName = " " },
		"unknown rail": func(i *Instruction) { i.Rail = "RTC" },
	}
	for name, mut := range cases {
		s := NewSim(true)
		in := instr("ffffffff-"+name[:4], "")
		mut(&in)
		r, err := s.Initiate(in)
		if err != nil || r.Status != railsv1.RailStatus_RAIL_STATUS_REJECTED {
			t.Errorf("%s: want rail rejection, got %+v, %v", name, r, err)
		}
	}
}

func TestRejectAtInitiation(t *testing.T) {
	s := NewSim(true)
	r, err := s.Initiate(instr("99999999-0001", "reject_at_initiation"))
	if err != nil || r.Status != railsv1.RailStatus_RAIL_STATUS_REJECTED {
		t.Fatalf("reject_at_initiation = %+v, %v", r, err)
	}
	if _, err := s.Status("99999999-0001"); err != nil {
		t.Fatal("a rejected instruction is still known to the rail")
	}
}

func TestUnknownInstructionIsNotFound(t *testing.T) {
	if _, err := NewSim(true).Status("nope"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown status: %v", err)
	}
}
