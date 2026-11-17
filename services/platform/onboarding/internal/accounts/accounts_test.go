package accounts

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5/pgxpool"

	ledgerv1 "github.com/khayaai/forge-mutual/pkg/api/forge/ledger/v1"
	partnerv1 "github.com/khayaai/forge-mutual/pkg/api/forge/partner/v1"
	partyv1 "github.com/khayaai/forge-mutual/pkg/api/forge/party/v1"
	paymentsv1 "github.com/khayaai/forge-mutual/pkg/api/forge/payments/v1"
	"github.com/khayaai/forge-mutual/pkg/licence"
	"github.com/khayaai/forge-mutual/services/platform/onboarding/internal/store"
	"github.com/khayaai/forge-mutual/services/platform/onboarding/internal/testdb"
)

type fakes struct {
	mu       sync.Mutex
	status   partyv1.PartyStatus
	ledger   map[string]string // idempotency key -> account
	specs    []*ledgerv1.AccountSpec
	partner  int               // accounts opened at the partner
	refs     map[string]string // reference -> account
	failRefs bool
}

func (f *fakes) GetParty(_ context.Context, r *connect.Request[partyv1.GetPartyRequest]) (*connect.Response[partyv1.GetPartyResponse], error) {
	return connect.NewResponse(&partyv1.GetPartyResponse{Party: &partyv1.Party{PartyId: r.Msg.PartyId, Status: f.status}}), nil
}

func (f *fakes) CreateAccounts(_ context.Context, r *connect.Request[ledgerv1.CreateAccountsRequest]) (*connect.Response[ledgerv1.CreateAccountsResponse], error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if id, ok := f.ledger[r.Msg.IdempotencyKey]; ok {
		return connect.NewResponse(&ledgerv1.CreateAccountsResponse{Status: ledgerv1.Status_STATUS_REPLAYED, AccountIds: []string{id}}), nil
	}
	id := fmt.Sprintf("acc-%04d-0000-4000-8000-000000000000", len(f.ledger)+1)
	f.ledger[r.Msg.IdempotencyKey] = id
	f.specs = append(f.specs, r.Msg.Accounts[0])
	return connect.NewResponse(&ledgerv1.CreateAccountsResponse{Status: ledgerv1.Status_STATUS_OK, AccountIds: []string{id}}), nil
}

func (f *fakes) OpenAccount(_ context.Context, r *connect.Request[partnerv1.OpenAccountRequest]) (*connect.Response[partnerv1.OpenAccountResponse], error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.partner++
	return connect.NewResponse(&partnerv1.OpenAccountResponse{Partner: "sim-partner-" + r.Msg.Asset, AccountId: fmt.Sprint(f.partner),
		Reference: fmt.Sprintf("FORGE%sPB%04d", r.Msg.Asset, f.partner)}), nil
}

func (f *fakes) RegisterInboundReference(_ context.Context, r *connect.Request[paymentsv1.RegisterInboundReferenceRequest]) (*connect.Response[paymentsv1.RegisterInboundReferenceResponse], error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failRefs {
		return nil, connect.NewError(connect.CodeUnavailable, errors.New("payments down"))
	}
	if acc, ok := f.refs[r.Msg.Reference]; ok && acc != r.Msg.AccountId {
		return nil, connect.NewError(connect.CodeAlreadyExists, errors.New("moved"))
	}
	f.refs[r.Msg.Reference] = r.Msg.AccountId
	return connect.NewResponse(&paymentsv1.RegisterInboundReferenceResponse{}), nil
}

func newOpener(t *testing.T, gate licence.Gate) (*Opener, *fakes) {
	t.Helper()
	dsn := testdb.Scratch(t, "ONBOARDING_TEST_PG")
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if _, err := store.Open(ctx, pool); err != nil {
		t.Fatal(err)
	}
	f := &fakes{status: partyv1.PartyStatus_PARTY_STATUS_ACTIVE, ledger: map[string]string{}, refs: map[string]string{}}
	return &Opener{Pool: pool, Parties: f, Ledger: f, Partners: f, References: f, Licence: gate, Entity: "BANK", Insured: true}, f
}

// Opening the same currency again, even from many requests at once, returns the one account: one ledger account, one
// account at the partner bank, one reference.
func TestOneAccountPerCurrencyEvenUnderConcurrentRequests(t *testing.T) {
	o, f := newOpener(t, licence.Gate{})
	var wg sync.WaitGroup
	got := make([]Account, 8)
	errs := make([]error, 8)
	for i := range got {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			got[i], errs[i] = o.Open(context.Background(), "party-1", "eur", "Thandi")
		}(i)
	}
	wg.Wait()
	for i := range got {
		if errs[i] != nil || got[i] != got[0] {
			t.Fatalf("request %d: %+v, %v (first: %+v)", i, got[i], errs[i], got[0])
		}
	}
	if f.partner != 1 || len(f.ledger) != 1 || len(f.refs) != 1 {
		t.Fatalf("partner accounts %d, ledger accounts %d, references %d; want one each", f.partner, len(f.ledger), len(f.refs))
	}
	if got[0].Asset != "EUR" || got[0].Reference != "FORGEEURPB0001" || f.refs[got[0].Reference] != got[0].AccountID {
		t.Fatalf("account = %+v, refs = %v", got[0], f.refs)
	}
	if spec := f.specs[0]; spec.Insured || spec.Asset != "EUR" || !spec.DebitsMustNotExceedCredits || spec.OwnerPartyId != "party-1" {
		t.Fatalf("a foreign balance is not insured and may not go negative: %+v", spec)
	}
}

// A rand account needs no partner; its reference names the account, and it carries the insurance disclosure.
func TestARandAccountHasItsOwnReference(t *testing.T) {
	o, f := newOpener(t, licence.Gate{})
	acc, err := o.Open(context.Background(), "party-2", "ZAR", "Thandi")
	if err != nil || f.partner != 0 || len(acc.Reference) != 17 || acc.Reference[:5] != "FORGE" {
		t.Fatalf("rand account: %+v, %v, partner calls %d", acc, err, f.partner)
	}
	if !f.specs[0].Insured {
		t.Fatal("a rand account at the sponsor carries the insurance flag it was configured with")
	}
}

// Only an active customer may open an account, and production refuses an unapproved currency before anything is made.
func TestRefusals(t *testing.T) {
	o, f := newOpener(t, licence.Gate{})
	f.status = partyv1.PartyStatus_PARTY_STATUS_KYC_PENDING
	if _, err := o.Open(context.Background(), "party-3", "USD", "Thandi"); !errors.Is(err, ErrNotActive) {
		t.Fatalf("a customer still being verified: %v", err)
	}
	if _, err := o.Open(context.Background(), "party-3", "JPY", "Thandi"); !errors.Is(err, ErrInvalid) {
		t.Fatalf("an unsupported currency: %v", err)
	}
	gate, _ := licence.FromEnv(func(k string) string { return map[string]string{"FORGE_ENV": "production"}[k] })
	o.Licence = gate
	f.status = partyv1.PartyStatus_PARTY_STATUS_ACTIVE
	if _, err := o.Open(context.Background(), "party-3", "GBP", "Thandi"); !errors.Is(err, licence.ErrNotApproved) {
		t.Fatalf("an unapproved pound in production: %v", err)
	}
	if len(f.ledger) != 0 || f.partner != 0 {
		t.Fatal("a refusal created something")
	}
}

// If payments cannot be reached, the account is not reported open; the next request registers the same reference
// for the same account, and opens nothing new.
func TestAFailedRegistrationIsFinishedByTheNextRequest(t *testing.T) {
	o, f := newOpener(t, licence.Gate{})
	f.failRefs = true
	if _, err := o.Open(context.Background(), "party-4", "USD", "Thandi"); err == nil {
		t.Fatal("reported open with no reference registered")
	}
	f.failRefs = false
	acc, err := o.Open(context.Background(), "party-4", "USD", "Thandi")
	if err != nil || f.partner != 1 || len(f.ledger) != 1 || f.refs[acc.Reference] != acc.AccountID {
		t.Fatalf("retry: %+v, %v; partner %d, ledger %d", acc, err, f.partner, len(f.ledger))
	}
}
