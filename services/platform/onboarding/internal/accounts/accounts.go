// Package accounts opens further current accounts for an active customer, one per currency: the rand, the dollar, the
// euro and the pound. Each step is safe to repeat. The ledger account is keyed by party and currency; a foreign
// account at the partner bank is opened once, under a row lock, and remembered; the payer's reference is registered
// with payments so that money sent to it is booked to the account.
package accounts

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	ledgerv1 "github.com/khayaai/forge-mutual/pkg/api/forge/ledger/v1"
	partnerv1 "github.com/khayaai/forge-mutual/pkg/api/forge/partner/v1"
	partyv1 "github.com/khayaai/forge-mutual/pkg/api/forge/party/v1"
	paymentsv1 "github.com/khayaai/forge-mutual/pkg/api/forge/payments/v1"
	"github.com/khayaai/forge-mutual/pkg/licence"
	"github.com/khayaai/forge-mutual/pkg/uuid"
)

// The calls this package makes, one interface per service, so tests can stand in for each.
type (
	Parties interface {
		GetParty(context.Context, *connect.Request[partyv1.GetPartyRequest]) (*connect.Response[partyv1.GetPartyResponse], error)
	}
	Ledger interface {
		CreateAccounts(context.Context, *connect.Request[ledgerv1.CreateAccountsRequest]) (*connect.Response[ledgerv1.CreateAccountsResponse], error)
	}
	Partners interface {
		OpenAccount(context.Context, *connect.Request[partnerv1.OpenAccountRequest]) (*connect.Response[partnerv1.OpenAccountResponse], error)
	}
	References interface {
		RegisterInboundReference(context.Context, *connect.Request[paymentsv1.RegisterInboundReferenceRequest]) (*connect.Response[paymentsv1.RegisterInboundReferenceResponse], error)
	}
)

// Errors a caller can act on.
var (
	ErrInvalid       = errors.New("accounts: party and a currency of ZAR, USD, EUR or GBP are required")
	ErrNotActive     = errors.New("accounts: only an active customer can open an account")
	ErrLedgerRefused = errors.New("accounts: the ledger refused the account")
)

const accountNamespace = "6c0b2f4e-1a9d-5e37-8b42-0f3d7a6e9c15"

// Opener opens currency accounts.
type Opener struct {
	Pool       *pgxpool.Pool
	Parties    Parties
	Ledger     Ledger
	Partners   Partners // nil: foreign currencies are refused
	References References
	Licence    licence.Gate
	Entity     string // "BANK"
	Insured    bool   // the deposit-insurance disclosure for rand accounts (INSURED_DEPOSITS)
}

// Account is what the customer is told about an opened account.
type Account struct {
	AccountID string
	Asset     string
	Reference string
	Partner   string
}

// Open opens the party's account in a currency, or returns the one it already has.
func (o *Opener) Open(ctx context.Context, party, asset, actor string) (Account, error) {
	asset = strings.ToUpper(strings.TrimSpace(asset))
	if party == "" || actor == "" || !(asset == "ZAR" || asset == "USD" || asset == "EUR" || asset == "GBP") {
		return Account{}, ErrInvalid
	}
	if err := o.Licence.AllowCurrency(asset); err != nil {
		return Account{}, err
	}
	if asset != "ZAR" && o.Partners == nil {
		return Account{}, fmt.Errorf("%w: no partner bank is configured for %s", licence.ErrNotApproved, asset)
	}
	p, err := o.Parties.GetParty(ctx, connect.NewRequest(&partyv1.GetPartyRequest{PartyId: party}))
	if err != nil {
		return Account{}, fmt.Errorf("accounts: party: %w", err)
	}
	if p.Msg.Party.GetStatus() != partyv1.PartyStatus_PARTY_STATUS_ACTIVE {
		return Account{}, ErrNotActive
	}

	var acc Account
	err = pgx.BeginFunc(ctx, o.Pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `insert into currency_account (party_id, asset) values ($1, $2) on conflict do nothing`, party, asset); err != nil {
			return err
		}
		var accountID, partner, reference string
		if err := tx.QueryRow(ctx, `select account_id, partner, reference from currency_account
			where party_id = $1 and asset = $2 for update`, party, asset).Scan(&accountID, &partner, &reference); err != nil {
			return err
		}
		if accountID == "" {
			id, err := o.ledgerAccount(ctx, party, asset, actor)
			if err != nil {
				return err
			}
			accountID = id
		}
		if reference == "" {
			if asset == "ZAR" {
				// The rand account is paid into at the sponsor's pooled account; the reference names the account.
				reference = "FORGE" + strings.ToUpper(strings.ReplaceAll(accountID, "-", ""))[:12]
			} else {
				// A foreign account is opened at the partner bank once, while this row is locked.
				res, err := o.Partners.OpenAccount(ctx, connect.NewRequest(&partnerv1.OpenAccountRequest{Owner: party, Asset: asset}))
				if err != nil {
					return fmt.Errorf("accounts: partner: %w", err)
				}
				partner, reference = res.Msg.Partner, res.Msg.Reference
				if reference == "" {
					return errors.New("accounts: the partner returned no reference")
				}
			}
		}
		if _, err := tx.Exec(ctx, `update currency_account set account_id = $3, partner = $4, reference = $5
			where party_id = $1 and asset = $2`, party, asset, accountID, partner, reference); err != nil {
			return err
		}
		acc = Account{AccountID: accountID, Asset: asset, Reference: reference, Partner: partner}
		return nil
	})
	if err != nil {
		return Account{}, err
	}
	// Registering is repeatable, so it runs after the commit: a crash here is finished by the next request.
	if _, err := o.References.RegisterInboundReference(ctx, connect.NewRequest(&paymentsv1.RegisterInboundReferenceRequest{
		Reference: acc.Reference, Asset: asset, AccountId: acc.AccountID, PartyId: party,
	})); err != nil {
		return Account{}, fmt.Errorf("accounts: register reference: %w", err)
	}
	if _, err := o.Pool.Exec(ctx, `update currency_account set registered = true where party_id = $1 and asset = $2`, party, asset); err != nil {
		return Account{}, err
	}
	return acc, nil
}

// ledgerAccount creates the customer deposit account. The key is the party and currency, so a repeat is a replay.
func (o *Opener) ledgerAccount(ctx context.Context, party, asset, actor string) (string, error) {
	key := "currency-account:" + party + ":" + asset
	txn, err := uuid.NewV5(accountNamespace, key)
	if err != nil {
		return "", err
	}
	res, err := o.Ledger.CreateAccounts(ctx, connect.NewRequest(&ledgerv1.CreateAccountsRequest{
		IdempotencyKey: key, BusinessTxnId: txn, EntityCode: o.Entity, Actor: actor, Reason: "open " + asset + " account",
		Accounts: []*ledgerv1.AccountSpec{{
			OwnerPartyId: party, Asset: asset, Code: 1001, ProductId: strings.ToLower(asset) + "-current",
			DebitsMustNotExceedCredits: true,
			// Only rand deposits at the sponsor can be covered by deposit insurance; a balance held at a foreign
			// partner bank is not.
			Insured: asset == "ZAR" && o.Insured,
		}},
	}))
	if err != nil {
		return "", fmt.Errorf("accounts: ledger: %w", err)
	}
	if (res.Msg.Status != ledgerv1.Status_STATUS_OK && res.Msg.Status != ledgerv1.Status_STATUS_REPLAYED) || len(res.Msg.AccountIds) != 1 {
		return "", fmt.Errorf("%w: %v", ErrLedgerRefused, res.Msg.RejectionCodes)
	}
	return res.Msg.AccountIds[0], nil
}
