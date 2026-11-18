package service

import (
	"context"
	"testing"

	"connectrpc.com/connect"

	onboardingv1 "github.com/khayaai/forge-mutual/pkg/api/forge/onboarding/v1"
)

func personRequest(subject, nationalID string) *connect.Request[onboardingv1.StartPersonOnboardingRequest] {
	return connect.NewRequest(&onboardingv1.StartPersonOnboardingRequest{
		LegalName: "Naledi Mokoena", NationalId: nationalID, Country: "ZA", Actor: "applicant", ApplicantSubject: subject,
	})
}

const validID = "9001015009086" // a valid South African identity number (date and check digit)

// An applicant has one application of each kind. Applying again with the same details is the same application, and
// the applicant finds it by who they are, without keeping an ID.
func TestAnApplicantHasOneApplicationOfEachKindAndFindsIt(t *testing.T) {
	tc := newFakeTemporal()
	s := New(tc, mustFake(t), "onboarding")
	ctx := context.Background()

	first, err := s.StartPersonOnboarding(ctx, personRequest("user_01", validID))
	if err != nil {
		t.Fatal(err)
	}
	again, err := s.StartPersonOnboarding(ctx, personRequest("user_01", validID))
	if err != nil || again.Msg.OnboardingId != first.Msg.OnboardingId {
		t.Fatalf("applying again: %v %v", again, err)
	}
	if _, err := s.StartPersonOnboarding(ctx, personRequest("user_01", "8001015009087")); connect.CodeOf(err) != connect.CodeAlreadyExists {
		t.Fatalf("a different application from the same applicant: %v, want AlreadyExists", err)
	}

	found, err := s.FindApplication(ctx, connect.NewRequest(&onboardingv1.FindApplicationRequest{ApplicantSubject: "user_01", Kind: "person"}))
	if err != nil || found.Msg.Status.OnboardingId != first.Msg.OnboardingId || found.Msg.Status.Kind != "person" {
		t.Fatalf("find: %v %v", found, err)
	}
	// Another applicant does not find it, and the same applicant has no business application.
	if _, err := s.FindApplication(ctx, connect.NewRequest(&onboardingv1.FindApplicationRequest{ApplicantSubject: "user_02", Kind: "person"})); connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("another applicant: %v", err)
	}
	if _, err := s.FindApplication(ctx, connect.NewRequest(&onboardingv1.FindApplicationRequest{ApplicantSubject: "user_01", Kind: "business"})); connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("no business application: %v", err)
	}
	// A caller's own idempotency key cannot reach an applicant's application.
	other, err := s.StartPersonOnboarding(ctx, connect.NewRequest(&onboardingv1.StartPersonOnboardingRequest{
		IdempotencyKey: "user_01", LegalName: "X", NationalId: validID, Country: "ZA", Actor: "a",
	}))
	if err != nil || other.Msg.OnboardingId == first.Msg.OnboardingId {
		t.Fatalf("a plain key collided with an applicant's application: %v %v", other, err)
	}
}

func TestAPersonApplicationIsValidated(t *testing.T) {
	s := New(newFakeTemporal(), mustFake(t), "onboarding")
	ctx := context.Background()
	for name, req := range map[string]*connect.Request[onboardingv1.StartPersonOnboardingRequest]{
		"bad check digit":     personRequest("user_01", "9001015009087"),
		"impossible date":     personRequest("user_01", "9013325009086"),
		"too short":           personRequest("user_01", "900101500908"),
		"subject with spaces": personRequest("user 01", validID),
		"no name":             connect.NewRequest(&onboardingv1.StartPersonOnboardingRequest{NationalId: validID, Country: "ZA", Actor: "a", ApplicantSubject: "u"}),
	} {
		if _, err := s.StartPersonOnboarding(ctx, req); connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("%s: %v, want InvalidArgument", name, err)
		}
	}
	if _, err := s.FindApplication(ctx, connect.NewRequest(&onboardingv1.FindApplicationRequest{ApplicantSubject: "u", Kind: "trust"})); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("unknown kind: %v", err)
	}
}

func TestABusinessApplicationIsKeyedToItsApplicant(t *testing.T) {
	tc := newFakeTemporal()
	s := New(tc, mustFake(t), "onboarding")
	req := request("ignored-key", func(r *onboardingv1.StartBusinessOnboardingRequest) { r.ApplicantSubject = "user_07" })
	res, err := s.StartBusinessOnboarding(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	found, err := s.FindApplication(context.Background(), connect.NewRequest(&onboardingv1.FindApplicationRequest{ApplicantSubject: "user_07", Kind: "business"}))
	if err != nil || found.Msg.Status.OnboardingId != res.Msg.OnboardingId || found.Msg.Status.Kind != "business" {
		t.Fatalf("find: %v %v", found, err)
	}
}
