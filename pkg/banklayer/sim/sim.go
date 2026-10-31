// Package sim is an in-memory sponsor bank and USD partner bank for local development, CI and tests. It keeps
// accounts, payments, pooled balances and statements, and lets tests inject inbound credits. It implements
// banklayer.BankLayer and is checked by the same contract suite as the HTTP client. Its behaviour is ours, not
// any bank's: the real sponsor's wire format and rules are unverified (VERIFY-SPONSOR, VERIFY-PARTNER).
package sim

import (
	"context"
	"crypto/sha256"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/khayaai/forge-mutual/pkg/banklayer"
)

var accountNumber = regexp.MustCompile(`^[0-9]{6,16}$`)

// Bank is the simulator. Use New; the zero value is not usable.
type Bank struct {
	mu       sync.Mutex
	now      func() time.Time
	seq      int
	accounts map[string]*banklayer.Account
	payments map[string]*payment
	pools    map[string]*pool // sponsor pooled account, per asset
	partner  map[string]*pool // USD partner bank, per asset
	caps     banklayer.Capabilities
}

type pool struct {
	moves []banklayer.Movement
}

type payment struct {
	fingerprint [32]byte
	req         banklayer.PayRequest
	partner     bool
	status      string
	polls       int
}

// New returns a simulator on the sponsor route. now supplies booking times; nil means time.Now.
func New(now func() time.Time) *Bank {
	if now == nil {
		now = time.Now
	}
	return &Bank{
		now:      now,
		accounts: map[string]*banklayer.Account{},
		payments: map[string]*payment{},
		pools:    map[string]*pool{},
		partner:  map[string]*pool{},
		caps:     banklayer.SponsorCapabilities(),
	}
}

// Capabilities implements banklayer.BankLayer.
func (b *Bank) Capabilities() banklayer.Capabilities { return b.caps }

// OpenAccount implements banklayer.BankLayer. The account is a sub-ledger reference inside the pooled account.
func (b *Bank) OpenAccount(_ context.Context, req banklayer.OpenAccountRequest) (banklayer.Account, error) {
	if req.Owner == "" || req.Asset == "" {
		return banklayer.Account{}, fmt.Errorf("%w: owner and asset are required", banklayer.ErrInvalid)
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.seq++
	id := fmt.Sprintf("SP%08d", b.seq)
	acc := &banklayer.Account{
		ID: id, Owner: req.Owner, Asset: req.Asset, Reference: "FORGE" + id, Status: "OPEN", OpenedAt: b.now(),
	}
	b.accounts[id] = acc
	b.poolLocked(req.Asset, false)
	return *acc, nil
}

// CloseAccount implements banklayer.BankLayer. Closing a closed account is a no-op so retries are safe.
func (b *Bank) CloseAccount(_ context.Context, accountID, reason string) error {
	if reason == "" {
		return fmt.Errorf("%w: a reason is required", banklayer.ErrInvalid)
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	acc, ok := b.accounts[accountID]
	if !ok {
		return banklayer.ErrNotFound
	}
	acc.Status = "CLOSED"
	return nil
}

// Rails implements banklayer.BankLayer.
func (b *Bank) Rails() banklayer.Rail { return sponsorRail{b} }

// ForeignCurrency implements banklayer.BankLayer. v1 has one partner, for USD.
func (b *Bank) ForeignCurrency() []banklayer.ForeignAccountProvider {
	return []banklayer.ForeignAccountProvider{partnerUSD{b}}
}

// StatementSource implements banklayer.BankLayer.
func (b *Bank) StatementSource() banklayer.StatementSource { return sponsorStatements{b} }

// DepositInsurance implements banklayer.BankLayer. On the sponsor route the sponsor reports deposit insurance.
func (b *Bank) DepositInsurance() banklayer.InsuranceReporter { return sponsorInsurance{} }

// PrudentialReporter implements banklayer.BankLayer. Nothing is required of Forge on the sponsor route.
func (b *Bank) PrudentialReporter() banklayer.PrudentialReporter { return noPrudential{} }

// InjectInbound books a credit to the pool, as the sponsor would after a payer's transfer. It is a test and
// simulator hook, not part of BankLayer. partner selects the USD partner bank instead of the sponsor pool.
func (b *Bank) InjectInbound(asset string, amountMinor int64, reference string, partner bool) (banklayer.Movement, error) {
	if !banklayer.ValidAmount(amountMinor) || asset == "" {
		return banklayer.Movement{}, banklayer.ErrInvalid
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	p := b.poolLocked(asset, partner)
	m := banklayer.Movement{
		ID:          fmt.Sprintf("%s-IN-%08d", asset, len(p.moves)+1),
		Direction:   "IN",
		AmountMinor: amountMinor,
		Asset:       asset,
		Reference:   reference,
		BookedAt:    b.now(),
	}
	p.moves = append(p.moves, m)
	return m, nil
}

func (b *Bank) poolLocked(asset string, partner bool) *pool {
	pools := b.pools
	if partner {
		pools = b.partner
	}
	p, ok := pools[asset]
	if !ok {
		p = &pool{}
		pools[asset] = p
	}
	return p
}

func fingerprint(req banklayer.PayRequest) [32]byte {
	return sha256.Sum256([]byte(strings.Join([]string{
		req.ID, req.Asset, fmt.Sprint(req.AmountMinor), req.Beneficiary,
		req.AccountNumber, req.BankCode, req.Reference,
	}, "\x00")))
}

// pay accepts or rejects a payment. It is idempotent by ID: the same request returns the same status, and a
// different request under a reused ID is a conflict.
func (b *Bank) pay(req banklayer.PayRequest, partner bool) (banklayer.PayResult, error) {
	if req.ID == "" || req.Asset == "" || req.Beneficiary == "" || !banklayer.ValidAmount(req.AmountMinor) {
		return banklayer.PayResult{}, fmt.Errorf("%w: id, asset, beneficiary and a positive amount are required", banklayer.ErrInvalid)
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	fp := fingerprint(req)
	if prev, ok := b.payments[req.ID]; ok {
		if prev.fingerprint != fp {
			return banklayer.PayResult{}, banklayer.ErrConflict
		}
		return banklayer.PayResult{ID: req.ID, Status: prev.status}, nil
	}
	p := &payment{fingerprint: fp, req: req, partner: partner, status: banklayer.StatusAccepted}
	if !accountNumber.MatchString(req.AccountNumber) {
		// The bank refuses bad beneficiary details at once. That is a rejection, not an error.
		p.status = banklayer.StatusRejected
	}
	b.payments[req.ID] = p
	return banklayer.PayResult{ID: req.ID, Status: p.status}, nil
}

// advance moves a payment one step per status read: accepted, then pending, then settled. Settling books the
// debit on the pool the payment was sent from. Settled and rejected are terminal and never book twice.
func (b *Bank) advance(id string) (banklayer.PayResult, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	p, ok := b.payments[id]
	if !ok {
		return banklayer.PayResult{}, banklayer.ErrNotFound
	}
	if banklayer.Terminal(p.status) {
		return banklayer.PayResult{ID: id, Status: p.status}, nil
	}
	p.polls++
	if p.polls == 1 {
		p.status = banklayer.StatusPending
		return banklayer.PayResult{ID: id, Status: p.status}, nil
	}
	p.status = banklayer.StatusSettled
	pl := b.poolLocked(p.req.Asset, p.partner)
	pl.moves = append(pl.moves, banklayer.Movement{
		ID:          fmt.Sprintf("%s-OUT-%s", p.req.Asset, id),
		Direction:   "OUT",
		AmountMinor: p.req.AmountMinor,
		Asset:       p.req.Asset,
		Reference:   p.req.Reference,
		BookedAt:    b.now(),
	})
	return banklayer.PayResult{ID: id, Status: p.status}, nil
}

// statement computes one UTC day. Opening is the balance from everything booked before that day.
func (b *Bank) statement(asset string, date time.Time, partner bool) banklayer.Statement {
	b.mu.Lock()
	defer b.mu.Unlock()
	day := date.UTC().Truncate(24 * time.Hour)
	next := day.Add(24 * time.Hour)
	st := banklayer.Statement{Asset: asset, Date: day, Movements: []banklayer.Movement{}}
	for _, m := range b.poolLocked(asset, partner).moves {
		if m.BookedAt.Before(day) {
			st.OpeningMinor += signed(m)
		} else if m.BookedAt.Before(next) {
			st.Movements = append(st.Movements, m)
		}
	}
	st.ClosingMinor = st.OpeningMinor
	for _, m := range st.Movements {
		st.ClosingMinor += signed(m)
	}
	return st
}

// inbound lists credits booked at or after since, oldest first.
func (b *Bank) inbound(asset string, since time.Time, partner bool) []banklayer.Movement {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := []banklayer.Movement{}
	for _, m := range b.poolLocked(asset, partner).moves {
		if m.Direction == "IN" && !m.BookedAt.Before(since) {
			out = append(out, m)
		}
	}
	return out
}

func signed(m banklayer.Movement) int64 {
	if m.Direction == "OUT" {
		return -m.AmountMinor
	}
	return m.AmountMinor
}

type sponsorRail struct{ b *Bank }

func (r sponsorRail) Pay(_ context.Context, req banklayer.PayRequest) (banklayer.PayResult, error) {
	return r.b.pay(req, false)
}

func (r sponsorRail) Status(_ context.Context, id string) (banklayer.PayResult, error) {
	return r.b.advance(id)
}

type partnerUSD struct{ b *Bank }

func (partnerUSD) Partner() string { return "sim-partner-usd" }
func (partnerUSD) Asset() string   { return "USD" }

func (p partnerUSD) OpenAccount(_ context.Context, req banklayer.OpenAccountRequest) (banklayer.Account, error) {
	if req.Owner == "" {
		return banklayer.Account{}, fmt.Errorf("%w: owner is required", banklayer.ErrInvalid)
	}
	req.Asset = "USD"
	p.b.mu.Lock()
	defer p.b.mu.Unlock()
	p.b.seq++
	id := fmt.Sprintf("PB%08d", p.b.seq)
	acc := &banklayer.Account{
		ID: id, Owner: req.Owner, Asset: "USD", Reference: "FORGEUSD" + id, Status: "OPEN", OpenedAt: p.b.now(),
	}
	p.b.accounts[id] = acc
	p.b.poolLocked("USD", true)
	return *acc, nil
}

func (p partnerUSD) Payout(_ context.Context, req banklayer.PayRequest) (banklayer.PayResult, error) {
	if req.Asset != "USD" {
		return banklayer.PayResult{}, fmt.Errorf("%w: partner handles USD only", banklayer.ErrInvalid)
	}
	return p.b.pay(req, true)
}

func (p partnerUSD) PayoutStatus(_ context.Context, id string) (banklayer.PayResult, error) {
	return p.b.advance(id)
}

func (p partnerUSD) Statement(_ context.Context, date time.Time) (banklayer.Statement, error) {
	return p.b.statement("USD", date, true), nil
}

func (p partnerUSD) Inbound(_ context.Context, since time.Time) ([]banklayer.Movement, error) {
	return p.b.inbound("USD", since, true), nil
}

type sponsorStatements struct{ b *Bank }

func (s sponsorStatements) Statement(_ context.Context, asset string, date time.Time) (banklayer.Statement, error) {
	if asset == "" {
		return banklayer.Statement{}, fmt.Errorf("%w: asset is required", banklayer.ErrInvalid)
	}
	return s.b.statement(asset, date, false), nil
}

func (s sponsorStatements) Inbound(_ context.Context, asset string, since time.Time) ([]banklayer.Movement, error) {
	if asset == "" {
		return nil, fmt.Errorf("%w: asset is required", banklayer.ErrInvalid)
	}
	return s.b.inbound(asset, since, false), nil
}

type sponsorInsurance struct{}

func (sponsorInsurance) Duty() string { return "sponsor" }

func (sponsorInsurance) Report(_ context.Context, _ time.Time, asset string, insuredMinor int64) error {
	if asset == "" || insuredMinor < 0 {
		return fmt.Errorf("%w: asset and a non-negative amount are required", banklayer.ErrInvalid)
	}
	return nil
}

type noPrudential struct{}

func (noPrudential) Required() bool { return false }

func (noPrudential) Submit(context.Context, string, []byte) error {
	return fmt.Errorf("%w: prudential returns are not required on the sponsor route", banklayer.ErrNotSupported)
}
