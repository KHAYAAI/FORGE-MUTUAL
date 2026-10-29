// Package integration runs the onboarding workflow against the real party, consent, policy and ledger
// services. It builds their binaries, starts each with its own Postgres database, and drives the
// workflow in Temporal's in-process test environment.
//
// Set ONBOARDING_TEST_PG to a Postgres base DSN, for example postgres://postgres@127.0.0.1:5433/postgres?sslmode=disable.
// The test creates four throwaway databases and drops them afterwards.
package integration

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"go.temporal.io/sdk/testsuite"

	consentv1 "github.com/khayaai/forge-mutual/pkg/api/forge/consent/v1"
	"github.com/khayaai/forge-mutual/pkg/api/forge/consent/v1/consentv1connect"
	ledgerv1 "github.com/khayaai/forge-mutual/pkg/api/forge/ledger/v1"
	"github.com/khayaai/forge-mutual/pkg/api/forge/ledger/v1/ledgerv1connect"
	partyv1 "github.com/khayaai/forge-mutual/pkg/api/forge/party/v1"
	"github.com/khayaai/forge-mutual/pkg/api/forge/party/v1/partyv1connect"
	policyv1connect "github.com/khayaai/forge-mutual/pkg/api/forge/policy/v1/policyv1connect"
	"github.com/khayaai/forge-mutual/pkg/callerauth"
	"github.com/khayaai/forge-mutual/services/platform/onboarding/internal/kyc"
	"github.com/khayaai/forge-mutual/services/platform/onboarding/internal/onboarding"
	"github.com/khayaai/forge-mutual/services/platform/onboarding/internal/screening"
)

const secret = "0123456789abcdef-integration-secret"

// service is a started binary.
type service struct {
	name string
	url  string
	cmd  *exec.Cmd
}

// runtime holds everything one test run started, so cleanup can stop and drop it.
type runtime struct {
	t        *testing.T
	base     string
	dbs      []string
	services []*service
}

func randSuffix(t *testing.T) string {
	t.Helper()
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	return fmt.Sprintf("%x", b)
}

func randKey(t *testing.T) string {
	t.Helper()
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	return base64.StdEncoding.EncodeToString(b)
}

func freeAddr(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	return l.Addr().String()
}

func createDB(t *testing.T, base, name string) string {
	t.Helper()
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, base)
	if err != nil {
		t.Fatalf("connect to base database: %v", err)
	}
	defer conn.Close(ctx)
	if _, err := conn.Exec(ctx, "create database "+name); err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(base)
	if err != nil {
		t.Fatal(err)
	}
	u.Path = "/" + name
	return u.String()
}

func dropDB(base, name string) {
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, base)
	if err != nil {
		return
	}
	defer conn.Close(ctx)
	_, _ = conn.Exec(ctx, "drop database if exists "+name+" with (force)")
}

// build compiles one service binary from its import path.
func build(t *testing.T, dir, importPath, name string) string {
	t.Helper()
	out := filepath.Join(dir, name)
	cmd := exec.Command("go", "build", "-o", out, importPath)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("build %s: %v\n%s", name, err, stderr.String())
	}
	return out
}

// start runs a binary with the given environment and waits for its health endpoint.
func (r *runtime) start(bin, name, listenEnv string, env []string) *service {
	addr := freeAddr(r.t)
	cmd := exec.Command(bin)
	cmd.Env = append(append(os.Environ(), env...), listenEnv+"="+addr)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Start(); err != nil {
		r.t.Fatal(err)
	}
	svc := &service{name: name, url: "http://" + addr, cmd: cmd}
	r.services = append(r.services, svc)
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get(svc.url + "/healthz")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return svc
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	r.t.Fatalf("%s did not become healthy\n%s", name, out.String())
	return nil
}

func (r *runtime) cleanup() {
	for _, s := range r.services {
		if s.cmd.Process != nil {
			_ = s.cmd.Process.Kill()
			_, _ = s.cmd.Process.Wait()
		}
	}
	for _, db := range r.dbs {
		dropDB(r.base, db)
	}
}

func writeFile(t *testing.T, dir, name, body string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

// startStack builds and starts the four services the workflow calls.
func startStack(t *testing.T, base string) (*runtime, map[string]string) {
	t.Helper()
	dir := t.TempDir()
	r := &runtime{t: t, base: base}
	t.Cleanup(r.cleanup)
	suffix := randSuffix(t)

	partyDSN := createDB(t, base, "onb_party_"+suffix)
	r.dbs = append(r.dbs, "onb_party_"+suffix)
	consentDSN := createDB(t, base, "onb_consent_"+suffix)
	r.dbs = append(r.dbs, "onb_consent_"+suffix)
	policyDSN := createDB(t, base, "onb_policy_"+suffix)
	r.dbs = append(r.dbs, "onb_policy_"+suffix)
	ledgerDSN := createDB(t, base, "onb_ledger_"+suffix)
	r.dbs = append(r.dbs, "onb_ledger_"+suffix)

	partyBin := build(t, dir, "github.com/khayaai/forge-mutual/services/platform/party-service/cmd/party-service", "party")
	consentBin := build(t, dir, "github.com/khayaai/forge-mutual/services/platform/consent-service/cmd/consent-service", "consent")
	policyBin := build(t, dir, "github.com/khayaai/forge-mutual/services/shared/policy-service/cmd/policy-service", "policy")
	ledgerBin := build(t, dir, "github.com/khayaai/forge-mutual/services/bank/ledger-gateway-bank/cmd/ledger-gateway-bank", "ledger")

	partyCallers := writeFile(t, dir, "party-callers.json", `{"svc-onboarding":["CreateParty","GetParty","SetStatus","SetTier","AddRelationship"],"svc-test":["GetParty","ListRelationships"]}`)
	consentCallers := writeFile(t, dir, "consent-callers.json", `{"svc-onboarding":["Grant","Check","List"],"svc-test":["Check"]}`)
	policyCallers := writeFile(t, dir, "policy-callers.json", `{"svc-onboarding":["Evaluate"]}`)
	ledgerCallers := writeFile(t, dir, "ledger-callers.json", `{"svc-onboarding":["CreateAccounts","GetBalances"],"svc-test":["GetBalances"]}`)

	repo, _ := filepath.Abs("../../../../../policy")

	party := r.start(partyBin, "party", "PARTY_LISTEN", []string{
		"DATABASE_URL=" + partyDSN, "PARTY_CALLERS_FILE=" + partyCallers, "PARTY_PUBLISHER=log",
		"PARTY_SEAL_KEY=" + randKey(t), "PARTY_INDEX_KEY=" + randKey(t),
	})
	consent := r.start(consentBin, "consent", "CONSENT_LISTEN", []string{
		"DATABASE_URL=" + consentDSN, "CONSENT_CALLERS_FILE=" + consentCallers, "CONSENT_PUBLISHER=log",
	})
	policy := r.start(policyBin, "policy", "POLICY_LISTEN", []string{
		"DATABASE_URL=" + policyDSN, "POLICY_CALLERS_FILE=" + policyCallers, "POLICY_PUBLISHER=log",
		"POLICY_BUNDLE_DIR=" + repo, "POLICY_DEV_UNSIGNED=true",
	})
	ledger := r.start(ledgerBin, "ledger", "LEDGER_LISTEN", []string{
		"DATABASE_URL=" + ledgerDSN, "LEDGER_CALLERS_FILE=" + ledgerCallers, "LEDGER_STORE=memory", "LEDGER_PUBLISHER=log",
	})
	return r, map[string]string{"party": party.url, "consent": consent.url, "policy": policy.url, "ledger": ledger.url}
}

// activities wires the real activities to the started services, with the fake KYC provider and a
// fixture screening list that contains no one in the test application.
func activities(t *testing.T, urls map[string]string) *onboarding.Activities {
	t.Helper()
	caller := func(name string) connect.Option {
		return connect.WithInterceptors(connect.UnaryInterceptorFunc(func(next connect.UnaryFunc) connect.UnaryFunc {
			return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
				req.Header().Set(callerauth.DefaultHeader, name)
				return next(ctx, req)
			}
		}))
	}
	provider, err := kyc.NewFake([]byte(secret))
	if err != nil {
		t.Fatal(err)
	}
	scr, err := screening.NewFixture([]string{"Nobody Known"}, time.Now(), time.Hour, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	return &onboarding.Activities{
		Party:        partyv1connect.NewPartyServiceClient(http.DefaultClient, urls["party"], caller("svc-onboarding")),
		Consent:      consentv1connect.NewConsentServiceClient(http.DefaultClient, urls["consent"], caller("svc-onboarding")),
		Policy:       policyv1connect.NewPolicyServiceClient(http.DefaultClient, urls["policy"], caller("svc-onboarding")),
		Ledger:       ledgerv1connect.NewLedgerServiceClient(http.DefaultClient, urls["ledger"], caller("svc-onboarding")),
		KYC:          provider,
		Screener:     scr,
		PolicyEntity: "PLAT",
	}
}

// testCaller identifies the test's own read-back calls. It is allowed only the reads the assertions need.
func testCaller() connect.Option {
	return connect.WithInterceptors(connect.UnaryInterceptorFunc(func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			req.Header().Set(callerauth.DefaultHeader, "svc-test")
			return next(ctx, req)
		}
	}))
}

func application(sector string) onboarding.Input {
	return onboarding.Input{
		OnboardingID:       "3f0c7a1e-8c2d-4b6a-9e51-2d7f4a9b6c10",
		RequestHash:        "integration",
		LegalName:          "Forge Pilot (Pty) Ltd",
		RegistrationNumber: "2026/777",
		Country:            "ZA",
		SectorCode:         sector,
		Actor:              "founder",
		Directors: []onboarding.DirectorInput{
			{LegalName: "Thandi Dube", NationalID: "8001010012345", OwnershipBps: 6000, IsUBO: true},
			{LegalName: "Sipho Ndlovu", NationalID: "8002020012345", OwnershipBps: 4000},
		},
	}
}

// sendVerdicts signals every subject of the running workflow, as the webhook receiver would. The subject
// IDs come from the workflow's status query.
func sendVerdicts(t *testing.T, env *testsuite.TestWorkflowEnvironment, verdict string) {
	t.Helper()
	val, err := env.QueryWorkflow(onboarding.QueryStatus)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	var st onboarding.Status
	if err := val.Get(&st); err != nil {
		t.Fatal(err)
	}
	for _, id := range st.AllParties() {
		env.SignalWorkflow(onboarding.SignalKYCResult, onboarding.KYCSignal{SubjectPartyID: id, Verdict: verdict})
	}
}

func runWorkflow(t *testing.T, acts *onboarding.Activities, in onboarding.Input) (onboarding.Status, *testsuite.TestWorkflowEnvironment) {
	t.Helper()
	var ts testsuite.WorkflowTestSuite
	env := ts.NewTestWorkflowEnvironment()
	env.RegisterWorkflow(onboarding.OnboardBusiness)
	env.RegisterActivity(acts)
	env.RegisterDelayedCallback(func() { sendVerdicts(t, env, "APPROVED") }, time.Minute)
	env.ExecuteWorkflow(onboarding.OnboardBusiness, in)
	if err := env.GetWorkflowError(); err != nil {
		t.Fatalf("workflow error: %v", err)
	}
	var st onboarding.Status
	if err := env.GetWorkflowResult(&st); err != nil {
		t.Fatal(err)
	}
	return st, env
}

func TestApprovedBusinessIsActivatedWithAnAccount(t *testing.T) {
	base := os.Getenv("ONBOARDING_TEST_PG")
	if base == "" {
		t.Skip("ONBOARDING_TEST_PG not set")
	}
	r, urls := startStack(t, base)
	acts := activities(t, urls)
	st, _ := runWorkflow(t, acts, application("software_saas"))

	if st.State != onboarding.StateCompleted || st.Tier != "enhanced" || len(st.AccountIDs) != 1 {
		t.Fatalf("status = %+v", st)
	}
	ctx := context.Background()
	party := func(id string) *partyv1.Party {
		res, err := partyv1connect.NewPartyServiceClient(http.DefaultClient, urls["party"], testCaller()).GetParty(ctx,
			connect.NewRequest(&partyv1.GetPartyRequest{PartyId: id}))
		if err != nil {
			t.Fatalf("get party %s: %v", id, err)
		}
		return res.Msg.Party
	}
	if party(st.CompanyPartyID).Status != partyv1.PartyStatus_PARTY_STATUS_ACTIVE {
		t.Fatal("company must be ACTIVE after onboarding")
	}
	for _, id := range st.PersonPartyIDs {
		if party(id).Status != partyv1.PartyStatus_PARTY_STATUS_ACTIVE {
			t.Fatalf("person %s must be ACTIVE", id)
		}
	}
	if got := party(st.CompanyPartyID).Tier; got != "enhanced" {
		t.Fatalf("company tier = %q", got)
	}

	// The ledger account exists and is owned by the company.
	bal, err := ledgerv1connect.NewLedgerServiceClient(http.DefaultClient, urls["ledger"], testCaller()).GetBalances(ctx,
		connect.NewRequest(&ledgerv1.GetBalancesRequest{EntityCode: "BANK", AccountIds: st.AccountIDs}))
	if err != nil || len(bal.Msg.Balances) != 1 || bal.Msg.Balances[0].Asset != "ZAR" {
		t.Fatalf("ledger balance = %v, %v", bal, err)
	}

	// Each director is linked to the company: as a director, and as a UBO with a share.
	rels, err := partyv1connect.NewPartyServiceClient(http.DefaultClient, urls["party"], testCaller()).ListRelationships(ctx,
		connect.NewRequest(&partyv1.ListRelationshipsRequest{PartyId: st.CompanyPartyID}))
	if err != nil {
		t.Fatalf("list relationships: %v", err)
	}
	var directors, ubos int
	for _, r := range rels.Msg.Relationships {
		switch r.Type {
		case partyv1.RelationshipType_RELATIONSHIP_TYPE_DIRECTOR:
			directors++
		case partyv1.RelationshipType_RELATIONSHIP_TYPE_UBO:
			ubos++
		}
	}
	if directors != 2 || ubos != 1 {
		t.Fatalf("relationships: directors=%d ubos=%d, want 2 and 1", directors, ubos)
	}

	// The company's KYB consent is recorded and answers a check.
	chk, err := consentv1connect.NewConsentServiceClient(http.DefaultClient, urls["consent"], testCaller()).Check(ctx,
		connect.NewRequest(&consentv1.CheckRequest{PartyId: st.CompanyPartyID, Purpose: "kyb.processing", Scope: "company.registry"}))
	if err != nil || !chk.Msg.Allowed {
		t.Fatalf("consent check = %v, %v", chk, err)
	}
	_ = r
}

func TestSectorRefusalClosesPartiesAndCreatesNoAccount(t *testing.T) {
	base := os.Getenv("ONBOARDING_TEST_PG")
	if base == "" {
		t.Skip("ONBOARDING_TEST_PG not set")
	}
	_, urls := startStack(t, base)
	acts := activities(t, urls)
	st, _ := runWorkflow(t, acts, application("defence"))

	if st.State != onboarding.StateDeclined || !strings.HasPrefix(st.DeclineReason, "sector_not_eligible") {
		t.Fatalf("status = %+v", st)
	}
	if len(st.AccountIDs) != 0 {
		t.Fatal("a declined application must not create an account")
	}
	res, err := partyv1connect.NewPartyServiceClient(http.DefaultClient, urls["party"], testCaller()).GetParty(context.Background(),
		connect.NewRequest(&partyv1.GetPartyRequest{PartyId: st.CompanyPartyID}))
	if err != nil || res.Msg.Party.Status != partyv1.PartyStatus_PARTY_STATUS_CLOSED {
		t.Fatalf("declined company must be CLOSED: %v %v", res, err)
	}
}
