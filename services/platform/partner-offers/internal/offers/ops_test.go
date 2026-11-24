package offers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"

	ledgerv1connect "github.com/khayaai/forge-mutual/pkg/api/forge/ledger/v1/ledgerv1connect"
	offersv1 "github.com/khayaai/forge-mutual/pkg/api/forge/offers/v1"
	"github.com/khayaai/forge-mutual/services/platform/partner-offers/internal/ledgerfake"
)

type names map[string]string

func (n names) Name(_ context.Context, p string) (string, error) { return n[p], nil }

type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// provider's side of the delivery, recorded.
type inbox struct {
	mu     sync.Mutex
	status int
	got    []*http.Request
	bodies [][]byte
}

func (i *inbox) client() *http.Client {
	return &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
		b, _ := io.ReadAll(r.Body)
		i.mu.Lock()
		defer i.mu.Unlock()
		i.got, i.bodies = append(i.got, r), append(i.bodies, b)
		st := i.status
		if st == 0 {
			st = 200
		}
		return &http.Response{StatusCode: st, Body: io.NopCloser(strings.NewReader("")), Header: http.Header{}}, nil
	})}
}

func (i *inbox) count() int { i.mu.Lock(); defer i.mu.Unlock(); return len(i.got) }

const deliveryURL = "https://hooks.acme-wealth.example/forge"

// referral makes a live offer from a provider that can receive details, and refers alice to it (consent given, statement acknowledged).
func (r *rig) referral(cap string) (referralID string, key []byte) {
	r.t.Helper()
	ctx := context.Background()
	_, err := r.s.RegisterProvider(ctx, connect.NewRequest(&offersv1.RegisterProviderRequest{ProviderId: "acme", Name: "Acme Wealth (Pty) Ltd", LicenceType: "FSP", Regulator: "FSCA",
		LicenceNumber: "FSP 12345", LicenceSourceUrl: "https://www.fsca.co.za/register/acme", DeliveryUrl: deliveryURL, Actor: "anna"}))
	r.must(err)
	_, err = r.s.VerifyLicence(ctx, connect.NewRequest(&offersv1.VerifyLicenceRequest{ProviderId: "acme", Actor: "ben"}))
	r.must(err)
	_, err = r.s.UpsertOffer(ctx, connect.NewRequest(&offersv1.UpsertOfferRequest{OfferId: "acme-tfsa", ProviderId: "acme", Category: "INVESTMENTS", Title: "Acme Tax-Free Savings",
		Summary: "s", HandoffUrl: "https://invest.acme-wealth.example/start", ReferralFeeCapMinor: cap, Actor: "anna"}))
	r.must(err)
	_, err = r.s.SetOfferStatus(ctx, connect.NewRequest(&offersv1.SetOfferStatusRequest{OfferId: "acme-tfsa", Status: "ACTIVE", Actor: "ben"}))
	r.must(err)
	r.con[alice+"|share_with_partner|referral:acme"] = true
	dir := r.t.TempDir()
	key = []byte("0123456789abcdef-signing-key")
	r.must(os.WriteFile(filepath.Join(dir, "acme.key"), key, 0o600))
	r.s.SecretsDir = dir
	r.s.Names = names{alice: "Thandi Dube"}
	o := r.list("")[0]
	res, err := r.s.CreateReferral(ctx, connect.NewRequest(&offersv1.CreateReferralRequest{IdempotencyKey: "k", PartyId: alice, OfferId: o.OfferId, AcknowledgedThirdParty: true, StatementHash: o.StatementHash}))
	r.must(err)
	return res.Msg.Referral.ReferralId, key
}

func (r *rig) deliveryStatus(id string) (string, string, int) {
	var st, detail string
	var attempts int
	if err := r.s.Pool.QueryRow(context.Background(), `select status, detail, attempts from referral_delivery where referral_id = $1`, id).Scan(&st, &detail, &attempts); err != nil {
		r.t.Fatal(err)
	}
	return st, detail, attempts
}

// ---- the statement's fee sentence comes from the cap ----

func TestTheFeeSentenceIsGeneratedFromTheCapAndTheCapIsBounded(t *testing.T) {
	r := setup(t)
	ctx := context.Background()
	r.referral("150000")
	if st := r.list("")[0].Statement; !strings.Contains(st, "Forge Mutual may receive a referral fee of up to R1500.00 from Acme Wealth (Pty) Ltd") || !strings.Contains(st, "does not add to what you pay") || strings.Contains(st, "receives no fee") {
		t.Fatalf("the statement must carry the cap:\n%s", st)
	}
	up := func(cap string) error {
		_, err := r.s.UpsertOffer(ctx, connect.NewRequest(&offersv1.UpsertOfferRequest{OfferId: "acme-tfsa", ProviderId: "acme", Category: "INVESTMENTS", Title: "T", Summary: "s",
			HandoffUrl: "https://invest.acme-wealth.example/start", ReferralFeeCapMinor: cap, Actor: "anna"}))
		return err
	}
	for _, bad := range []string{"-1", "abc", "", "500001"} {
		if connect.CodeOf(up(bad)) != connect.CodeInvalidArgument {
			t.Fatalf("cap %q must be refused", bad)
		}
	}
	if err := up("500000"); err != nil {
		t.Fatalf("the largest allowed cap: %v", err)
	}
}

// ---- referral fee ledger entry ----

func feeRig(t *testing.T) (*rig, *ledgerfake.Fake) {
	r := setup(t)
	fake := ledgerfake.New()
	mux := http.NewServeMux()
	path, h := ledgerv1connect.NewLedgerServiceHandler(fake)
	mux.Handle(path, h)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	r.s.Ledger = ledgerv1connect.NewLedgerServiceClient(srv.Client(), srv.URL)
	if err := r.s.EnsureFeeAccounts(context.Background()); err != nil {
		t.Fatal(err)
	}
	return r, fake
}

func (r *rig) feeBy(who, ref, bank, amount string) (*offersv1.RecordReferralFeeResponse, error) {
	res, err := r.s.RecordReferralFee(context.Background(), connect.NewRequest(&offersv1.RecordReferralFeeRequest{ReferralId: ref, AmountMinor: amount, BankReference: bank, Officer: who}))
	if err != nil {
		return nil, err
	}
	return res.Msg, nil
}

// fee is the whole routine: Naledi enters it from the statement, Pieter confirms it.
func (r *rig) fee(ref, bank string, amount string) error {
	if res, err := r.feeBy("Naledi", ref, bank, amount); err != nil {
		return err
	} else if res.Fee != nil {
		return nil // already booked: this is a repeat
	}
	_, err := r.feeBy("Pieter", ref, bank, amount)
	return err
}

func TestNothingIsBookedUntilADifferentPersonConfirmsTheSameFee(t *testing.T) {
	r, fake := feeRig(t)
	ref, _ := r.referral("50000")
	res, err := r.feeBy("Naledi", ref, "STMT-5/001", "30000")
	if err != nil || res.Fee != nil || !res.AwaitingSecondPerson || fake.Balance(r.s.FeeIncome) != 0 {
		t.Fatalf("a proposal books nothing: %+v %v income %d", res, err, fake.Balance(r.s.FeeIncome))
	}
	if res, err = r.feeBy("naledi", ref, "STMT-5/001", "30000"); err != nil || res.Fee != nil || !res.AwaitingSecondPerson || fake.Balance(r.s.FeeIncome) != 0 {
		t.Fatalf("the same person confirming their own entry books nothing: %+v %v", res, err)
	}
	if _, err = r.feeBy("Pieter", ref, "STMT-5/001", "29999"); connect.CodeOf(err) != connect.CodeFailedPrecondition || fake.Balance(r.s.FeeIncome) != 0 {
		t.Fatalf("confirming a different amount must be refused: %v", err)
	}
	res, err = r.feeBy("Pieter", ref, "STMT-5/001", "30000")
	if err != nil || res.Fee == nil || res.Fee.RecordedBy != "Naledi" || res.Fee.ApprovedBy != "Pieter" || fake.Balance(r.s.FeeIncome) != 30000 {
		t.Fatalf("confirmed: %+v %v income %d", res, err, fake.Balance(r.s.FeeIncome))
	}
	var left int
	_ = r.s.Pool.QueryRow(context.Background(), `select count(*) from referral_fee_proposal`).Scan(&left)
	if left != 0 {
		t.Fatal("a booked fee leaves no proposal behind")
	}
	// the cap is checked again when it is booked, not only when it is proposed
	if _, err = r.feeBy("Naledi", ref, "STMT-5/002", "15000"); err != nil {
		t.Fatal(err)
	}
	if _, err = r.feeBy("Naledi", ref, "STMT-5/003", "15000"); err != nil {
		t.Fatal(err)
	}
	if _, err = r.feeBy("Pieter", ref, "STMT-5/002", "15000"); err != nil {
		t.Fatal(err)
	}
	if _, err = r.feeBy("Pieter", ref, "STMT-5/003", "15000"); connect.CodeOf(err) != connect.CodeFailedPrecondition || fake.Balance(r.s.FeeIncome) != 45000 {
		t.Fatalf("two proposals cannot together pass the cap: %v income %d", err, fake.Balance(r.s.FeeIncome))
	}
}

func TestAReferralFeeIsBookedOnceAgainstTheBankReceiptWithinTheCapByTwoPeople(t *testing.T) {
	r, fake := feeRig(t)
	ref, _ := r.referral("50000")
	if err := r.fee(ref, "STMT-1/001", "30000"); err != nil {
		t.Fatal(err)
	}
	if fake.Balance(r.s.FeeIncome) != 30000 || fake.Balance(r.s.SponsorPool) != -30000 {
		t.Fatalf("income %d, pool %d", fake.Balance(r.s.FeeIncome), fake.Balance(r.s.SponsorPool))
	}
	if err := r.fee(ref, "STMT-1/001", "30000"); err != nil || fake.Balance(r.s.FeeIncome) != 30000 {
		t.Fatalf("the same receipt again must change nothing: %v %d", err, fake.Balance(r.s.FeeIncome))
	}
	if connect.CodeOf(r.fee(ref, "STMT-1/001", "10000")) != connect.CodeAlreadyExists {
		t.Fatal("a bank reference cannot be reused for another amount")
	}
	if connect.CodeOf(r.fee(ref, "STMT-1/002", "30000")) != connect.CodeFailedPrecondition {
		t.Fatal("30 000 on top of 30 000 is over the cap of 50 000 the customer was told")
	}
	if err := r.fee(ref, "STMT-1/003", "20000"); err != nil {
		t.Fatalf("exactly the rest of the cap: %v", err)
	}
	for name, c := range map[string]func() error{
		"no officer":    func() error { _, err := r.feeBy("", ref, "X-1234", "1"); return err },
		"bad reference": func() error { return r.fee(ref, "x y", "1") },
		"zero amount":   func() error { return r.fee(ref, "STMT-9", "0") },
		"no referral":   func() error { return r.fee("nope", "STMT-9", "1") },
	} {
		if err := c(); err == nil {
			t.Fatalf("%s must be refused", name)
		}
	}
	if fake.Balance(r.s.FeeIncome) != 50000 {
		t.Fatalf("income %d, want exactly the 50 000 booked", fake.Balance(r.s.FeeIncome))
	}
	list, err := r.s.ListAdminReferrals(context.Background(), connect.NewRequest(&offersv1.ListAdminReferralsRequest{}))
	if err != nil || len(list.Msg.Referrals) != 1 || list.Msg.Referrals[0].FeeCapMinor != "50000" || list.Msg.Referrals[0].FeesRecordedMinor != "50000" {
		t.Fatalf("admin list: %v %+v", err, list)
	}
}

func TestAnOfferThatPaysNothingBooksNoFeeAndNoLedgerMeansNoFee(t *testing.T) {
	r, fake := feeRig(t)
	ref, _ := r.referral("0")
	if connect.CodeOf(r.fee(ref, "STMT-2/001", "100")) != connect.CodeFailedPrecondition || fake.Balance(r.s.FeeIncome) != 0 {
		t.Fatal("an offer with a zero cap pays Forge nothing")
	}
	r.s.Ledger = nil
	if connect.CodeOf(r.fee(ref, "STMT-2/002", "100")) != connect.CodeFailedPrecondition {
		t.Fatal("no ledger, no fee")
	}
}

func TestReceiptsRacingForTheLastOfTheCapCannotPassIt(t *testing.T) {
	r, fake := feeRig(t)
	ref, _ := r.referral("50000")
	var wg sync.WaitGroup
	var mu sync.Mutex
	ok := 0
	for i := range 6 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if r.fee(ref, fmt.Sprintf("RACE-%02d", i), "20000") == nil {
				mu.Lock()
				ok++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if ok != 2 || fake.Balance(r.s.FeeIncome) != 40000 {
		t.Fatalf("%d booked, income %d; R200 receipts under a R500 cap is 2", ok, fake.Balance(r.s.FeeIncome))
	}
}

// ---- sending the customer's details ----

func TestOnlyTheConsentedFieldsAreSentSignedAndOnce(t *testing.T) {
	r := setup(t)
	ref, key := r.referral("0")
	in := &inbox{}
	r.s.HTTP = in.client()
	if st, _, _ := r.deliveryStatus(ref); st != "PENDING" {
		t.Fatalf("queued, not sent, when the customer clicked: %s", st)
	}
	if in.count() != 0 {
		t.Fatal("nothing is sent at referral time")
	}
	n, err := r.s.Deliver(context.Background())
	if err != nil || n != 1 || in.count() != 1 {
		t.Fatalf("deliver: %d %v sent %d", n, err, in.count())
	}
	req, body := in.got[0], in.bodies[0]
	if req.Method != "POST" || req.URL.String() != deliveryURL || req.Header.Get("X-Forge-Signature") != Sign(key, body) || req.Header.Get("X-Forge-Referral-Id") != ref || !strings.HasPrefix(req.Header.Get("X-Forge-Signature"), "sha256=") {
		t.Fatalf("request: %s %s headers %v", req.Method, req.URL, req.Header)
	}
	var top map[string]any
	_ = json.Unmarshal(body, &top)
	keys := []string{}
	for k := range top {
		keys = append(keys, k)
	}
	if len(keys) != 6 || top["referral_id"] != ref || top["provider_id"] != "acme" || top["consent_id"] == "" {
		t.Fatalf("the envelope: %v", top)
	}
	details := top["details"].(map[string]any)
	want := map[string]bool{}
	for _, label := range SharedFields {
		want[detailKeys[label]] = true
	}
	if len(details) != len(want) {
		t.Fatalf("details %v, want exactly %v: the consent promised only %v", details, want, SharedFields)
	}
	for k := range details {
		if !want[k] {
			t.Fatalf("detail %q was not in what the customer consented to", k)
		}
	}
	if details["name"] != "Thandi Dube" || bytes.Contains(body, []byte("8001")) {
		t.Fatalf("details: %v", details)
	}
	if st, _, a := r.deliveryStatus(ref); st != "SENT" || a != 1 {
		t.Fatalf("status %s attempts %d", st, a)
	}
	var hash string
	_ = r.s.Pool.QueryRow(context.Background(), `select payload_hash from referral_delivery where referral_id=$1`, ref).Scan(&hash)
	if len(hash) != 64 {
		t.Fatalf("only a hash of what was sent is kept: %q", hash)
	}
	if n, _ := r.s.Deliver(context.Background()); n != 0 || in.count() != 1 {
		t.Fatal("a delivered referral is never sent again")
	}
}

func TestEverySharedFieldHasAKeyTheSenderCanFill(t *testing.T) {
	for _, label := range SharedFields {
		if _, ok := detailKeys[label]; !ok {
			t.Fatalf("%q is promised to the customer but the sender cannot fill it", label)
		}
	}
}

func TestNothingIsSentIfConsentIsWithdrawnTheProviderIsSuspendedOrThereIsNoAddress(t *testing.T) {
	ctx := context.Background()
	// consent withdrawn before the sender runs
	r := setup(t)
	ref, _ := r.referral("0")
	in := &inbox{}
	r.s.HTTP = in.client()
	r.con[alice+"|share_with_partner|referral:acme"] = false
	if n, _ := r.s.Deliver(ctx); n != 0 || in.count() != 0 {
		t.Fatal("details were sent after the customer withdrew")
	}
	if st, d, _ := r.deliveryStatus(ref); st != "CANCELLED" || !strings.Contains(d, "withdrew") {
		t.Fatalf("%s %s", st, d)
	}
	// provider suspended
	r = setup(t)
	ref, _ = r.referral("0")
	in = &inbox{}
	r.s.HTTP = in.client()
	_, err := r.s.SuspendProvider(ctx, connect.NewRequest(&offersv1.SuspendProviderRequest{ProviderId: "acme", Reason: "x", Actor: "anna"}))
	r.must(err)
	_, _ = r.s.Deliver(ctx)
	if st, _, _ := r.deliveryStatus(ref); st != "CANCELLED" || in.count() != 0 {
		t.Fatalf("a suspended provider gets nothing: %s", st)
	}
	// frozen customer
	r = setup(t)
	ref, _ = r.referral("0")
	in = &inbox{}
	r.s.HTTP = in.client()
	r.pty[alice] = false
	_, _ = r.s.Deliver(ctx)
	if st, _, _ := r.deliveryStatus(ref); st != "CANCELLED" || in.count() != 0 {
		t.Fatalf("a frozen customer's details are not sent: %s", st)
	}
	// no delivery address
	r = setup(t)
	_, _ = r.s.RegisterProvider(ctx, connect.NewRequest(&offersv1.RegisterProviderRequest{ProviderId: "plain", Name: "Plain Co", LicenceType: "FSP", Regulator: "FSCA", LicenceNumber: "1", LicenceSourceUrl: "https://www.fsca.co.za/x", Actor: "anna"}))
	_, _ = r.s.VerifyLicence(ctx, connect.NewRequest(&offersv1.VerifyLicenceRequest{ProviderId: "plain", Actor: "ben"}))
	_, _ = r.s.UpsertOffer(ctx, connect.NewRequest(&offersv1.UpsertOfferRequest{OfferId: "plain-1", ProviderId: "plain", Category: "INVESTMENTS", Title: "T", Summary: "s", HandoffUrl: "https://plain.example/go", ReferralFeeCapMinor: "0", Actor: "anna"}))
	_, _ = r.s.SetOfferStatus(ctx, connect.NewRequest(&offersv1.SetOfferStatusRequest{OfferId: "plain-1", Status: "ACTIVE", Actor: "ben"}))
	r.con[alice+"|share_with_partner|referral:plain"] = true
	r.s.Names = names{alice: "Thandi Dube"}
	o := r.list("")[0]
	res, err := r.s.CreateReferral(ctx, connect.NewRequest(&offersv1.CreateReferralRequest{IdempotencyKey: "k", PartyId: alice, OfferId: o.OfferId, AcknowledgedThirdParty: true, StatementHash: o.StatementHash}))
	r.must(err)
	if st, _, _ := r.deliveryStatus(res.Msg.Referral.ReferralId); st != "NOT_CONFIGURED" {
		t.Fatalf("a provider with no address is sent nothing: %s", st)
	}
}

func TestFailedDeliveriesAreRetriedWithGrowingGapsThenGiveUp(t *testing.T) {
	r := setup(t)
	ref, _ := r.referral("0")
	in := &inbox{status: 503}
	r.s.HTTP = in.client()
	ctx := context.Background()
	if n, _ := r.s.Deliver(ctx); n != 0 || in.count() != 1 {
		t.Fatal("first attempt")
	}
	if st, d, a := r.deliveryStatus(ref); st != "PENDING" || a != 1 || !strings.Contains(d, "503") {
		t.Fatalf("after one failure: %s %s %d", st, d, a)
	}
	if _, _ = r.s.Deliver(ctx); in.count() != 1 {
		t.Fatal("it must wait before trying again")
	}
	r.now = r.now.Add(2 * time.Minute)
	_, _ = r.s.Deliver(ctx)
	if in.count() != 2 {
		t.Fatalf("second attempt after the first gap: %d", in.count())
	}
	in.mu.Lock()
	in.status = 200
	in.mu.Unlock()
	r.now = r.now.Add(10 * time.Minute)
	if n, _ := r.s.Deliver(ctx); n != 1 {
		t.Fatal("it succeeds once the provider is back")
	}
	if st, _, a := r.deliveryStatus(ref); st != "SENT" || a != 3 {
		t.Fatalf("%s %d", st, a)
	}
	// a provider that never recovers: FAILED after MaxAttempts
	r = setup(t)
	ref, _ = r.referral("0")
	in = &inbox{status: 500}
	r.s.HTTP = in.client()
	for range MaxAttempts + 2 {
		_, _ = r.s.Deliver(ctx)
		r.now = r.now.Add(24 * time.Hour)
	}
	if st, d, a := r.deliveryStatus(ref); st != "FAILED" || a != MaxAttempts || !strings.Contains(d, "gave up") || in.count() != MaxAttempts {
		t.Fatalf("%s %s %d tries %d", st, d, a, in.count())
	}
	// a refusal is final
	r = setup(t)
	ref, _ = r.referral("0")
	in = &inbox{status: 400}
	r.s.HTTP = in.client()
	_, _ = r.s.Deliver(ctx)
	if st, d, _ := r.deliveryStatus(ref); st != "FAILED" || !strings.Contains(d, "400") {
		t.Fatalf("a 400 is not retried: %s %s", st, d)
	}
}

func TestAMissingSigningKeyIsRetriedNotSent(t *testing.T) {
	r := setup(t)
	ref, _ := r.referral("0")
	in := &inbox{}
	r.s.HTTP = in.client()
	r.s.SecretsDir = t.TempDir() // no key in it
	_, _ = r.s.Deliver(context.Background())
	if st, d, _ := r.deliveryStatus(ref); st != "PENDING" || in.count() != 0 || !strings.Contains(d, "signing key") {
		t.Fatalf("%s %s sent %d", st, d, in.count())
	}
}

func TestTheProviderClientRefusesPrivateAddressesAndRedirects(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(200) }))
	defer srv.Close()
	if _, err := publicClient().Get(srv.URL); err == nil || !strings.Contains(err.Error(), "non-public") {
		t.Fatalf("a loopback address must be refused: %v", err)
	}
	for _, ip := range []string{"10.0.0.1", "192.168.1.1", "172.16.0.1", "169.254.169.254", "127.0.0.1", "0.0.0.0"} {
		if _, err := publicClient().Get("http://" + net.JoinHostPort(ip, "9")); err == nil || !strings.Contains(err.Error(), "non-public") {
			t.Fatalf("%s must be refused: %v", ip, err)
		}
	}
}

// ---- automatic licence checks ----

func writeRegister(t *testing.T, path string, rows ...string) {
	t.Helper()
	body := "regulator,licence_type,licence_number,name,status,expires_on\n" + strings.Join(rows, "\n") + "\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func (r *rig) touch(path string) { _ = os.Chtimes(path, r.now, r.now) }

func TestTheRegisterCheckConfirmsSuspendsAndFailsClosed(t *testing.T) {
	r := setup(t)
	ctx := context.Background()
	r.referral("0")
	path := filepath.Join(t.TempDir(), "register.csv")
	reg := &CSVRegister{Path: path, MaxAge: 48 * time.Hour, Now: func() time.Time { return r.now }}
	r.s.Register, r.s.RegisterMaxAge = reg, 35*24*time.Hour

	if len(r.list("")) != 0 {
		t.Fatal("with a register configured, a provider never checked is hidden")
	}
	writeRegister(t, path, "FSCA,FSP,FSP 12345,ACME WEALTH PTY LTD,ACTIVE,")
	r.touch(path)
	res, err := r.s.Recheck(ctx)
	if err != nil || res.Confirmed != 1 || len(r.list("")) != 1 {
		t.Fatalf("confirmed: %+v %v shown %d", res, err, len(r.list("")))
	}
	// no further check for 36 days: hidden again
	r.now = r.now.Add(36 * 24 * time.Hour)
	if len(r.list("")) != 0 {
		t.Fatal("a provider the register has not confirmed for 35 days is hidden")
	}
	// the extract goes stale: nothing changes, nothing is trusted
	if res, _ := r.s.Recheck(ctx); res.Unchecked != 1 || res.Suspended != 0 {
		t.Fatalf("a stale extract: %+v", res)
	}
	for name, row := range map[string]string{
		"not listed":     "FSCA,FSP,FSP 99999,ACME WEALTH PTY LTD,ACTIVE,",
		"different name": "FSCA,FSP,FSP 12345,SOMEONE ELSE PTY LTD,ACTIVE,",
		"revoked":        "FSCA,FSP,FSP 12345,Acme Wealth (Pty) Ltd,REVOKED,",
		"expired":        "FSCA,FSP,FSP 12345,Acme Wealth (Pty) Ltd,ACTIVE,2000-01-01",
	} {
		r2 := setup(t)
		r2.referral("0")
		p2 := filepath.Join(t.TempDir(), "register.csv")
		r2.s.Register = &CSVRegister{Path: p2, MaxAge: 48 * time.Hour, Now: func() time.Time { return r2.now }}
		writeRegister(t, p2, row)
		r2.touch(p2)
		res, err := r2.s.Recheck(ctx)
		var st, reason string
		_ = r2.s.Pool.QueryRow(ctx, `select status, status_reason from provider where provider_id='acme'`).Scan(&st, &reason)
		if err != nil || res.Suspended != 1 || st != "SUSPENDED" || !strings.HasPrefix(reason, "register check:") || len(r2.list("")) != 0 {
			t.Fatalf("%s: %+v %v status %s %q", name, res, err, st, reason)
		}
		var ev int
		_ = r2.s.Pool.QueryRow(ctx, `select count(*) from provider_event where provider_id='acme' and action='AUTO_SUSPENDED' and actor='register-check'`).Scan(&ev)
		if ev != 1 {
			t.Fatalf("%s: the suspension must be on the audit trail", name)
		}
	}
	// only a person can reinstate it
	r3 := setup(t)
	r3.referral("0")
	p3 := filepath.Join(t.TempDir(), "register.csv")
	r3.s.Register = &CSVRegister{Path: p3, MaxAge: 48 * time.Hour, Now: func() time.Time { return r3.now }}
	writeRegister(t, p3, "FSCA,FSP,FSP 12345,Acme Wealth,SUSPENDED,")
	r3.touch(p3)
	_, _ = r3.s.Recheck(ctx)
	writeRegister(t, p3, "FSCA,FSP,FSP 12345,Acme Wealth,ACTIVE,")
	r3.touch(p3)
	if res, _ := r3.s.Recheck(ctx); res.Confirmed != 0 || len(r3.list("")) != 0 {
		t.Fatalf("a suspended provider stays suspended until a person verifies it: %+v", res)
	}
}

func TestAConfirmedProviderCanStayVisibleBeyondAYearWhenTheRegisterRenewsIt(t *testing.T) {
	r := setup(t)
	ctx := context.Background()
	r.referral("0")
	path := filepath.Join(t.TempDir(), "register.csv")
	r.s.Register, r.s.RegisterMaxAge, r.s.RegisterRenews = &CSVRegister{Path: path, MaxAge: 48 * time.Hour, Now: func() time.Time { return r.now }}, 35*24*time.Hour, true
	for i := 0; i < 14; i++ { // 14 monthly checks: over a year
		writeRegister(t, path, "FSCA,FSP,FSP 12345,Acme Wealth (Pty) Ltd,ACTIVE,")
		r.touch(path)
		if res, err := r.s.Recheck(ctx); err != nil || res.Confirmed != 1 {
			t.Fatalf("month %d: %+v %v", i, res, err)
		}
		r.now = r.now.Add(30 * 24 * time.Hour)
	}
	if len(r.list("")) != 1 {
		t.Fatal("a provider the register keeps confirming must not drop out after twelve months")
	}
	var by string
	_ = r.s.Pool.QueryRow(ctx, `select licence_verified_by from provider where provider_id='acme'`).Scan(&by)
	if by != "register-check" {
		t.Fatalf("the renewal must say who did it: %q", by)
	}
	// and without renewal the same year hides it
	r2 := setup(t)
	r2.referral("0")
	p2 := filepath.Join(t.TempDir(), "register.csv")
	r2.s.Register, r2.s.RegisterMaxAge = &CSVRegister{Path: p2, MaxAge: 48 * time.Hour, Now: func() time.Time { return r2.now }}, 35*24*time.Hour
	for i := 0; i < 14; i++ {
		writeRegister(t, p2, "FSCA,FSP,FSP 12345,Acme Wealth (Pty) Ltd,ACTIVE,")
		r2.touch(p2)
		_, _ = r2.s.Recheck(ctx)
		r2.now = r2.now.Add(30 * 24 * time.Hour)
	}
	if len(r2.list("")) != 0 {
		t.Fatal("without renewal the human verification still lapses after twelve months")
	}
}

func TestRegisterNamesAndExtractFormat(t *testing.T) {
	if normName("Acme Wealth (Pty) Ltd") != normName("ACME WEALTH PTY LTD") || normName("Acme Wealth") == normName("Acme Capital") {
		t.Fatal("name matching")
	}
	path := filepath.Join(t.TempDir(), "r.csv")
	reg := &CSVRegister{Path: path}
	if _, err := reg.Lookup(context.Background(), "FSCA", "FSP", "1"); err == nil {
		t.Fatal("a missing extract is an error, not 'not listed'")
	}
	_ = os.WriteFile(path, []byte("a,b\n1,2\n"), 0o600)
	if _, err := reg.Lookup(context.Background(), "FSCA", "FSP", "1"); err == nil {
		t.Fatal("a malformed extract is an error")
	}
	_ = os.WriteFile(path, []byte("regulator,licence_type,licence_number,name,status,expires_on\nFSCA,FSP,1,A,ACTIVE,not-a-date\n"), 0o600)
	reg = &CSVRegister{Path: path}
	if _, err := reg.Lookup(context.Background(), "FSCA", "FSP", "1"); err == nil || errors.Is(err, nil) {
		t.Fatal("a bad date is an error")
	}
}
