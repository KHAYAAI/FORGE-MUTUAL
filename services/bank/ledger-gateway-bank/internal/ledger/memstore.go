package ledger

import (
	"context"
	"math"
	"sync"
	"time"
)

// MemStore is an in-process Store with TigerBeetle semantics. It is the reference
// model for the rules: the TigerBeetle adapter must agree with it on every test.
// It is not durable.
type MemStore struct {
	mu        sync.Mutex
	now       func() time.Time
	clock     uint64
	accounts  map[ID]Account
	transfers map[ID]Transfer
	pending   map[ID]*pendingState
	// failed holds the IDs of transfers refused for a transient reason. They are refused for good (Reason.Transient).
	failed map[ID]struct{}
}

type pendingState struct {
	state     pendingPhase
	expiresAt time.Time // zero means no timeout
}

type pendingPhase uint8

const (
	phaseOpen pendingPhase = iota
	phasePosted
	phaseVoided
	phaseExpired
)

// NewMemStore returns an empty store. now is the clock used for pending timeouts; nil means time.Now.
func NewMemStore(now func() time.Time) *MemStore {
	if now == nil {
		now = time.Now
	}
	return &MemStore{
		now:       now,
		accounts:  make(map[ID]Account),
		transfers: make(map[ID]Transfer),
		pending:   make(map[ID]*pendingState),
		failed:    make(map[ID]struct{}),
	}
}

// CreateAccounts creates accounts. Linked accounts form chains that commit or roll back together.
// Balances must be zero on creation.
func (s *MemStore) CreateAccounts(_ context.Context, batch []Account) ([]Outcome, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sweep(s.now())

	out := make([]Outcome, len(batch))
	for i := 0; i < len(batch); {
		end := i
		for end < len(batch) && batch[end].Linked {
			end++
		}
		if end == len(batch) {
			for k := i; k < len(batch); k++ {
				out[k] = Outcome{Index: k, Reason: ReasonInvalidAccount}
			}
			break
		}
		s.createChain(batch[i:end+1], out[i:end+1], i)
		i = end + 1
	}
	return out, nil
}

// createChain validates the whole chain before inserting anything. Any answer but OK breaks the chain, and that
// includes "exists": TigerBeetle answers a chain that is sent a second time with "exists" for its first member and
// "linked_event_failed" for the rest. A chain of one is simply its own answer.
func (s *MemStore) createChain(chain []Account, out []Outcome, base int) {
	staged := make(map[ID]Account, len(chain))
	failAt := -1
	var failReason Reason
	for k, a := range chain {
		if r := s.checkAccount(a, staged); r != ReasonOK {
			failAt, failReason = k, r
			break
		}
		staged[a.ID] = a
	}
	if failAt >= 0 {
		for k := range chain {
			reason := ReasonLinkedEventFailed
			if k == failAt {
				reason = failReason
			}
			out[k] = Outcome{Index: base + k, Reason: reason}
		}
		return
	}
	for k, a := range chain {
		a.Timestamp = s.nextTimestamp()
		s.accounts[a.ID] = a
		out[k] = Outcome{Index: base + k, Reason: ReasonOK}
	}
}

// checkAccount reports whether a can be created, without changing state. staged holds the accounts earlier in the
// same chain.
func (s *MemStore) checkAccount(a Account, staged map[ID]Account) Reason {
	if a.ID.IsZero() || a.Ledger == 0 || a.Code == 0 {
		return ReasonInvalidAccount
	}
	if a.DebitsPending|a.DebitsPosted|a.CreditsPending|a.CreditsPosted != 0 {
		return ReasonInvalidAccount
	}
	existing, ok := staged[a.ID]
	if !ok {
		existing, ok = s.accounts[a.ID]
	}
	if ok {
		if sameAccountFields(existing, a) {
			return ReasonExists
		}
		return ReasonExistsWithDifferentFields
	}
	return ReasonOK
}

// CreateTransfers applies a batch. Linked events form chains that commit or roll back together.
func (s *MemStore) CreateTransfers(_ context.Context, batch []Transfer) ([]Outcome, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sweep(s.now())

	out := make([]Outcome, len(batch))
	for i := 0; i < len(batch); {
		end := i
		for end < len(batch) && batch[end].Linked {
			end++
		}
		if end == len(batch) {
			// Chain left open at the end of the batch: nothing in it applies.
			for k := i; k < len(batch); k++ {
				out[k] = Outcome{Index: k, Reason: ReasonInvalidTransfer}
			}
			break
		}
		s.applyChain(batch[i:end+1], out[i:end+1], i)
		i = end + 1
	}
	return out, nil
}

// applyChain applies one chain, or nothing. Any answer but OK breaks the chain, and that includes "exists": when a
// chain that was already applied is sent again, TigerBeetle answers "exists" for its first member and
// "linked_event_failed" for the rest, and changes nothing. A caller that retries must read that as "already done",
// and confirm it by looking the transfers up (the gateway does both). A chain of one is simply its own answer.
func (s *MemStore) applyChain(chain []Transfer, out []Outcome, base int) {
	st := newStaging(s)
	failAt := -1
	var failReason Reason
	for k, t := range chain {
		r := st.apply(t)
		if r != ReasonOK {
			failAt, failReason = k, r
			break
		}
		out[k] = Outcome{Index: base + k, Reason: r}
	}
	if failAt >= 0 {
		for k := range chain {
			reason := ReasonLinkedEventFailed
			if k == failAt {
				reason = failReason
			}
			out[k] = Outcome{Index: base + k, Reason: reason}
		}
		// The chain is rolled back, but the ID that was refused for a transient reason stays refused. The other
		// members of the chain were not refused on their own account, and their IDs can be used again.
		if failReason.Transient() {
			s.failed[chain[failAt].ID] = struct{}{}
		}
		return
	}
	st.commit()
}

// LookupAccounts implements Store.
func (s *MemStore) LookupAccounts(_ context.Context, ids []ID) ([]Account, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sweep(s.now())
	var out []Account
	for _, id := range ids {
		if a, ok := s.accounts[id]; ok {
			out = append(out, a)
		}
	}
	return out, nil
}

// LookupTransfers implements Store.
func (s *MemStore) LookupTransfers(_ context.Context, ids []ID) ([]Transfer, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []Transfer
	for _, id := range ids {
		if t, ok := s.transfers[id]; ok {
			out = append(out, t)
		}
	}
	return out, nil
}

// sweep releases the reservations of open pending transfers whose timeout has passed.
// Callers hold s.mu.
func (s *MemStore) sweep(now time.Time) {
	for id, p := range s.pending {
		if p.state != phaseOpen || p.expiresAt.IsZero() || now.Before(p.expiresAt) {
			continue
		}
		t := s.transfers[id]
		debit := s.accounts[t.DebitAccountID]
		credit := s.accounts[t.CreditAccountID]
		debit.DebitsPending -= t.Amount
		credit.CreditsPending -= t.Amount
		s.accounts[t.DebitAccountID] = debit
		s.accounts[t.CreditAccountID] = credit
		p.state = phaseExpired
	}
}

func (s *MemStore) nextTimestamp() uint64 {
	s.clock++
	return s.clock
}

// staging holds the working copy of one linked chain. Nothing reaches the store
// until every event in the chain has succeeded.
type staging struct {
	s        *MemStore
	accounts map[ID]Account
	transfer map[ID]Transfer
	pending  map[ID]pendingState
	clock    uint64
}

func newStaging(s *MemStore) *staging {
	return &staging{
		s:        s,
		accounts: make(map[ID]Account),
		transfer: make(map[ID]Transfer),
		pending:  make(map[ID]pendingState),
		clock:    s.clock,
	}
}

func (st *staging) account(id ID) (Account, bool) {
	if a, ok := st.accounts[id]; ok {
		return a, true
	}
	a, ok := st.s.accounts[id]
	return a, ok
}

func (st *staging) lookupTransfer(id ID) (Transfer, bool) {
	if t, ok := st.transfer[id]; ok {
		return t, true
	}
	t, ok := st.s.transfers[id]
	return t, ok
}

func (st *staging) pendingPhase(id ID) (pendingState, bool) {
	if p, ok := st.pending[id]; ok {
		return p, true
	}
	if p, ok := st.s.pending[id]; ok {
		return *p, true
	}
	return pendingState{}, false
}

func (st *staging) nextTimestamp() uint64 {
	st.clock++
	return st.clock
}

func (st *staging) apply(t Transfer) Reason {
	if t.ID.IsZero() || t.Ledger == 0 || t.Code == 0 {
		return ReasonInvalidTransfer
	}
	if t.Amount == 0 || t.Amount > math.MaxInt64 {
		return ReasonInvalidAmount
	}
	if existing, ok := st.lookupTransfer(t.ID); ok {
		if (t.PostPending || t.VoidPending) && t.PendingID == existing.PendingID {
			// A post or void is stored with the pending transfer's user data where its own was empty, so the same
			// request sent again must be compared after the same filling in.
			if pt, ok := st.lookupTransfer(t.PendingID); ok {
				t = inheritUserData(t, pt)
			}
		}
		if sameTransferFields(existing, t) {
			return ReasonExists
		}
		return ReasonExistsWithDifferentFields
	}
	if _, ok := st.s.failed[t.ID]; ok {
		return ReasonIDAlreadyFailed
	}
	if t.PostPending && t.VoidPending {
		return ReasonInvalidTransfer
	}
	if t.Pending && (t.PostPending || t.VoidPending) {
		return ReasonInvalidTransfer
	}
	if t.PostPending || t.VoidPending {
		return st.resolvePending(t)
	}
	return st.createTransfer(t)
}

func (st *staging) createTransfer(t Transfer) Reason {
	if t.DebitAccountID == t.CreditAccountID {
		return ReasonSameAccount
	}
	debit, ok := st.account(t.DebitAccountID)
	if !ok {
		return ReasonAccountNotFound
	}
	credit, ok := st.account(t.CreditAccountID)
	if !ok {
		return ReasonAccountNotFound
	}
	if debit.Ledger != t.Ledger || credit.Ledger != t.Ledger {
		return ReasonLedgerMismatch
	}

	amt := t.Amount
	if debit.DebitsMustNotExceedCredits {
		total, ok := addU64(debit.DebitsPending, debit.DebitsPosted)
		if !ok {
			return ReasonOverflow
		}
		total, ok = addU64(total, amt)
		if !ok {
			return ReasonOverflow
		}
		if total > debit.CreditsPosted {
			return ReasonInsufficientFunds
		}
	}

	if t.Pending {
		var ok1, ok2 bool
		debit.DebitsPending, ok1 = addU64(debit.DebitsPending, amt)
		credit.CreditsPending, ok2 = addU64(credit.CreditsPending, amt)
		if !ok1 || !ok2 {
			return ReasonOverflow
		}
		var expires time.Time
		if t.Timeout > 0 {
			expires = st.s.now().Add(time.Duration(t.Timeout) * time.Second)
		}
		st.pending[t.ID] = pendingState{state: phaseOpen, expiresAt: expires}
	} else {
		var ok1, ok2 bool
		debit.DebitsPosted, ok1 = addU64(debit.DebitsPosted, amt)
		credit.CreditsPosted, ok2 = addU64(credit.CreditsPosted, amt)
		if !ok1 || !ok2 {
			return ReasonOverflow
		}
	}
	st.accounts[t.DebitAccountID] = debit
	st.accounts[t.CreditAccountID] = credit

	t.Timestamp = st.nextTimestamp()
	st.transfer[t.ID] = t
	return ReasonOK
}

func (st *staging) resolvePending(t Transfer) Reason {
	pt, ok := st.lookupTransfer(t.PendingID)
	if !ok {
		return ReasonPendingNotFound
	}
	if !pt.Pending {
		return ReasonPendingNotPending
	}
	p, _ := st.pendingPhase(t.PendingID)
	switch p.state {
	case phasePosted:
		return ReasonPendingAlreadyPosted
	case phaseVoided:
		return ReasonPendingAlreadyVoided
	case phaseExpired:
		return ReasonPendingExpired
	}
	if !p.expiresAt.IsZero() && !st.s.now().Before(p.expiresAt) {
		return ReasonPendingExpired
	}
	if t.DebitAccountID != pt.DebitAccountID || t.CreditAccountID != pt.CreditAccountID ||
		t.Amount != pt.Amount || t.Ledger != pt.Ledger {
		return ReasonPendingMismatch
	}

	debit, ok := st.account(pt.DebitAccountID)
	if !ok {
		return ReasonAccountNotFound
	}
	credit, ok := st.account(pt.CreditAccountID)
	if !ok {
		return ReasonAccountNotFound
	}
	debit.DebitsPending -= pt.Amount
	credit.CreditsPending -= pt.Amount

	if t.PostPending {
		var ok1, ok2 bool
		debit.DebitsPosted, ok1 = addU64(debit.DebitsPosted, pt.Amount)
		credit.CreditsPosted, ok2 = addU64(credit.CreditsPosted, pt.Amount)
		if !ok1 || !ok2 {
			return ReasonOverflow
		}
		p.state = phasePosted
	} else {
		p.state = phaseVoided
	}
	st.accounts[pt.DebitAccountID] = debit
	st.accounts[pt.CreditAccountID] = credit
	st.pending[t.PendingID] = p

	t = inheritUserData(t, pt)
	t.Timestamp = st.nextTimestamp()
	st.transfer[t.ID] = t
	return ReasonOK
}

func (st *staging) commit() {
	for id, a := range st.accounts {
		st.s.accounts[id] = a
	}
	for id, t := range st.transfer {
		st.s.transfers[id] = t
	}
	for id, p := range st.pending {
		p := p
		if existing, ok := st.s.pending[id]; ok {
			*existing = p
		} else {
			st.s.pending[id] = &p
		}
	}
	st.s.clock = st.clock
}

// inheritUserData fills a post's or void's empty user data from its pending transfer, as TigerBeetle stores it.
func inheritUserData(t, pending Transfer) Transfer {
	if t.UserData128 == ([16]byte{}) {
		t.UserData128 = pending.UserData128
	}
	if t.UserData64 == 0 {
		t.UserData64 = pending.UserData64
	}
	if t.UserData32 == 0 {
		t.UserData32 = pending.UserData32
	}
	return t
}

func addU64(a, b uint64) (uint64, bool) {
	if a > math.MaxUint64-b {
		return 0, false
	}
	return a + b, true
}

func sameAccountFields(a, b Account) bool {
	return a.Ledger == b.Ledger && a.Code == b.Code &&
		a.DebitsMustNotExceedCredits == b.DebitsMustNotExceedCredits &&
		a.UserData128 == b.UserData128 && a.UserData64 == b.UserData64 && a.UserData32 == b.UserData32
}

func sameTransferFields(a, b Transfer) bool {
	return a.DebitAccountID == b.DebitAccountID && a.CreditAccountID == b.CreditAccountID &&
		a.Amount == b.Amount && a.PendingID == b.PendingID &&
		a.UserData128 == b.UserData128 && a.UserData64 == b.UserData64 && a.UserData32 == b.UserData32 &&
		a.Timeout == b.Timeout && a.Ledger == b.Ledger && a.Code == b.Code &&
		a.Pending == b.Pending && a.PostPending == b.PostPending && a.VoidPending == b.VoidPending
}
