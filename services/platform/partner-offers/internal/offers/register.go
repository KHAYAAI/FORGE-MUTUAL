package offers

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/jackc/pgx/v5"
)

// Automatic licence checks against the regulator's register.
//
// Forge does not call a regulator's website: no stable interface is known and none is verified (VERIFY). It reads an EXTRACT of the register that
// compliance drops in a file (downloaded from the regulator, or supplied by a data service), and checks every active provider against it each day.
// The extract is only as good as its source, so a stale extract is refused rather than trusted.

// Entry is what the register says about one licence.
type Entry struct {
	Found     bool
	Name      string
	Status    string
	ExpiresOn *time.Time
}

// Register looks a licence up.
type Register interface {
	Lookup(ctx context.Context, regulator, licenceType, number string) (Entry, error)
}

// CSVRegister reads a register extract. Columns, with a header row: regulator, licence_type, licence_number, name, status, expires_on (YYYY-MM-DD or
// empty). The format is PROPOSED (VERIFY against the extract actually used). MaxAge refuses an extract whose file is older than that.
type CSVRegister struct {
	Path   string
	MaxAge time.Duration
	Now    func() time.Time

	mu    sync.Mutex
	mtime time.Time
	rows  map[string]Entry
}

func key(regulator, typ, number string) string {
	return strings.ToUpper(strings.TrimSpace(regulator)) + "|" + strings.ToUpper(strings.TrimSpace(typ)) + "|" + strings.ToUpper(strings.Join(strings.Fields(number), " "))
}

func (c *CSVRegister) now() time.Time {
	if c.Now != nil {
		return c.Now()
	}
	return time.Now()
}

func (c *CSVRegister) load() error {
	st, err := os.Stat(c.Path)
	if err != nil {
		return fmt.Errorf("the register extract cannot be read: %w", err)
	}
	maxAge := c.MaxAge
	if maxAge <= 0 {
		maxAge = 7 * 24 * time.Hour
	}
	if c.now().Sub(st.ModTime()) > maxAge {
		return errors.New("the register extract is out of date")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.rows != nil && st.ModTime().Equal(c.mtime) {
		return nil
	}
	f, err := os.Open(c.Path)
	if err != nil {
		return err
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.FieldsPerRecord = 6
	rows := map[string]Entry{}
	for i := 0; ; i++ {
		rec, err := r.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("the register extract is malformed: %w", err)
		}
		if i == 0 {
			if strings.TrimSpace(rec[0]) != "regulator" {
				return errors.New("the register extract has no header row")
			}
			continue
		}
		e := Entry{Found: true, Name: strings.TrimSpace(rec[3]), Status: strings.ToUpper(strings.TrimSpace(rec[4]))}
		if d := strings.TrimSpace(rec[5]); d != "" {
			t, err := time.Parse("2006-01-02", d)
			if err != nil {
				return fmt.Errorf("the register extract has a bad date %q", d)
			}
			e.ExpiresOn = &t
		}
		rows[key(rec[0], rec[1], rec[2])] = e
	}
	c.rows, c.mtime = rows, st.ModTime()
	return nil
}

func (c *CSVRegister) Lookup(_ context.Context, regulator, licenceType, number string) (Entry, error) {
	if err := c.load(); err != nil {
		return Entry{}, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.rows[key(regulator, licenceType, number)], nil
}

var activeStatuses = map[string]bool{"ACTIVE": true, "AUTHORISED": true, "AUTHORIZED": true}

// normName makes "Acme Wealth (Pty) Ltd" and "ACME WEALTH PTY LTD" the same name.
func normName(n string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(n) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsSpace(r) {
			b.WriteRune(r)
		}
	}
	var out []string
	for _, w := range strings.Fields(b.String()) {
		switch w {
		case "pty", "ltd", "limited", "proprietary", "inc", "the":
		default:
			out = append(out, w)
		}
	}
	return strings.Join(out, " ")
}

// RecheckResult says what a pass did.
type RecheckResult struct{ Confirmed, Suspended, Unchecked int }

// Recheck asks the register about every ACTIVE provider.
//   - The register confirms the licence (found, same name, active, not expired): the check time is recorded, and if RegisterRenews is set the human
//     verification is renewed too (by "register-check"), so a confirmed provider does not drop out after twelve months.
//   - The register contradicts it (not found, a different name, not active, expired): the provider is SUSPENDED at once and every offer vanishes.
//     Only a person can reinstate it, by verifying the licence again.
//   - The register cannot be read: nothing changes. With RegisterMaxAge set, a provider with no confirmation that recent is hidden anyway.
func (s *Service) Recheck(ctx context.Context) (RecheckResult, error) {
	var res RecheckResult
	if s.Register == nil {
		return res, nil
	}
	rows, err := s.Pool.Query(ctx, `select provider_id from provider where status = 'ACTIVE'`)
	if err != nil {
		return res, err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return res, err
		}
		ids = append(ids, id)
	}
	rows.Close()
	for _, id := range ids {
		p, err := s.loadProvider(ctx, s.Pool, id)
		if err != nil {
			return res, err
		}
		e, err := s.Register.Lookup(ctx, p.Regulator, p.LicenceType, p.Number)
		if err != nil {
			res.Unchecked++
			continue
		}
		now := s.now()
		why := ""
		switch {
		case !e.Found:
			why = "the register does not list this licence"
		case normName(e.Name) != normName(p.Name):
			why = "the register lists this licence under a different name"
		case !activeStatuses[e.Status]:
			why = "the register shows the licence as " + strings.ToLower(e.Status)
		case e.ExpiresOn != nil && !e.ExpiresOn.After(now):
			why = "the register shows the licence as expired"
		}
		if why != "" {
			err := pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
				if _, err := tx.Exec(ctx, `update provider set status='SUSPENDED', status_reason=$2 where provider_id=$1 and status='ACTIVE'`, id, "register check: "+why); err != nil {
					return err
				}
				return s.event(ctx, tx, id, "register-check", "AUTO_SUSPENDED", why)
			})
			if err != nil {
				return res, err
			}
			res.Suspended++
			continue
		}
		err = pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
			if _, err := tx.Exec(ctx, `update provider set licence_checked_at=$2, licence_expires_at=coalesce($3, licence_expires_at) where provider_id=$1`, id, now, e.ExpiresOn); err != nil {
				return err
			}
			if s.RegisterRenews {
				if _, err := tx.Exec(ctx, `update provider set licence_verified_at=$2, licence_verified_by='register-check' where provider_id=$1`, id, now); err != nil {
					return err
				}
			}
			return s.event(ctx, tx, id, "register-check", "REGISTER_CONFIRMED", "")
		})
		if err != nil {
			return res, err
		}
		res.Confirmed++
	}
	return res, nil
}
