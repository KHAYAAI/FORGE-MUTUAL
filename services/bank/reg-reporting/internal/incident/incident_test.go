package incident

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"
)

var t0 = time.Date(2026, 11, 17, 9, 0, 0, 0, time.UTC)

func ready(t *testing.T, kind Kind) *Incident {
	t.Helper()
	in, err := Detect("INC-1", "Forge", kind, t0, "ops")
	if err != nil {
		t.Fatal(err)
	}
	in.Contact1 = Person{"A Person", "Head of Security", "a@example.co.za", "0110000000"}
	in.Description, in.Coverage = "ledger gateway unreachable", "all customers"
	in.PreliminaryCategory = CategoryOptions[0]
	in.BusinessLines, in.AffectedSystem, in.SystemType = "payments", "ledger-gateway-bank", "Core banking systems"
	in.ActionsTaken, in.OtherAuthorityNotified = "failover started", "No"
	if kind == KindCyber {
		in.Cyber = Cyber{"IT security team", "alert", "External", "Malware", "Organised criminal group", "Ransomware", "No", "Insufficient patch management"}
	}
	return in
}

func TestDeadlinesFollowClassificationAndNotification(t *testing.T) {
	in := ready(t, KindIT)
	if !in.ImmediateDue().IsZero() {
		t.Fatal("no deadline before the incident is classified")
	}
	cls := t0.Add(2 * time.Hour)
	if err := in.Classify(true, "customers cannot pay for over an hour", "ciso", cls); err != nil {
		t.Fatal(err)
	}
	if got, want := in.ImmediateDue(), cls.Add(24*time.Hour); !got.Equal(want) {
		t.Fatalf("immediate due %v, want %v", got, want)
	}
	if !in.UpdateDue().IsZero() {
		t.Fatal("no update deadline before notification")
	}
	nt := cls.Add(5 * time.Hour)
	if err := in.Notify("UMOJA-1", "analyst", "ciso", nt); err != nil {
		t.Fatal(err)
	}
	if got, want := in.UpdateDue(), nt.Add(14*24*time.Hour); !got.Equal(want) {
		t.Fatalf("update due %v, want %v", got, want)
	}
}

func TestClockHoursNotBusinessHours(t *testing.T) {
	in := ready(t, KindIT)
	friday := time.Date(2026, 11, 20, 17, 0, 0, 0, time.UTC) // a Friday evening
	_ = in.Classify(true, "criteria", "ciso", friday)
	if got, want := in.ImmediateDue(), time.Date(2026, 11, 21, 17, 0, 0, 0, time.UTC); !got.Equal(want) {
		t.Fatalf("due %v: the 24 hours run through the weekend, want %v", got, want)
	}
}

func TestClassifyNeedsCriteriaAndIsOnlyForNewIncidents(t *testing.T) {
	in := ready(t, KindIT)
	if err := in.Classify(true, " ", "ciso", t0); err == nil {
		t.Fatal("criteria are required")
	}
	if err := in.Classify(false, "limited, no customer effect", "ciso", t0); err != nil || in.State != StateNotMaterial {
		t.Fatalf("state %v err %v", in.State, err)
	}
	if !in.ImmediateDue().IsZero() {
		t.Fatal("a not-material incident has no clock")
	}
	// It can be reclassified if more is learned.
	if err := in.Classify(true, "now affects customers", "ciso", t0.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := in.Classify(false, "x", "ciso", t0); !errors.Is(err, ErrState) {
		t.Fatalf("a material incident cannot be declassified: %v", err)
	}
}

func TestNotifyRefusedWhenIncompleteOrSelfApproved(t *testing.T) {
	in := ready(t, KindCyber)
	in.Cyber.TypeOfAttack = ""
	_ = in.Classify(true, "c", "ciso", t0)
	err := in.Notify("R", "analyst", "ciso", t0.Add(time.Hour))
	var m Missing
	if !errors.As(err, &m) || !errors.Is(err, ErrIncomplete) || len(m) != 1 || !strings.Contains(m[0], "type of attack") {
		t.Fatalf("want one missing field, got %v", err)
	}
	in.Cyber.TypeOfAttack = "Ransomware"
	if err := in.Notify("R", "ciso", "ciso", t0.Add(time.Hour)); !errors.Is(err, ErrSameApprover) {
		t.Fatalf("self approval must be refused: %v", err)
	}
	if err := in.Notify("R", "analyst", "ciso", t0.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
}

func TestItIncidentDoesNotNeedCyberTab(t *testing.T) {
	in := ready(t, KindIT)
	_ = in.Classify(true, "criteria", "ciso", t0)
	if m := in.ImmediateMissing(); len(m) != 0 {
		t.Fatalf("complete IT incident reported missing %v", m)
	}
	for _, tab := range in.Tabs(t0) {
		if tab.Number == 4 {
			t.Fatal("tab 4 belongs to cyber incidents only")
		}
	}
	cy := ready(t, KindCyber)
	has4 := false
	for _, tab := range cy.Tabs(t0) {
		has4 = has4 || tab.Number == 4
	}
	if !has4 {
		t.Fatal("a cyber incident must carry tab 4")
	}
}

func TestOptionValuesAreChecked(t *testing.T) {
	in := ready(t, KindIT)
	_ = in.Classify(true, "criteria", "ciso", t0)
	in.PreliminaryCategory = "severe"
	in.OtherAuthorityNotified = "maybe"
	m := in.ImmediateMissing()
	if len(m) != 2 {
		t.Fatalf("want two option errors, got %v", m)
	}
	in.PreliminaryCategory = "Other (Please specify)"
	if len(in.ImmediateMissing()) != 1 {
		t.Fatal("Other is allowed as in the template")
	}
}

func fillImpact(in *Incident) {
	in.Impact = Impact{
		UnauthorisedDataAccess: "No", DataReleased: "No", SensitiveCustomerDataLeaked: "No", CustomersImpacted: "120",
		StakeholdersInformed: "Yes", ClaimsLodged: "No", ImmediateRemedialActions: "restarted", ServiceDisrupted: "Yes",
		ThirdPartyIncident: "No", LegalBreach: "To be determined", MediaCoverage: "No", EventsDescription: "events",
		EntryVector: "n/a", Cause: "disk full", IRPActivated: "Yes", DRPActivated: "No", BCPActivated: "No",
		EscalatedToBoard: "Yes", Resolved: "Yes", RecurrenceStrategy: "alerts",
	}
}

func TestObligationsBeforeAnyClock(t *testing.T) {
	in := ready(t, KindIT)
	for _, o := range in.Obligations(t0) {
		if o.Status != "not_started" || !o.Due.IsZero() {
			t.Fatalf("%s should not have started: %+v", o.Name, o)
		}
	}
}

func TestWarnAndOverdueAndLate(t *testing.T) {
	in := ready(t, KindIT)
	_ = in.Classify(true, "criteria", "ciso", t0) // due t0+24h
	if got := in.Obligations(t0.Add(17 * time.Hour))[0].Status; got != "pending" {
		t.Fatalf("7h before due is pending, got %s", got)
	}
	if got := in.Obligations(t0.Add(18 * time.Hour))[0].Status; got != "due_soon" {
		t.Fatalf("6h before due is due_soon, got %s", got)
	}
	if got := in.Obligations(t0.Add(24*time.Hour + time.Second))[0].Status; got != "overdue" {
		t.Fatalf("after due is overdue, got %s", got)
	}
	if got := in.Obligations(t0.Add(24 * time.Hour))[0].Status; got != "due_soon" {
		t.Fatalf("exactly at the deadline is not yet overdue, got %s", got)
	}
	// Notified late: recorded as done_late and the history says so.
	late := t0.Add(30 * time.Hour)
	if err := in.Notify("R", "analyst", "ciso", late); err != nil {
		t.Fatal(err)
	}
	if got := in.Obligations(late)[0].Status; got != "done_late" {
		t.Fatalf("got %s", got)
	}
	if h := in.History[len(in.History)-1]; !strings.Contains(h.Detail, "LATE by 6h0m0s") {
		t.Fatalf("history should record lateness: %q", h.Detail)
	}
}

func TestUpdateAndFullReportSequence(t *testing.T) {
	in := ready(t, KindIT)
	_ = in.Classify(true, "criteria", "ciso", t0)
	_ = in.Notify("R1", "analyst", "ciso", t0.Add(3*time.Hour))
	if err := in.SendFullReport("R3", "ciso", t0); !errors.Is(err, ErrState) {
		t.Fatal("full report only after the update")
	}
	if err := in.SendUpdate("R2", "analyst", "ciso", t0.Add(48*time.Hour)); !errors.Is(err, ErrIncomplete) {
		t.Fatalf("tab 5 incomplete must be refused: %v", err)
	}
	fillImpact(in)
	if err := in.SendUpdate("R2", "analyst", "analyst", t0.Add(48*time.Hour)); !errors.Is(err, ErrSameApprover) {
		t.Fatal("self approval refused")
	}
	if err := in.SendUpdate("R2", "analyst", "ciso", t0.Add(48*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := in.AgreeFullReport(t0.Add(30*24*time.Hour), "ciso", t0.Add(49*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if got := in.Obligations(t0.Add(50 * time.Hour))[2].Status; got != "pending" {
		t.Fatalf("full report pending, got %s", got)
	}
	if err := in.SendFullReport("R3", "ciso", t0.Add(20*24*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if in.State != StateReported {
		t.Fatal(in.State)
	}
	if n := len(in.History); n != 6 {
		t.Fatalf("history has %d events", n)
	}
}

func TestExportCSV(t *testing.T) {
	in := ready(t, KindCyber)
	var b bytes.Buffer
	if err := WriteCSV(&b, in.ImmediateTabs(t0)); err != nil {
		t.Fatal(err)
	}
	out := b.String()
	for _, want := range []string{"2. CONTACT DETAILS", "3. DETAILS OF THE INCIDENT", "4. CYBER INCIDENT INFO", "17/11/2026 09:00", "Ransomware", "Core banking systems"} {
		if !strings.Contains(out, want) {
			t.Errorf("export lacks %q", want)
		}
	}
	if strings.Contains(out, "5. IMPACT") {
		t.Error("immediate export must not include tab 5")
	}
	if !strings.HasPrefix(out, "tab,label,value\n") {
		t.Errorf("header: %q", strings.SplitN(out, "\n", 2)[0])
	}
}

func TestDetectValidates(t *testing.T) {
	if _, err := Detect("", "F", KindIT, t0, "x"); err == nil {
		t.Fatal("id required")
	}
	if _, err := Detect("1", "F", "weird", t0, "x"); err == nil {
		t.Fatal("kind checked")
	}
}
