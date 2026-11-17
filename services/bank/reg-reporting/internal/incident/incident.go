// Package incident records material IT and cyber incidents and tracks what the regulators require of each one, under
// Joint Standard 1 of 2023, Joint Standard 2 of 2024 and Joint Notice 2 of 2026 (Annexure A template).
//
// What it does: holds the facts the template asks for, keeps the three deadlines, refuses a notification that is
// incomplete or that the preparer approved alone, and exports the template tab by tab.
//
// What it does not do: it does not submit anything. Banks and mutual banks submit on the Prudential Authority's Umoja
// portal, which this code does not call (VERIFY-UMOJA: no API is documented in the notice). A person submits the export
// and records the portal reference here.
//
// The three deadlines, from the template's cover page:
//   - Immediate notification: within 24 hours (clock hours, not business hours) of classifying the incident as material.
//   - Subsequent update: within 14 calendar days of the date of notification.
//   - Full incident report: on the timeline agreed with the responsible authority, so it is set by hand.
//
// Rounding rule (VERIFY with the PA): the 14 calendar days are counted as 14 x 24 hours from the moment of notification,
// which is never later than any reading that counts whole days.
package incident

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// Kind says which standard an incident falls under.
type Kind string

const (
	KindIT    Kind = "IT Incident"                             // Joint Standard 1 of 2023
	KindCyber Kind = "Cyber and Information Security incident" // Joint Standard 2 of 2024; also fill tab 4
)

// Deadlines.
const (
	ImmediateWindow = 24 * time.Hour
	UpdateWindow    = 14 * 24 * time.Hour
	// WarnBefore is how long before a deadline an obligation shows as due soon.
	WarnBefore = 6 * time.Hour
)

// State is where an incident is in its life.
type State string

const (
	StateDetected    State = "detected"     // seen, not yet judged
	StateNotMaterial State = "not_material" // judged not material; the judgement and its criteria are kept
	StateMaterial    State = "material"     // classified material: the 24-hour clock is running
	StateNotified    State = "notified"     // immediate notification sent: the 14-day clock is running
	StateUpdated     State = "updated"      // subsequent update sent
	StateReported    State = "reported"     // full report sent
)

// Person is a contact on tab 2.
type Person struct {
	Name, JobTitle, Email, Phone string
}

// Impact is tab 5 (subsequent update). Fields are free text unless noted. Yes/No fields take "Yes", "No" or
// "To be determined".
type Impact struct {
	UnauthorisedDataAccess, DataReleased, SensitiveCustomerDataLeaked string // Yes/No
	CustomerDataTypes                                                 string
	CustomerImpactAssessment                                          string
	CustomersImpacted                                                 string
	StakeholdersInformed                                              string // Yes/No
	HowInformed                                                       string
	ClaimsLodged                                                      string // Yes/No
	TimelyCustomerCommunication                                       string // Yes/No
	CustomerPrivacyRightsMeasures                                     string
	CustomerSupport                                                   string
	EnhancedCustomerService                                           string // Yes/No
	CustomerContactEase                                               string
	DataProtectionBefore                                              string
	ImmediateRemedialActions                                          string
	ServiceDisrupted                                                  string // Yes/No
	ServicesAffected                                                  string
	DisruptionDuration                                                string
	ThirdPartyIncident                                                string // Yes/No
	ThirdPartyName                                                    string
	LegalBreach                                                       string // Yes/No
	MediaCoverage                                                     string // Yes/No
	EventsDescription                                                 string
	EntryVector                                                       string
	Cause                                                             string
	IRPActivated, DRPActivated, BCPActivated                          string // Yes/No
	EscalatedToBoard                                                  string // Yes/No
	Resolved                                                          string // Yes/No
	ExpectedResolution                                                string
	RecoveryTime                                                      string
	LawEnforcementInvolved                                            string // Yes/No
	RecurrenceStrategy                                                string
	DirectLossRand, IndirectLossRand, CustomerLossRand                string
	LossRecovered                                                     string // see LossRecoveryOptions
	Conclusion, LessonsLearnt                                         string
}

// Cyber is tab 4. It is required when Kind is KindCyber.
type Cyber struct {
	DiscoveredBy             string
	HowDiscovered            string
	OriginOfThreat           string
	CauseOfIncident          string
	CategoryOfAttacker       string
	TypeOfAttack             string
	ThirdPartyCompromise     string // Yes/No
	VulnerabilitiesExploited string
}

// Incident is one record. Zero values mean "not yet known".
type Incident struct {
	ID          string
	Institution string
	Kind        Kind
	State       State

	// Tab 2.
	Contact1, Contact2 Person

	// Tab 3.
	DetectedAt             time.Time
	Description            string
	Coverage               string
	PreliminaryCategory    string // one of CategoryOptions
	BusinessLines          string
	AffectedSystem         string
	SystemType             string // one of AffectedAreaOptions
	ActionsTaken           string
	MaterialityCriteria    string
	OtherAuthorityNotified string // Yes/No
	OtherAuthorityDetail   string

	Cyber  Cyber
	Impact Impact

	// Classification.
	ClassifiedAt time.Time
	ClassifiedBy string

	// Immediate notification.
	NotifiedAt       time.Time
	NotifiedRef      string // the Umoja portal reference
	NotifyPreparedBy string
	NotifyApprovedBy string

	// Subsequent update.
	UpdatedAt        time.Time
	UpdatedRef       string
	UpdatePreparedBy string
	UpdateApprovedBy string

	// Full report: due date agreed with the authority, then sent.
	FullReportDue    time.Time
	FullReportSentAt time.Time
	FullReportRef    string

	History []Event
}

// Event is one line of the incident's own history.
type Event struct {
	At     time.Time
	By     string
	Action string
	Detail string
}

// Errors.
var (
	ErrState        = errors.New("incident: not allowed in this state")
	ErrIncomplete   = errors.New("incident: required fields are missing")
	ErrSameApprover = errors.New("incident: the preparer cannot approve their own submission")
)

// Missing lists the fields a submission still lacks. It is the error value for ErrIncomplete.
type Missing []string

func (m Missing) Error() string {
	return "incident: required fields are missing: " + strings.Join(m, "; ")
}
func (m Missing) Unwrap() error { return ErrIncomplete }

// Detect starts a record.
func Detect(id, institution string, kind Kind, detectedAt time.Time, by string) (*Incident, error) {
	if id == "" || institution == "" || by == "" || detectedAt.IsZero() {
		return nil, errors.New("incident: id, institution, detection time and recorder are required")
	}
	if kind != KindIT && kind != KindCyber {
		return nil, fmt.Errorf("incident: unknown kind %q", kind)
	}
	in := &Incident{ID: id, Institution: institution, Kind: kind, State: StateDetected, DetectedAt: detectedAt.UTC()}
	in.log(detectedAt, by, "detected", string(kind))
	return in, nil
}

func (in *Incident) log(at time.Time, by, action, detail string) {
	in.History = append(in.History, Event{At: at.UTC(), By: by, Action: action, Detail: detail})
}

// Classify records the institution's own judgement of materiality. The authorities leave the threshold to the
// institution, so the criteria are required either way: the template asks for them, and a later reviewer will too.
// Classifying as material starts the 24-hour clock at the time given.
func (in *Incident) Classify(material bool, criteria, by string, at time.Time) error {
	if in.State != StateDetected && in.State != StateNotMaterial {
		return fmt.Errorf("%w: %s", ErrState, in.State)
	}
	if strings.TrimSpace(criteria) == "" || by == "" {
		return errors.New("incident: the criteria used and the person classifying are required")
	}
	in.MaterialityCriteria = criteria
	in.ClassifiedBy = by
	if !material {
		in.State = StateNotMaterial
		in.log(at, by, "classified_not_material", criteria)
		return nil
	}
	in.State = StateMaterial
	in.ClassifiedAt = at.UTC()
	in.log(at, by, "classified_material", criteria)
	return nil
}

// ImmediateDue is when the immediate notification must be with the authority. Zero until the incident is material.
func (in *Incident) ImmediateDue() time.Time {
	if in.ClassifiedAt.IsZero() {
		return time.Time{}
	}
	return in.ClassifiedAt.Add(ImmediateWindow)
}

// UpdateDue is when the subsequent update must be with the authority. Zero until the immediate notification is sent.
func (in *Incident) UpdateDue() time.Time {
	if in.NotifiedAt.IsZero() {
		return time.Time{}
	}
	return in.NotifiedAt.Add(UpdateWindow)
}

// ImmediateMissing lists what tabs 2 and 3 (and tab 4 for a cyber incident) still lack for the immediate notification.
func (in *Incident) ImmediateMissing() Missing {
	var m Missing
	req := func(label, v string) {
		if strings.TrimSpace(v) == "" {
			m = append(m, label)
		}
	}
	req("Institution name", in.Institution)
	req("Person 1 name", in.Contact1.Name)
	req("Person 1 job title", in.Contact1.JobTitle)
	req("Person 1 e-mail", in.Contact1.Email)
	req("Person 1 phone", in.Contact1.Phone)
	if in.DetectedAt.IsZero() {
		m = append(m, "Incident detection date and time")
	}
	req("Detailed description", in.Description)
	req("Incident coverage", in.Coverage)
	req("Preliminary incident classification", in.PreliminaryCategory)
	req("Business lines affected", in.BusinessLines)
	req("Affected system", in.AffectedSystem)
	req("Type of system", in.SystemType)
	req("Actions taken", in.ActionsTaken)
	req("Materiality criteria", in.MaterialityCriteria)
	req("Other authority notified", in.OtherAuthorityNotified)
	if in.PreliminaryCategory != "" && !oneOf(in.PreliminaryCategory, CategoryOptions) && !strings.HasPrefix(in.PreliminaryCategory, "Other") {
		m = append(m, "Preliminary incident classification must be one of the template's options")
	}
	if in.OtherAuthorityNotified != "" && !oneOf(in.OtherAuthorityNotified, YesNo) {
		m = append(m, "Other authority notified must be Yes, No or To be determined")
	}
	if in.Kind == KindCyber {
		c := in.Cyber
		req("Cyber: discovered by", c.DiscoveredBy)
		req("Cyber: how discovered", c.HowDiscovered)
		req("Cyber: origin of the threat", c.OriginOfThreat)
		req("Cyber: cause of incident", c.CauseOfIncident)
		req("Cyber: category of attackers", c.CategoryOfAttacker)
		req("Cyber: type of attack", c.TypeOfAttack)
		req("Cyber: third-party compromise", c.ThirdPartyCompromise)
		req("Cyber: vulnerabilities exploited", c.VulnerabilitiesExploited)
	}
	return m
}

// Notify records that the immediate notification was submitted. The preparer and approver must differ, the template
// must be complete, and the incident must be classified material. The time given is when it reached the portal.
func (in *Incident) Notify(portalRef, preparedBy, approvedBy string, at time.Time) error {
	if in.State != StateMaterial {
		return fmt.Errorf("%w: %s", ErrState, in.State)
	}
	if portalRef == "" || preparedBy == "" || approvedBy == "" {
		return errors.New("incident: portal reference, preparer and approver are required")
	}
	if preparedBy == approvedBy {
		return ErrSameApprover
	}
	if m := in.ImmediateMissing(); len(m) > 0 {
		return m
	}
	in.State = StateNotified
	in.NotifiedAt = at.UTC()
	in.NotifiedRef, in.NotifyPreparedBy, in.NotifyApprovedBy = portalRef, preparedBy, approvedBy
	late := ""
	if at.After(in.ImmediateDue()) {
		late = fmt.Sprintf(" LATE by %s", at.Sub(in.ImmediateDue()).Round(time.Minute))
	}
	in.log(at, approvedBy, "notified", portalRef+late)
	return nil
}

// UpdateMissing lists what tab 5 still lacks for the subsequent update.
func (in *Incident) UpdateMissing() Missing {
	var m Missing
	i := in.Impact
	for _, f := range []struct{ label, v string }{
		{"Unauthorised access of data", i.UnauthorisedDataAccess},
		{"Release of data", i.DataReleased},
		{"Sensitive customer data leaked", i.SensitiveCustomerDataLeaked},
		{"Customers impacted", i.CustomersImpacted},
		{"Stakeholders informed", i.StakeholdersInformed},
		{"Claims lodged", i.ClaimsLodged},
		{"Immediate remedial actions", i.ImmediateRemedialActions},
		{"Service disrupted", i.ServiceDisrupted},
		{"Third-party incident", i.ThirdPartyIncident},
		{"Breach of legal or regulatory requirements", i.LegalBreach},
		{"Media coverage", i.MediaCoverage},
		{"Description of events", i.EventsDescription},
		{"Entry vector", i.EntryVector},
		{"Cause", i.Cause},
		{"IRP activated", i.IRPActivated},
		{"DRP activated", i.DRPActivated},
		{"BCP activated", i.BCPActivated},
		{"Escalated to the board", i.EscalatedToBoard},
		{"Resolved", i.Resolved},
		{"Recurrence strategy", i.RecurrenceStrategy},
	} {
		if strings.TrimSpace(f.v) == "" {
			m = append(m, f.label)
		}
	}
	return m
}

// SendUpdate records the subsequent update (tab 5). Unlike the immediate notification, an update may say "To be
// determined" for a Yes/No field. Same rules on preparer and approver.
func (in *Incident) SendUpdate(portalRef, preparedBy, approvedBy string, at time.Time) error {
	if in.State != StateNotified {
		return fmt.Errorf("%w: %s", ErrState, in.State)
	}
	if portalRef == "" || preparedBy == "" || approvedBy == "" {
		return errors.New("incident: portal reference, preparer and approver are required")
	}
	if preparedBy == approvedBy {
		return ErrSameApprover
	}
	if m := in.UpdateMissing(); len(m) > 0 {
		return m
	}
	in.State = StateUpdated
	in.UpdatedAt = at.UTC()
	in.UpdatedRef, in.UpdatePreparedBy, in.UpdateApprovedBy = portalRef, preparedBy, approvedBy
	late := ""
	if at.After(in.UpdateDue()) {
		late = fmt.Sprintf(" LATE by %s", at.Sub(in.UpdateDue()).Round(time.Minute))
	}
	in.log(at, approvedBy, "update_sent", portalRef+late)
	return nil
}

// AgreeFullReport records the date the authority agreed for the full incident report.
func (in *Incident) AgreeFullReport(due time.Time, by string, at time.Time) error {
	if in.State != StateNotified && in.State != StateUpdated {
		return fmt.Errorf("%w: %s", ErrState, in.State)
	}
	if due.IsZero() || by == "" {
		return errors.New("incident: the agreed date and who agreed it are required")
	}
	in.FullReportDue = due.UTC()
	in.log(at, by, "full_report_agreed", due.UTC().Format(time.RFC3339))
	return nil
}

// SendFullReport records the full report (all tabs and the final investigation report).
func (in *Incident) SendFullReport(portalRef, by string, at time.Time) error {
	if in.State != StateUpdated {
		return fmt.Errorf("%w: %s", ErrState, in.State)
	}
	if portalRef == "" || by == "" {
		return errors.New("incident: portal reference and sender are required")
	}
	in.State = StateReported
	in.FullReportSentAt = at.UTC()
	in.FullReportRef = portalRef
	in.log(at, by, "full_report_sent", portalRef)
	return nil
}

// Obligation is one thing owed to the authority.
type Obligation struct {
	Name   string // "immediate", "update" or "full_report"
	Due    time.Time
	Status string // "not_started", "pending", "due_soon", "overdue", "done", "done_late"
}

// Obligations returns what is owed and how it stands at now. An obligation not yet triggered has a zero Due and the
// status "not_started".
func (in *Incident) Obligations(now time.Time) []Obligation {
	return []Obligation{
		obligation("immediate", in.ImmediateDue(), in.NotifiedAt, now),
		obligation("update", in.UpdateDue(), in.UpdatedAt, now),
		obligation("full_report", in.FullReportDue, in.FullReportSentAt, now),
	}
}

func obligation(name string, due, done, now time.Time) Obligation {
	o := Obligation{Name: name, Due: due}
	switch {
	case due.IsZero() && done.IsZero():
		o.Status = "not_started"
	case !done.IsZero() && (due.IsZero() || !done.After(due)):
		o.Status = "done"
	case !done.IsZero():
		o.Status = "done_late"
	case now.After(due):
		o.Status = "overdue"
	case due.Sub(now) <= WarnBefore:
		o.Status = "due_soon"
	default:
		o.Status = "pending"
	}
	return o
}

func oneOf(v string, opts []string) bool {
	for _, o := range opts {
		if v == o {
			return true
		}
	}
	return false
}
