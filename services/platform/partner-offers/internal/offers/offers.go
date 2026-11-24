// Package offers introduces customers to investment and crypto-asset products that a licensed third party provides.
//
// The rule this package exists to enforce: every product is provided, licensed and monitored by a third party, never by
// Forge Mutual. Forge does not provide it, advise on it, hold the customer's money or assets in it, or guarantee it. The
// statement that says so is built here from the provider's verified licence, so nobody can type a softer one; it goes with
// every offer, and a customer must acknowledge it, by its hash, before a referral is recorded. A product whose provider's
// licence has not been verified in the last twelve months is not shown at all.
package offers

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"regexp"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	offersv1 "github.com/khayaai/forge-mutual/pkg/api/forge/offers/v1"
	"github.com/khayaai/forge-mutual/pkg/licence"
)

//go:embed schema.sql
var schema string

// Open applies the schema.
func Open(ctx context.Context, pool *pgxpool.Pool) error {
	return pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `select pg_advisory_xact_lock(7771)`); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, schema); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `alter table offer add column if not exists updated_by text not null default ''`)
		return err
	})
}

// Consents says whether a party has agreed to share details with a provider.
type Consents interface {
	Allowed(ctx context.Context, party, purpose, scope string) (consentID string, ok bool, err error)
}

// Parties reports whether a party may act. Only ACTIVE parties are introduced to anyone.
type Parties interface {
	Active(ctx context.Context, party string) (bool, error)
}

// LicenceMaxAge is how old a licence check may be before the provider's offers disappear. A licence can be suspended any
// day, so a check older than a year proves little (PROPOSED; compliance may choose a shorter time).
const LicenceMaxAge = 365 * 24 * time.Hour

// The consent a referral needs. The consent service accepts only lower-case letters, digits, dots and underscores in a purpose, so the
// provider goes in the scope: purpose "share_with_partner", scope "referral:<provider_id>".
const ConsentPurpose = "share_with_partner"

// ConsentScope is the scope of the consent to share details with one provider.
func ConsentScope(providerID string) string { return "referral:" + providerID }

// Categories, and the licence each needs.
var categoryLicence = map[string]string{"INVESTMENTS": "FSP", "CRYPTO_ASSETS": "CASP"}

// categoryWarning is the risk statement that must accompany a category. The wording is PROPOSED and must be confirmed by the
// lawyer and the provider's own required disclosures (VERIFY).
var categoryWarning = map[string]string{
	"INVESTMENTS":   "Investments can go down as well as up, and you can get back less than you put in. Past performance does not predict future returns. Read the provider's documents before you decide.",
	"CRYPTO_ASSETS": "Crypto assets, including stablecoins, are volatile and can lose all of their value. They are not deposits, are not covered by deposit insurance, and may not be recoverable if lost or stolen or if the provider fails. Only use money you can afford to lose.",
}

var licenceKind = map[string]string{"FSP": "financial services provider", "CASP": "crypto asset service provider"}

var (
	idPattern = regexp.MustCompile(`^[a-z][a-z0-9-]{1,40}$`)

	ErrDisabled = errors.New("offers: third-party offers are switched off")
)

// Service is the third-party offers service.
type Service struct {
	Pool     *pgxpool.Pool
	Consents Consents
	Parties  Parties
	Licence  licence.Gate
	// Enabled is false until someone deliberately turns the whole layer on (PARTNER_OFFERS_ENABLED). Off, every call is refused.
	Enabled bool
	Log     *slog.Logger
	Now     func() time.Time
}

func (s *Service) now() time.Time {
	if s.Now != nil {
		return s.Now().UTC()
	}
	return time.Now().UTC()
}

func (s *Service) allowed() error {
	if !s.Enabled {
		return connect.NewError(connect.CodeFailedPrecondition, ErrDisabled)
	}
	if err := s.Licence.Allow(licence.PartnerOffers); err != nil {
		return connect.NewError(connect.CodeFailedPrecondition, err)
	}
	return nil
}

func invalid(msg string) error { return connect.NewError(connect.CodeInvalidArgument, errors.New(msg)) }

func wrap(err error) error {
	var ce *connect.Error
	if err == nil || errors.As(err, &ce) {
		return err
	}
	return connect.NewError(connect.CodeInternal, fmt.Errorf("offers: %w", err))
}

// provider and offer as stored.
type provider struct {
	ID, Name, LicenceType, Regulator, Number, SourceURL, Status, RegisteredBy string
	VerifiedAt, ExpiresAt                                                     *time.Time
	VerifiedBy                                                                string
}

type offerRow struct {
	ID, ProviderID, Category, Title, Summary, URL, Fee, Status, UpdatedBy string
}

// Statement builds the mandatory third-party statement. It is the same words for every product, with only the provider's
// verified facts filled in, so no one can write a softer one.
func Statement(p provider, o offerRow) string {
	var b strings.Builder
	fmt.Fprintf(&b, "THIRD-PARTY PRODUCT: provided, licensed and monitored by %s, not by Forge Mutual.\n\n", p.Name)
	fmt.Fprintf(&b, "%s is a product of %s, which is licensed by the %s as a %s (licence number %s). %s is responsible for this product and for monitoring it. ",
		o.Title, p.Name, p.Regulator, licenceKind[p.LicenceType], p.Number, p.Name)
	b.WriteString("Forge Mutual does not provide this product, advise you on it, hold your money or assets in it, or guarantee it. ")
	b.WriteString("It is not a Forge Mutual product, it is not a deposit with Forge Mutual, and it is not covered by any deposit insurance through us.\n\n")
	b.WriteString(categoryWarning[o.Category] + "\n\n")
	fmt.Fprintf(&b, "You deal directly with %s, under its terms, and it will check who you are itself. Complaints about the product go to %s. Complaints about how Forge introduced it go to Forge.\n\n", p.Name, p.Name)
	fmt.Fprintf(&b, "Fees: %s", o.Fee)
	return b.String()
}

// StatementHash identifies the exact text a customer acknowledged.
func StatementHash(statement string) string {
	h := sha256.Sum256([]byte(statement))
	return hex.EncodeToString(h[:])
}

// Visible reports whether an offer may be shown now: the offer is active, the provider is active, and its licence was
// verified within LicenceMaxAge and has not expired.
func Visible(now time.Time, providerStatus string, verifiedAt, expiresAt *time.Time, offerStatus string) bool {
	if offerStatus != "ACTIVE" || providerStatus != "ACTIVE" || verifiedAt == nil || now.Sub(*verifiedAt) > LicenceMaxAge {
		return false
	}
	return expiresAt == nil || now.Before(*expiresAt)
}

// httpsURL accepts only an https address on a named host: no credentials, no IP address, no localhost.
func httpsURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil {
		return errors.New("must be an https address on the provider's own domain")
	}
	h := strings.ToLower(u.Hostname())
	if h == "localhost" || net.ParseIP(h) != nil || !strings.Contains(h, ".") {
		return errors.New("must be an https address on the provider's own domain")
	}
	return nil
}

func tsPtr(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

func (p provider) proto() *offersv1.Provider {
	return &offersv1.Provider{ProviderId: p.ID, Name: p.Name, LicenceType: p.LicenceType, Regulator: p.Regulator, LicenceNumber: p.Number,
		Status: p.Status, LicenceVerifiedAt: tsPtr(p.VerifiedAt), LicenceVerifiedBy: p.VerifiedBy, LicenceExpiresAt: tsPtr(p.ExpiresAt)}
}

func (s *Service) loadProvider(ctx context.Context, q interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, id string) (provider, error) {
	var p provider
	var by *string
	err := q.QueryRow(ctx, `select provider_id, name, licence_type, regulator, licence_number, licence_source_url, status, registered_by,
		licence_verified_at, licence_verified_by, licence_expires_at from provider where provider_id = $1`, id).
		Scan(&p.ID, &p.Name, &p.LicenceType, &p.Regulator, &p.Number, &p.SourceURL, &p.Status, &p.RegisteredBy, &p.VerifiedAt, &by, &p.ExpiresAt)
	if by != nil {
		p.VerifiedBy = *by
	}
	return p, err
}

func (s *Service) event(ctx context.Context, q interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}, id, actor, action, detail string) error {
	_, err := q.Exec(ctx, `insert into provider_event (provider_id, at, actor, action, detail) values ($1,$2,$3,$4,$5)`, id, s.now(), actor, action, detail)
	return err
}
