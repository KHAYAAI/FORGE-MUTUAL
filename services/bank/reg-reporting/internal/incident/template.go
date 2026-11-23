package incident

import (
	"encoding/csv"
	"fmt"
	"io"
	"time"
)

// Option lists copied from the "Options" sheet of Annexure A (Reporting of Material IT and Cyber Incident Template).
// Only lists whose purpose is unambiguous in the sheet are kept as lists. The sheet's other drop-downs are not
// reproduced here, because their headings do not say which field they feed (VERIFY against the portal).
var (
	YesNo = []string{"Yes", "No", "To be determined"}
	// CategoryOptions feeds "Preliminary incident classification" (tab 3).
	CategoryOptions = []string{
		"Affected critical system",
		"Incident occurred on system or network that could put another critical system at risk",
		"Incident occurred on non-critical system",
	}
	// AffectedAreaOptions feeds "Type of system" (tab 3).
	AffectedAreaOptions = []string{
		"Core banking systems", "ATM", "Payment systems", "Financial reporting system", "Policy administration system",
		"Contact center system", "Claims system", "Settlement system", "Trading system/platform",
		"Datacentre/Recovery system",
	}
	LossRecoveryOptions = []string{"Yes - full recovery", "Yes - partial recovery", "No", "Not available"}
	LossKindOptions     = []string{"Direct", "Indirect", "Both"}
)

// Field is one labelled line of a template tab.
type Field struct{ Label, Value string }

// Tab is one tab of the template.
type Tab struct {
	Number int
	Name   string
	Fields []Field
}

func ts(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("02/01/2006 15:04")
}

func dt(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("02/01/2006")
}

// Tabs returns the template tabs for this incident, in the template's order and with the template's labels. Tab 4 is
// included only for a cyber incident. Date and time use the template's DD/MM/CCCC and DD/MM/CCCC HH:MM formats. Times
// are written in UTC: convert if the portal expects local time (VERIFY).
func (in *Incident) Tabs(at time.Time) []Tab {
	i, c := in.Impact, in.Cyber
	tabs := []Tab{
		{2, "CONTACT DETAILS", []Field{
			{"Name of the financial institution", in.Institution},
			{"Date of notification", dt(at)},
			{"Person 1: Name", in.Contact1.Name}, {"Person 1: Job title", in.Contact1.JobTitle},
			{"Person 1: E-mail", in.Contact1.Email}, {"Person 1: Phone", in.Contact1.Phone},
			{"Person 2: Name", in.Contact2.Name}, {"Person 2: Job title", in.Contact2.JobTitle},
			{"Person 2: E-mail", in.Contact2.Email}, {"Person 2: Phone", in.Contact2.Phone},
		}},
		{3, "DETAILS OF THE INCIDENT", []Field{
			{"Incident detection date and time", ts(in.DetectedAt)},
			{"Detailed description", in.Description},
			{"Incident coverage", in.Coverage},
			{"Preliminary incident classification", in.PreliminaryCategory},
			{"Business lines affected", in.BusinessLines},
			{"Affected system", in.AffectedSystem},
			{"Type of system", in.SystemType},
			{"What high-level actions or responses have been taken by the financial institution?", in.ActionsTaken},
			{"Is this an IT or cyber incident", string(in.Kind)},
			{"What criteria was utilised to assess the materiality of the incident?", in.MaterialityCriteria},
			{"Has any other regulatory authority been notified of this incident?", in.OtherAuthorityNotified},
			{"Other authority: more details", in.OtherAuthorityDetail},
		}},
	}
	if in.Kind == KindCyber {
		tabs = append(tabs, Tab{4, "CYBER INCIDENT INFO", []Field{
			{"Incident discovered by", c.DiscoveredBy}, {"How the incident was discovered?", c.HowDiscovered},
			{"Origin of the threat", c.OriginOfThreat}, {"Cause of incident", c.CauseOfIncident},
			{"Category of attackers", c.CategoryOfAttacker}, {"Type of attack", c.TypeOfAttack},
			{"Was this a third-party compromise?", c.ThirdPartyCompromise},
			{"What vulnerabilities / weaknesses were exploited?", c.VulnerabilitiesExploited},
		}})
	}
	tabs = append(tabs, Tab{5, "IMPACT OF THE INCIDENT", []Field{
		{"Was there any unauthorised access of data?", i.UnauthorisedDataAccess},
		{"Was there any intentional/unintentional release of data?", i.DataReleased},
		{"Was sensitive customer data leaked?", i.SensitiveCustomerDataLeaked},
		{"What type of customer data was potentially compromised?", i.CustomerDataTypes},
		{"What measures are in place to assess the potential impact of the incident on customers?", i.CustomerImpactAssessment},
		{"Estimated total number of customers impacted", i.CustomersImpacted},
		{"Were any stakeholders informed, including customers?", i.StakeholdersInformed},
		{"If yes, how were they informed?", i.HowInformed},
		{"Have any claims been lodged against the supervised entity as a result of the incident?", i.ClaimsLodged},
		{"Was clear and timely communication sent to customers about the nature and the potential impact of the incident?", i.TimelyCustomerCommunication},
		{"What measures are in place to ensure that affected customers are informed of their rights to data privacy and protection following the incident?", i.CustomerPrivacyRightsMeasures},
		{"What support services are in place to assist customers?", i.CustomerSupport},
		{"Are there any provisions in place for enhanced customer service?", i.EnhancedCustomerService},
		{"What was done to ensure that customers can reach out easily for assistance?", i.CustomerContactEase},
		{"What measures were in place to protect the data prior to the incident?", i.DataProtectionBefore},
		{"What immediate remedial actions were taken following the incident?", i.ImmediateRemedialActions},
		{"Was there any disruption of service?", i.ServiceDisrupted},
		{"Which services and components were affected?", i.ServicesAffected},
		{"What was the duration of the disruption?", i.DisruptionDuration},
		{"Was this a third-party service provider incident?", i.ThirdPartyIncident},
		{"What is the name of the third-party service provider?", i.ThirdPartyName},
		{"Was there any breach of legal or regulatory requirements?", i.LegalBreach},
		{"Was there any media coverage of this incident?", i.MediaCoverage},
		{"Provide a detailed description of the events", i.EventsDescription},
		{"What was the entry vector of the incident?", i.EntryVector},
		{"What caused the incident to occur?", i.Cause},
		{"Was the Incident Response Plan (IRP) activated?", i.IRPActivated},
		{"Was the Disaster Recovery Plan (DRP) activated?", i.DRPActivated},
		{"Was the Business Continuity Plan (BCP) activated?", i.BCPActivated},
		{"Was the incident escalated to the Board, Board Committees or Board-Delegated structures?", i.EscalatedToBoard},
		{"Is the incident resolved?", i.Resolved},
		{"If not, when do you expect to resolve the incident?", i.ExpectedResolution},
		{"What was the timeframe to recover from the incident?", i.RecoveryTime},
		{"Are the law enforcement or other security agencies involved in the investigation?", i.LawEnforcementInvolved},
		{"What strategies will be implemented to prevent a recurrence of such incident?", i.RecurrenceStrategy},
		{"Estimate direct financial loss in Rands", i.DirectLossRand},
		{"Estimate indirect financial loss in Rands", i.IndirectLossRand},
		{"Estimation of the financial impact on customers in Rands, if available", i.CustomerLossRand},
		{"Was there recovery of financial loss in accordance with civil liability and/or professional indemnity cover?", i.LossRecovered},
		{"What was the conclusion on the impact of the incident?", i.Conclusion},
		{"Lesson learnt from this incident", i.LessonsLearnt},
	}})
	return tabs
}

// ImmediateTabs are the tabs the template asks for with the immediate notification: 2 and 3, plus 4 for cyber.
func (in *Incident) ImmediateTabs(at time.Time) []Tab {
	var out []Tab
	for _, t := range in.Tabs(at) {
		if t.Number != 5 {
			out = append(out, t)
		}
	}
	return out
}

// WriteCSV writes tabs as CSV with the columns tab, label, value, so a person can paste them into the portal's template
// or attach them as evidence.
func WriteCSV(w io.Writer, tabs []Tab) error {
	cw := csv.NewWriter(w)
	if err := cw.Write([]string{"tab", "label", "value"}); err != nil {
		return err
	}
	for _, t := range tabs {
		for _, f := range t.Fields {
			if err := cw.Write([]string{fmt.Sprintf("%d. %s", t.Number, t.Name), f.Label, f.Value}); err != nil {
				return err
			}
		}
	}
	cw.Flush()
	return cw.Error()
}
