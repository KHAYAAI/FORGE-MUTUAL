package offers

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5/pgxpool"

	offersv1 "github.com/khayaai/forge-mutual/pkg/api/forge/offers/v1"
	"github.com/khayaai/forge-mutual/pkg/licence"
)

// These tests need PostgreSQL: PARTNER_OFFERS_TEST_DSN names a throwaway database whose tables are dropped.

type consents map[string]bool

func (c consents) Allowed(_ context.Context, party, purpose, scope string) (string, bool, error) {
	k := party + "|" + purpose + "|" + scope
	return "consent-" + party, c[k], nil
}

type parties map[string]bool

func (p parties) Active(_ context.Context, id string) (bool, error) { return p[id], nil }

type rig struct {
	t   *testing.T
	s   *Service
	now time.Time
	con consents
	pty parties
}

const alice = "party-alice"

func setup(t *testing.T) *rig {
	t.Helper()
	dsn := os.Getenv("PARTNER_OFFERS_TEST_DSN")
	if dsn == "" {
		t.Skip("PARTNER_OFFERS_TEST_DSN not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if _, err := pool.Exec(ctx, `drop table if exists referral, offer, provider_event, provider cascade`); err != nil {
		t.Fatal(err)
	}
	if err := Open(ctx, pool); err != nil {
		t.Fatal(err)
	}
	r := &rig{t: t, now: time.Date(2027, 3, 1, 10, 0, 0, 0, time.UTC), con: consents{}, pty: parties{alice: true}}
	r.s = &Service{Pool: pool, Consents: r.con, Parties: r.pty, Licence: licence.Gate{}, Enabled: true,
		Log: slog.New(slog.NewTextHandler(io.Discard, nil)), Now: func() time.Time { return r.now }}
	return r
}

func (r *rig) must(err error) {
	r.t.Helper()
	if err != nil {
		r.t.Fatal(err)
	}
}

// liveOffer registers a provider, has someone else verify its licence, writes an offer and has a third person switch it on.
func (r *rig) liveOffer(providerID, licenceType, category, offerID string) {
	r.t.Helper()
	ctx := context.Background()
	_, err := r.s.RegisterProvider(ctx, connect.NewRequest(&offersv1.RegisterProviderRequest{ProviderId: providerID, Name: "Acme Wealth (Pty) Ltd", LicenceType: licenceType,
		Regulator: "FSCA", LicenceNumber: "FSP 12345", LicenceSourceUrl: "https://www.fsca.co.za/register/acme", Actor: "anna"}))
	r.must(err)
	_, err = r.s.VerifyLicence(ctx, connect.NewRequest(&offersv1.VerifyLicenceRequest{ProviderId: providerID, Actor: "ben"}))
	r.must(err)
	_, err = r.s.UpsertOffer(ctx, connect.NewRequest(&offersv1.UpsertOfferRequest{OfferId: offerID, ProviderId: providerID, Category: category, Title: "Acme Tax-Free Savings",
		Summary: "A tax-free savings account.", HandoffUrl: "https://invest.acme-wealth.example/start", FeeDisclosure: "Forge receives no fee for introducing you.", Actor: "anna"}))
	r.must(err)
	_, err = r.s.SetOfferStatus(ctx, connect.NewRequest(&offersv1.SetOfferStatusRequest{OfferId: offerID, Status: "ACTIVE", Actor: "ben"}))
	r.must(err)
}

func (r *rig) list(category string) []*offersv1.Offer {
	r.t.Helper()
	res, err := r.s.ListOffers(context.Background(), connect.NewRequest(&offersv1.ListOffersRequest{PartyId: alice, Category: category}))
	r.must(err)
	return res.Msg.Offers
}

func TestSwitchedOffMeansEverythingIsRefused(t *testing.T) {
	r := setup(t)
	r.s.Enabled = false
	if _, err := r.s.ListOffers(context.Background(), connect.NewRequest(&offersv1.ListOffersRequest{PartyId: alice})); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("off by default: %v", err)
	}
	if _, err := r.s.RegisterProvider(context.Background(), connect.NewRequest(&offersv1.RegisterProviderRequest{ProviderId: "x-co", Actor: "a"})); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("the admin side too: %v", err)
	}
	prod, _ := licence.FromEnv(func(k string) string {
		if k == "FORGE_ENV" {
			return "production"
		}
		return ""
	})
	r.s.Enabled, r.s.Licence = true, prod
	if _, err := r.s.ListOffers(context.Background(), connect.NewRequest(&offersv1.ListOffersRequest{PartyId: alice})); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("production needs an approving decision: %v", err)
	}
}

func TestEveryOfferCarriesTheThirdPartyStatementInFull(t *testing.T) {
	r := setup(t)
	r.liveOffer("acme", "FSP", "INVESTMENTS", "acme-tfsa")
	r.liveOffer("coinco", "CASP", "CRYPTO_ASSETS", "coinco-onramp")
	offers := r.list("")
	if len(offers) != 2 {
		t.Fatalf("offers: %d", len(offers))
	}
	for _, o := range offers {
		for _, need := range []string{"THIRD-PARTY PRODUCT: provided, licensed and monitored by " + o.ProviderName + ", not by Forge Mutual.",
			"licensed by the FSCA", "licence number FSP 12345", "responsible for this product and for monitoring it",
			"does not provide this product, advise you on it, hold your money or assets in it, or guarantee it",
			"not covered by any deposit insurance through us", "Fees: Forge receives no fee"} {
			if !strings.Contains(o.Statement, need) {
				t.Errorf("%s: the statement lacks %q:\n%s", o.OfferId, need, o.Statement)
			}
		}
		if o.StatementHash != StatementHash(o.Statement) {
			t.Error("the hash must identify the text shown")
		}
	}
	if !strings.Contains(offers[0].Statement+offers[1].Statement, "Investments can go down as well as up") ||
		!strings.Contains(offers[0].Statement+offers[1].Statement, "not deposits, are not covered by deposit insurance") {
		t.Fatal("each category carries its own risk warning")
	}
	if got := r.list("CRYPTO_ASSETS"); len(got) != 1 || got[0].LicenceType != "CASP" {
		t.Fatalf("category filter: %+v", got)
	}
}

func TestNothingIsShownUnlessTheLicenceIsCurrentlyVerified(t *testing.T) {
	r := setup(t)
	ctx := context.Background()
	r.liveOffer("acme", "FSP", "INVESTMENTS", "acme-tfsa")
	if len(r.list("")) != 1 {
		t.Fatal("verified and active: shown")
	}
	r.now = r.now.Add(366 * 24 * time.Hour)
	if len(r.list("")) != 0 {
		t.Fatal("a licence check over a year old hides the offer")
	}
	r.now = time.Date(2027, 3, 1, 10, 0, 0, 0, time.UTC)
	_, err := r.s.SuspendProvider(ctx, connect.NewRequest(&offersv1.SuspendProviderRequest{ProviderId: "acme", Reason: "FSCA notice", Actor: "anna"}))
	r.must(err)
	if len(r.list("")) != 0 {
		t.Fatal("a suspended provider's offers vanish at once")
	}
	if _, err := r.s.SetOfferStatus(ctx, connect.NewRequest(&offersv1.SetOfferStatusRequest{OfferId: "acme-tfsa", Status: "ACTIVE", Actor: "carl"})); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("an offer cannot be switched on for a suspended provider: %v", err)
	}
	// An expiry date on the register hides offers after that date.
	r.liveOffer("beta", "FSP", "INVESTMENTS", "beta-fund")
	exp := r.now.Add(30 * 24 * time.Hour).Format(time.RFC3339)
	_, err = r.s.VerifyLicence(ctx, connect.NewRequest(&offersv1.VerifyLicenceRequest{ProviderId: "beta", Actor: "ben", LicenceExpiresAt: exp}))
	r.must(err)
	if len(r.list("")) != 1 {
		t.Fatal("beta is shown before its licence expires")
	}
	r.now = r.now.Add(31 * 24 * time.Hour)
	if len(r.list("")) != 0 {
		t.Fatal("an expired licence hides the offer")
	}
}

func TestComplianceRulesAreEnforced(t *testing.T) {
	r := setup(t)
	ctx := context.Background()
	reg := func(actor string) error {
		_, err := r.s.RegisterProvider(ctx, connect.NewRequest(&offersv1.RegisterProviderRequest{ProviderId: "acme", Name: "Acme", LicenceType: "FSP", Regulator: "FSCA",
			LicenceNumber: "FSP 1", LicenceSourceUrl: "https://www.fsca.co.za/x", Actor: actor}))
		return err
	}
	r.must(reg("anna"))
	if connect.CodeOf(reg("anna")) != connect.CodeAlreadyExists {
		t.Fatal("no duplicate providers")
	}
	if _, err := r.s.VerifyLicence(ctx, connect.NewRequest(&offersv1.VerifyLicenceRequest{ProviderId: "acme", Actor: "ANNA"})); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("the registrant cannot verify their own provider: %v", err)
	}
	for _, u := range []string{"http://insecure.example/", "https://localhost/", "https://10.0.0.1/x", "https://user:pw@x.example/", "https://nodot/"} {
		if _, err := r.s.RegisterProvider(ctx, connect.NewRequest(&offersv1.RegisterProviderRequest{ProviderId: "bad", Name: "B", LicenceType: "FSP", Regulator: "FSCA", LicenceNumber: "1", LicenceSourceUrl: u, Actor: "a"})); connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Fatalf("%s must be refused: %v", u, err)
		}
	}
	up := func(category, fee string) error {
		_, err := r.s.UpsertOffer(ctx, connect.NewRequest(&offersv1.UpsertOfferRequest{OfferId: "o1", ProviderId: "acme", Category: category, Title: "T", Summary: "S",
			HandoffUrl: "https://acme.example/go", FeeDisclosure: fee, Actor: "anna"}))
		return err
	}
	if connect.CodeOf(up("CRYPTO_ASSETS", "none")) != connect.CodeFailedPrecondition {
		t.Fatal("a crypto offer needs a CASP licence, not an FSP one")
	}
	if connect.CodeOf(up("INVESTMENTS", " ")) != connect.CodeInvalidArgument {
		t.Fatal("a fee disclosure is required")
	}
	r.must(up("INVESTMENTS", "No fee."))
	if _, err := r.s.SetOfferStatus(ctx, connect.NewRequest(&offersv1.SetOfferStatusRequest{OfferId: "o1", Status: "ACTIVE", Actor: "ben"})); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("not while the licence is unverified: %v", err)
	}
	_, err := r.s.VerifyLicence(ctx, connect.NewRequest(&offersv1.VerifyLicenceRequest{ProviderId: "acme", Actor: "ben"}))
	r.must(err)
	if _, err := r.s.SetOfferStatus(ctx, connect.NewRequest(&offersv1.SetOfferStatusRequest{OfferId: "o1", Status: "ACTIVE", Actor: "anna"})); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("the writer cannot switch their own offer on: %v", err)
	}
	if _, err := r.s.SetOfferStatus(ctx, connect.NewRequest(&offersv1.SetOfferStatusRequest{OfferId: "o1", Status: "ACTIVE", Actor: "ben"})); err != nil {
		t.Fatal(err)
	}
	// Changed words go back to DRAFT: someone must approve the new statement.
	r.must(up("INVESTMENTS", "Forge receives 0.5% of what you invest."))
	if len(r.list("")) != 0 {
		t.Fatal("a rewritten offer is hidden until switched on again")
	}
}

func TestAReferralNeedsTheCurrentStatementAcknowledgedConsentAndAnActiveParty(t *testing.T) {
	r := setup(t)
	ctx := context.Background()
	r.liveOffer("acme", "FSP", "INVESTMENTS", "acme-tfsa")
	o := r.list("")[0]
	refer := func(key string, ack bool, hash string) (*offersv1.Referral, error) {
		res, err := r.s.CreateReferral(ctx, connect.NewRequest(&offersv1.CreateReferralRequest{IdempotencyKey: key, PartyId: alice, OfferId: o.OfferId, AcknowledgedThirdParty: ack, StatementHash: hash}))
		if err != nil {
			return nil, err
		}
		return res.Msg.Referral, nil
	}
	if _, err := refer("k1", false, o.StatementHash); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("not acknowledged: %v", err)
	}
	if _, err := refer("k1", true, "stale"); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("a hash of some other text: %v", err)
	}
	if _, err := refer("k1", true, o.StatementHash); connect.CodeOf(err) != connect.CodeFailedPrecondition || !strings.Contains(err.Error(), "consent") {
		t.Fatalf("no consent to share details with this provider: %v", err)
	}
	r.con[alice+"|share_with_partner:acme|referral"] = true
	r.pty[alice] = false
	if _, err := refer("k1", true, o.StatementHash); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("a frozen party is introduced to no one: %v", err)
	}
	r.pty[alice] = true
	ref, err := refer("k1", true, o.StatementHash)
	if err != nil || !strings.HasPrefix(ref.HandoffUrl, "https://invest.acme-wealth.example/start?ref=ref-") || strings.Contains(ref.HandoffUrl, alice) {
		t.Fatalf("referral: %+v %v", ref, err)
	}
	again, err := refer("k1", true, o.StatementHash)
	if err != nil || again.ReferralId != ref.ReferralId {
		t.Fatalf("a retry returns the same referral: %+v %v", again, err)
	}
	list, err := r.s.ListReferrals(ctx, connect.NewRequest(&offersv1.ListReferralsRequest{PartyId: alice}))
	if err != nil || len(list.Msg.Referrals) != 1 {
		t.Fatalf("list: %v %v", list, err)
	}
	if other, _ := r.s.ListReferrals(ctx, connect.NewRequest(&offersv1.ListReferralsRequest{PartyId: "party-bob"})); len(other.Msg.Referrals) != 0 {
		t.Fatal("a party sees only their own referrals")
	}
	// Suspended after the fact: no new referral.
	_, err = r.s.SuspendProvider(ctx, connect.NewRequest(&offersv1.SuspendProviderRequest{ProviderId: "acme", Reason: "x", Actor: "anna"}))
	r.must(err)
	if _, err := refer("k2", true, o.StatementHash); connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("a hidden offer looks absent: %v", err)
	}
	_ = fmt.Sprint()
}
