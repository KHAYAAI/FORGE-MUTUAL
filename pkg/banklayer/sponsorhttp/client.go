package sponsorhttp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/khayaai/forge-mutual/pkg/banklayer"
)

// Client implements banklayer.BankLayer over HTTP. It only runs on the sponsor route: NewClient fetches the
// server's capabilities and refuses any other route, so this adapter cannot be wired to an own-licence bank by
// accident.
type Client struct {
	base string
	hc   *http.Client
	caps banklayer.Capabilities
}

// NewClient connects to a sponsor bank at baseURL and checks its route.
func NewClient(ctx context.Context, baseURL string, hc *http.Client) (*Client, error) {
	if hc == nil {
		hc = &http.Client{Timeout: 10 * time.Second}
	}
	c := &Client{base: strings.TrimRight(baseURL, "/"), hc: hc}
	if err := c.do(ctx, http.MethodGet, "/v1/capabilities", nil, &c.caps); err != nil {
		return nil, fmt.Errorf("sponsor capabilities: %w", err)
	}
	if c.caps.Route != banklayer.RouteSponsor {
		return nil, fmt.Errorf("%w: sponsor client refuses route %q", banklayer.ErrNotSupported, c.caps.Route)
	}
	return c, nil
}

// do sends one request. Non-2xx responses map back to the banklayer sentinel errors.
func (c *Client) do(ctx context.Context, method, path string, body, out any) error {
	var rdr io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rdr = bytes.NewReader(buf)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.base+path, rdr)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.hc.Do(req)
	if err != nil {
		return fmt.Errorf("sponsor unreachable: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return err
	}
	if resp.StatusCode >= 300 {
		var we wireError
		_ = json.Unmarshal(raw, &we)
		return mapErr(resp.StatusCode, we)
	}
	if out == nil || resp.StatusCode == http.StatusNoContent {
		return nil
	}
	return json.Unmarshal(raw, out)
}

func mapErr(status int, we wireError) error {
	var sentinel error
	switch {
	case status == http.StatusBadRequest:
		sentinel = banklayer.ErrInvalid
	case status == http.StatusNotFound:
		sentinel = banklayer.ErrNotFound
	case status == http.StatusConflict && we.Code == "CLOSED":
		sentinel = banklayer.ErrClosed
	case status == http.StatusConflict:
		sentinel = banklayer.ErrConflict
	case status == http.StatusNotImplemented:
		sentinel = banklayer.ErrNotSupported
	default:
		return fmt.Errorf("sponsor returned status %d", status)
	}
	return fmt.Errorf("%w: %s", sentinel, we.Message)
}

// Capabilities implements banklayer.BankLayer.
func (c *Client) Capabilities() banklayer.Capabilities { return c.caps }

// OpenAccount implements banklayer.BankLayer.
func (c *Client) OpenAccount(ctx context.Context, req banklayer.OpenAccountRequest) (banklayer.Account, error) {
	var acc banklayer.Account
	err := c.do(ctx, http.MethodPost, "/v1/accounts", req, &acc)
	return acc, err
}

// CloseAccount implements banklayer.BankLayer.
func (c *Client) CloseAccount(ctx context.Context, accountID, reason string) error {
	return c.do(ctx, http.MethodPost, "/v1/accounts/"+url.PathEscape(accountID)+"/close",
		map[string]string{"reason": reason}, nil)
}

// Rails implements banklayer.BankLayer.
func (c *Client) Rails() banklayer.Rail { return sponsorRail{c} }

// ForeignCurrency implements banklayer.BankLayer.
func (c *Client) ForeignCurrency() []banklayer.ForeignAccountProvider {
	return []banklayer.ForeignAccountProvider{usdPartner{c}}
}

// StatementSource implements banklayer.BankLayer.
func (c *Client) StatementSource() banklayer.StatementSource { return sponsorStatements{c} }

// DepositInsurance implements banklayer.BankLayer.
func (c *Client) DepositInsurance() banklayer.InsuranceReporter { return insurance{c} }

// PrudentialReporter implements banklayer.BankLayer.
func (c *Client) PrudentialReporter() banklayer.PrudentialReporter { return prudential{c} }

// InjectInbound books a simulated inbound credit. It exists only on the simulator; a real sponsor returns 404.
func (c *Client) InjectInbound(ctx context.Context, asset string, minor int64, reference string, partner bool) error {
	return c.do(ctx, http.MethodPost, "/sim/inbound", map[string]any{
		"asset": asset, "minor": minor, "reference": reference, "partner": partner,
	}, nil)
}

type sponsorRail struct{ c *Client }

func (r sponsorRail) Pay(ctx context.Context, req banklayer.PayRequest) (banklayer.PayResult, error) {
	var res banklayer.PayResult
	err := r.c.do(ctx, http.MethodPost, "/v1/payments", req, &res)
	return res, err
}

func (r sponsorRail) Status(ctx context.Context, id string) (banklayer.PayResult, error) {
	var res banklayer.PayResult
	err := r.c.do(ctx, http.MethodGet, "/v1/payments/"+url.PathEscape(id), nil, &res)
	return res, err
}

type usdPartner struct{ c *Client }

func (usdPartner) Partner() string { return "usd-partner" }
func (usdPartner) Asset() string   { return "USD" }

func (p usdPartner) OpenAccount(ctx context.Context, req banklayer.OpenAccountRequest) (banklayer.Account, error) {
	var acc banklayer.Account
	err := p.c.do(ctx, http.MethodPost, "/v1/partner/usd/accounts", req, &acc)
	return acc, err
}

func (p usdPartner) Payout(ctx context.Context, req banklayer.PayRequest) (banklayer.PayResult, error) {
	var res banklayer.PayResult
	err := p.c.do(ctx, http.MethodPost, "/v1/partner/usd/payouts", req, &res)
	return res, err
}

func (p usdPartner) PayoutStatus(ctx context.Context, id string) (banklayer.PayResult, error) {
	var res banklayer.PayResult
	err := p.c.do(ctx, http.MethodGet, "/v1/partner/usd/payouts/"+url.PathEscape(id), nil, &res)
	return res, err
}

func (p usdPartner) Statement(ctx context.Context, date time.Time) (banklayer.Statement, error) {
	var st banklayer.Statement
	q := url.Values{"date": {date.UTC().Format("2006-01-02")}}
	err := p.c.do(ctx, http.MethodGet, "/v1/partner/usd/statements?"+q.Encode(), nil, &st)
	return st, err
}

func (p usdPartner) Inbound(ctx context.Context, since time.Time) ([]banklayer.Movement, error) {
	var list []banklayer.Movement
	q := url.Values{"since": {since.UTC().Format(time.RFC3339Nano)}}
	err := p.c.do(ctx, http.MethodGet, "/v1/partner/usd/inbound?"+q.Encode(), nil, &list)
	return list, err
}

type sponsorStatements struct{ c *Client }

func (s sponsorStatements) Statement(ctx context.Context, asset string, date time.Time) (banklayer.Statement, error) {
	var st banklayer.Statement
	q := url.Values{"date": {date.UTC().Format("2006-01-02")}}
	err := s.c.do(ctx, http.MethodGet, "/v1/statements/"+url.PathEscape(asset)+"?"+q.Encode(), nil, &st)
	return st, err
}

func (s sponsorStatements) Inbound(ctx context.Context, asset string, since time.Time) ([]banklayer.Movement, error) {
	var list []banklayer.Movement
	q := url.Values{"since": {since.UTC().Format(time.RFC3339Nano)}}
	err := s.c.do(ctx, http.MethodGet, "/v1/inbound/"+url.PathEscape(asset)+"?"+q.Encode(), nil, &list)
	return list, err
}

type insurance struct{ c *Client }

func (insurance) Duty() string { return "sponsor" }

func (i insurance) Report(ctx context.Context, asOf time.Time, asset string, insuredMinor int64) error {
	return i.c.do(ctx, http.MethodPost, "/v1/insurance/"+url.PathEscape(asset), map[string]any{
		"as_of": asOf.UTC(), "insured_minor": insuredMinor,
	}, nil)
}

type prudential struct{ c *Client }

func (prudential) Required() bool { return false }

func (p prudential) Submit(ctx context.Context, period string, body []byte) error {
	if !json.Valid(body) {
		return errors.Join(banklayer.ErrInvalid, errors.New("body is not JSON"))
	}
	return p.c.do(ctx, http.MethodPost, "/v1/prudential", map[string]any{
		"period": period, "body": json.RawMessage(body),
	}, nil)
}
