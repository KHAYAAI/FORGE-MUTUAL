package offers

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"

	offersv1 "github.com/khayaai/forge-mutual/pkg/api/forge/offers/v1"
)

func (s *Service) offerProto(p provider, o offerRow) *offersv1.Offer {
	st := Statement(p, o)
	return &offersv1.Offer{OfferId: o.ID, Category: o.Category, Title: o.Title, Summary: o.Summary, ProviderName: p.Name, LicenceType: p.LicenceType,
		LicenceNumber: p.Number, Regulator: p.Regulator, Statement: st, StatementHash: StatementHash(st), FeeDisclosure: o.Fee}
}

const offerSelect = `select o.offer_id, o.provider_id, o.category, o.title, o.summary, o.handoff_url, o.fee_disclosure, o.status, o.updated_by,
	p.provider_id, p.name, p.licence_type, p.regulator, p.licence_number, p.licence_source_url, p.status, p.registered_by,
	p.licence_verified_at, coalesce(p.licence_verified_by,''), p.licence_expires_at
	from offer o join provider p using (provider_id)`

func scanOffer(row pgx.Row) (offerRow, provider, error) {
	var o offerRow
	var p provider
	err := row.Scan(&o.ID, &o.ProviderID, &o.Category, &o.Title, &o.Summary, &o.URL, &o.Fee, &o.Status, &o.UpdatedBy,
		&p.ID, &p.Name, &p.LicenceType, &p.Regulator, &p.Number, &p.SourceURL, &p.Status, &p.RegisteredBy, &p.VerifiedAt, &p.VerifiedBy, &p.ExpiresAt)
	return o, p, err
}

// ListOffers returns what the party may see, each with its statement. Nothing whose licence is not currently verified.
func (s *Service) ListOffers(ctx context.Context, r *connect.Request[offersv1.ListOffersRequest]) (*connect.Response[offersv1.ListOffersResponse], error) {
	if err := s.allowed(); err != nil {
		return nil, err
	}
	if r.Msg.PartyId == "" {
		return nil, invalid("party_id is required")
	}
	if c := r.Msg.Category; c != "" {
		if _, ok := categoryLicence[c]; !ok {
			return nil, invalid("category must be INVESTMENTS or CRYPTO_ASSETS")
		}
	}
	rows, err := s.Pool.Query(ctx, offerSelect+` where ($1 = '' or o.category = $1) order by o.title`, r.Msg.Category)
	if err != nil {
		return nil, wrap(err)
	}
	defer rows.Close()
	out := &offersv1.ListOffersResponse{}
	now := s.now()
	for rows.Next() {
		o, p, err := scanOffer(rows)
		if err != nil {
			return nil, wrap(err)
		}
		if Visible(now, p.Status, p.VerifiedAt, p.ExpiresAt, o.Status) {
			out.Offers = append(out.Offers, s.offerProto(p, o))
		}
	}
	return connect.NewResponse(out), wrap(rows.Err())
}

func newID(prefix string) (string, error) {
	b := make([]byte, 10)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return prefix + hex.EncodeToString(b), nil
}

func (s *Service) referralProto(id, offer, provider, url string, at time.Time) *offersv1.Referral {
	return &offersv1.Referral{ReferralId: id, OfferId: offer, ProviderName: provider, HandoffUrl: url, CreatedAt: at.UTC().Format(time.RFC3339)}
}

func handoff(base, id string) string {
	sep := "?"
	if strings.Contains(base, "?") {
		sep = "&"
	}
	return base + sep + "ref=" + id // the referral id and nothing about the customer
}

// CreateReferral records an introduction. It needs: the offer still visible; the exact current statement acknowledged; the party
// active; and the party's consent to share their details with this provider. The customer then continues on the provider's own
// site. Forge takes no money, gives no advice, and keeps no record of what the customer does there.
func (s *Service) CreateReferral(ctx context.Context, r *connect.Request[offersv1.CreateReferralRequest]) (*connect.Response[offersv1.CreateReferralResponse], error) {
	if err := s.allowed(); err != nil {
		return nil, err
	}
	m := r.Msg
	if m.IdempotencyKey == "" || m.PartyId == "" || m.OfferId == "" {
		return nil, invalid("idempotency_key, party_id and offer_id are required")
	}
	// A retry returns the referral already made.
	var id, existingOffer string
	var at time.Time
	err := s.Pool.QueryRow(ctx, `select referral_id, offer_id, created_at from referral where party_id = $1 and idempotency_key = $2`, m.PartyId, m.IdempotencyKey).Scan(&id, &existingOffer, &at)
	if err == nil {
		if existingOffer != m.OfferId {
			return nil, connect.NewError(connect.CodeAlreadyExists, errors.New("this key was used for a different request"))
		}
		o, p, err := scanOffer(s.Pool.QueryRow(ctx, offerSelect+` where o.offer_id = $1`, existingOffer))
		if err != nil {
			return nil, wrap(err)
		}
		return connect.NewResponse(&offersv1.CreateReferralResponse{Referral: s.referralProto(id, existingOffer, p.Name, handoff(o.URL, id), at)}), nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, wrap(err)
	}
	o, p, err := scanOffer(s.Pool.QueryRow(ctx, offerSelect+` where o.offer_id = $1`, m.OfferId))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("no such offer"))
	}
	if err != nil {
		return nil, wrap(err)
	}
	if !Visible(s.now(), p.Status, p.VerifiedAt, p.ExpiresAt, o.Status) {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("no such offer")) // hidden offers look absent
	}
	st := StatementHash(Statement(p, o))
	if !m.AcknowledgedThirdParty || m.StatementHash != st {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("you must read and acknowledge the current third-party statement for this product before we can introduce you"))
	}
	if s.Parties != nil {
		ok, err := s.Parties.Active(ctx, m.PartyId)
		if err != nil {
			return nil, connect.NewError(connect.CodeUnavailable, err)
		}
		if !ok {
			return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("this action is not available on your account; please contact us"))
		}
	}
	consentID, ok, err := s.Consents.Allowed(ctx, m.PartyId, "share_with_partner:"+p.ID, "referral")
	if err != nil {
		return nil, connect.NewError(connect.CodeUnavailable, err)
	}
	if !ok {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("we need your consent to share your details with this provider before we introduce you"))
	}
	id, err = newID("ref-")
	if err != nil {
		return nil, wrap(err)
	}
	now := s.now()
	if _, err := s.Pool.Exec(ctx, `insert into referral (referral_id, idempotency_key, party_id, offer_id, statement_hash, consent_id, created_at) values ($1,$2,$3,$4,$5,$6,$7)
		on conflict (party_id, idempotency_key) do nothing`, id, m.IdempotencyKey, m.PartyId, m.OfferId, st, consentID, now); err != nil {
		return nil, wrap(err)
	}
	if err := s.Pool.QueryRow(ctx, `select referral_id, created_at from referral where party_id = $1 and idempotency_key = $2`, m.PartyId, m.IdempotencyKey).Scan(&id, &at); err != nil {
		return nil, wrap(err)
	}
	return connect.NewResponse(&offersv1.CreateReferralResponse{Referral: s.referralProto(id, o.ID, p.Name, handoff(o.URL, id), at)}), nil
}

func (s *Service) ListReferrals(ctx context.Context, r *connect.Request[offersv1.ListReferralsRequest]) (*connect.Response[offersv1.ListReferralsResponse], error) {
	if err := s.allowed(); err != nil {
		return nil, err
	}
	if r.Msg.PartyId == "" {
		return nil, invalid("party_id is required")
	}
	rows, err := s.Pool.Query(ctx, `select r.referral_id, r.offer_id, p.name, o.handoff_url, r.created_at from referral r join offer o using (offer_id) join provider p using (provider_id)
		where r.party_id = $1 order by r.created_at desc`, r.Msg.PartyId)
	if err != nil {
		return nil, wrap(err)
	}
	defer rows.Close()
	out := &offersv1.ListReferralsResponse{}
	for rows.Next() {
		var id, offer, name, url string
		var at time.Time
		if err := rows.Scan(&id, &offer, &name, &url, &at); err != nil {
			return nil, wrap(err)
		}
		out.Referrals = append(out.Referrals, s.referralProto(id, offer, name, handoff(url, id), at))
	}
	return connect.NewResponse(out), wrap(rows.Err())
}

// ---- compliance's side ----

func needActor(actor string) error {
	if strings.TrimSpace(actor) == "" {
		return invalid("actor is required")
	}
	return nil
}

func (s *Service) RegisterProvider(ctx context.Context, r *connect.Request[offersv1.RegisterProviderRequest]) (*connect.Response[offersv1.RegisterProviderResponse], error) {
	if err := s.allowed(); err != nil {
		return nil, err
	}
	m := r.Msg
	if err := needActor(m.Actor); err != nil {
		return nil, err
	}
	if !idPattern.MatchString(m.ProviderId) {
		return nil, invalid("provider_id must be lower-case letters, digits and hyphens")
	}
	if _, ok := licenceKind[m.LicenceType]; !ok {
		return nil, invalid("licence_type must be FSP or CASP")
	}
	if strings.TrimSpace(m.Name) == "" || strings.TrimSpace(m.Regulator) == "" || strings.TrimSpace(m.LicenceNumber) == "" {
		return nil, invalid("name, regulator and licence_number are required")
	}
	if err := httpsURL(m.LicenceSourceUrl); err != nil {
		return nil, invalid("licence_source_url " + err.Error())
	}
	err := pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `insert into provider (provider_id, name, licence_type, regulator, licence_number, licence_source_url, status, registered_by, created_at)
			values ($1,$2,$3,$4,$5,$6,'PENDING',$7,$8) on conflict (provider_id) do nothing`, m.ProviderId, strings.TrimSpace(m.Name), m.LicenceType,
			strings.TrimSpace(m.Regulator), strings.TrimSpace(m.LicenceNumber), m.LicenceSourceUrl, m.Actor, s.now())
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return connect.NewError(connect.CodeAlreadyExists, errors.New("this provider is already registered"))
		}
		return s.event(ctx, tx, m.ProviderId, m.Actor, "REGISTERED", m.LicenceType+" "+m.LicenceNumber)
	})
	if err != nil {
		return nil, wrap(err)
	}
	p, err := s.loadProvider(ctx, s.Pool, m.ProviderId)
	return connect.NewResponse(&offersv1.RegisterProviderResponse{Provider: p.proto()}), wrap(err)
}

// VerifyLicence: someone other than the registrant checked the regulator's own register. This is what makes a provider ACTIVE.
func (s *Service) VerifyLicence(ctx context.Context, r *connect.Request[offersv1.VerifyLicenceRequest]) (*connect.Response[offersv1.VerifyLicenceResponse], error) {
	if err := s.allowed(); err != nil {
		return nil, err
	}
	m := r.Msg
	if err := needActor(m.Actor); err != nil {
		return nil, err
	}
	var expires *time.Time
	if strings.TrimSpace(m.LicenceExpiresAt) != "" {
		t, err := time.Parse(time.RFC3339, m.LicenceExpiresAt)
		if err != nil {
			return nil, invalid("licence_expires_at must be an RFC 3339 time")
		}
		if !t.After(s.now()) {
			return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("that licence has already expired"))
		}
		expires = &t
	}
	err := pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
		p, err := s.loadProvider(ctx, tx, m.ProviderId)
		if errors.Is(err, pgx.ErrNoRows) {
			return connect.NewError(connect.CodeNotFound, errors.New("no such provider"))
		}
		if err != nil {
			return err
		}
		if p.Status == "WITHDRAWN" {
			return connect.NewError(connect.CodeFailedPrecondition, errors.New("a withdrawn provider cannot be reactivated; register it again"))
		}
		if strings.EqualFold(strings.TrimSpace(m.Actor), p.RegisteredBy) {
			return connect.NewError(connect.CodeFailedPrecondition, errors.New("the person who registered a provider cannot verify its licence"))
		}
		if _, err := tx.Exec(ctx, `update provider set status='ACTIVE', licence_verified_at=$2, licence_verified_by=$3, licence_expires_at=$4, status_reason='' where provider_id=$1`,
			m.ProviderId, s.now(), m.Actor, expires); err != nil {
			return err
		}
		return s.event(ctx, tx, m.ProviderId, m.Actor, "LICENCE_VERIFIED", tsPtr(expires))
	})
	if err != nil {
		return nil, wrap(err)
	}
	p, err := s.loadProvider(ctx, s.Pool, m.ProviderId)
	return connect.NewResponse(&offersv1.VerifyLicenceResponse{Provider: p.proto()}), wrap(err)
}

// SuspendProvider hides every offer of a provider at once. Anyone in compliance may do it, alone and at once: stopping is
// always safe.
func (s *Service) SuspendProvider(ctx context.Context, r *connect.Request[offersv1.SuspendProviderRequest]) (*connect.Response[offersv1.SuspendProviderResponse], error) {
	if err := s.allowed(); err != nil {
		return nil, err
	}
	m := r.Msg
	if err := needActor(m.Actor); err != nil {
		return nil, err
	}
	if strings.TrimSpace(m.Reason) == "" {
		return nil, invalid("a reason is required")
	}
	err := pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `update provider set status='SUSPENDED', status_reason=$2 where provider_id=$1 and status <> 'WITHDRAWN'`, m.ProviderId, m.Reason)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return connect.NewError(connect.CodeNotFound, errors.New("no such provider"))
		}
		return s.event(ctx, tx, m.ProviderId, m.Actor, "SUSPENDED", m.Reason)
	})
	if err != nil {
		return nil, wrap(err)
	}
	p, err := s.loadProvider(ctx, s.Pool, m.ProviderId)
	return connect.NewResponse(&offersv1.SuspendProviderResponse{Provider: p.proto()}), wrap(err)
}

// UpsertOffer writes an offer as a DRAFT every time: changed words mean a changed statement, so someone must switch it on again.
func (s *Service) UpsertOffer(ctx context.Context, r *connect.Request[offersv1.UpsertOfferRequest]) (*connect.Response[offersv1.UpsertOfferResponse], error) {
	if err := s.allowed(); err != nil {
		return nil, err
	}
	m := r.Msg
	if err := needActor(m.Actor); err != nil {
		return nil, err
	}
	want, ok := categoryLicence[m.Category]
	if !ok {
		return nil, invalid("category must be INVESTMENTS or CRYPTO_ASSETS")
	}
	if !idPattern.MatchString(m.OfferId) {
		return nil, invalid("offer_id must be lower-case letters, digits and hyphens")
	}
	title, summary, fee := strings.TrimSpace(m.Title), strings.TrimSpace(m.Summary), strings.TrimSpace(m.FeeDisclosure)
	if title == "" || len(title) > 120 || summary == "" || len(summary) > 500 {
		return nil, invalid("a title of up to 120 and a summary of up to 500 characters are required")
	}
	if fee == "" {
		return nil, invalid("fee_disclosure is required: say what, if anything, Forge is paid")
	}
	if err := httpsURL(m.HandoffUrl); err != nil {
		return nil, invalid("handoff_url " + err.Error())
	}
	p, err := s.loadProvider(ctx, s.Pool, m.ProviderId)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("no such provider"))
	}
	if err != nil {
		return nil, wrap(err)
	}
	if p.LicenceType != want {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("this category needs a provider holding a "+want+" licence"))
	}
	o := offerRow{ID: m.OfferId, ProviderID: p.ID, Category: m.Category, Title: title, Summary: summary, URL: m.HandoffUrl, Fee: fee, Status: "DRAFT", UpdatedBy: m.Actor}
	if _, err := s.Pool.Exec(ctx, `insert into offer (offer_id, provider_id, category, title, summary, handoff_url, fee_disclosure, status, updated_at, updated_by)
		values ($1,$2,$3,$4,$5,$6,$7,'DRAFT',$8,$9)
		on conflict (offer_id) do update set provider_id=$2, category=$3, title=$4, summary=$5, handoff_url=$6, fee_disclosure=$7, status='DRAFT', updated_at=$8, updated_by=$9`,
		o.ID, o.ProviderID, o.Category, o.Title, o.Summary, o.URL, o.Fee, s.now(), m.Actor); err != nil {
		return nil, wrap(err)
	}
	return connect.NewResponse(&offersv1.UpsertOfferResponse{Offer: s.offerProto(p, o)}), nil
}

// SetOfferStatus switches an offer on or off. Switching on needs an active, verified provider, and a different person from
// the one who last wrote the offer.
func (s *Service) SetOfferStatus(ctx context.Context, r *connect.Request[offersv1.SetOfferStatusRequest]) (*connect.Response[offersv1.SetOfferStatusResponse], error) {
	if err := s.allowed(); err != nil {
		return nil, err
	}
	m := r.Msg
	if err := needActor(m.Actor); err != nil {
		return nil, err
	}
	if m.Status != "ACTIVE" && m.Status != "WITHDRAWN" {
		return nil, invalid("status must be ACTIVE or WITHDRAWN")
	}
	o, p, err := scanOffer(s.Pool.QueryRow(ctx, offerSelect+` where o.offer_id = $1`, m.OfferId))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("no such offer"))
	}
	if err != nil {
		return nil, wrap(err)
	}
	if m.Status == "ACTIVE" {
		if !Visible(s.now(), p.Status, p.VerifiedAt, p.ExpiresAt, "ACTIVE") {
			return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("the provider's licence is not currently verified"))
		}
		if strings.EqualFold(strings.TrimSpace(m.Actor), o.UpdatedBy) {
			return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("someone other than the person who wrote the offer must switch it on"))
		}
	}
	if _, err := s.Pool.Exec(ctx, `update offer set status=$2, updated_at=$3 where offer_id=$1`, m.OfferId, m.Status, s.now()); err != nil {
		return nil, wrap(err)
	}
	return connect.NewResponse(&offersv1.SetOfferStatusResponse{Status: m.Status}), nil
}

func (s *Service) ListProviders(ctx context.Context, _ *connect.Request[offersv1.ListProvidersRequest]) (*connect.Response[offersv1.ListProvidersResponse], error) {
	if err := s.allowed(); err != nil {
		return nil, err
	}
	rows, err := s.Pool.Query(ctx, `select provider_id from provider order by name`)
	if err != nil {
		return nil, wrap(err)
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, wrap(err)
		}
		ids = append(ids, id)
	}
	rows.Close()
	out := &offersv1.ListProvidersResponse{}
	for _, id := range ids {
		p, err := s.loadProvider(ctx, s.Pool, id)
		if err != nil {
			return nil, wrap(err)
		}
		out.Providers = append(out.Providers, p.proto())
	}
	return connect.NewResponse(out), nil
}
