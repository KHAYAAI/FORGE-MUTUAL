// Package sanctions loads a sanctions list, checks its integrity and freshness, and returns a screener. A list that
// fails any check is refused, and a stale list fails closed at screening time (pkg/screening).
//
// The list format here is Forge's normalised form, not any official format. The official FIC list's format and
// publication channel are unverified (VERIFY-FIC), so the source adapter that converts it is not written yet.
package sanctions

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/khayaai/forge-mutual/pkg/screening"
)

// List is one normalised sanctions list.
type List struct {
	Source      string    `json:"source"`
	Version     string    `json:"version"`
	PublishedAt time.Time `json:"published_at"`
	Entries     []Entry   `json:"entries"`
}

// Entry is one listed party.
type Entry struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Aliases []string `json:"aliases"`
}

// ErrIntegrity is returned when the file does not match its expected checksum.
var ErrIntegrity = errors.New("sanctions: checksum does not match")

// Parse reads a list from data. expectedSHA256 is the hex checksum from the publication, and it must match: a list
// that changed on the way in is refused. Unknown fields are refused, so a format change cannot pass silently.
func Parse(data []byte, expectedSHA256 string) (List, error) {
	sum := sha256.Sum256(data)
	if !strings.EqualFold(hex.EncodeToString(sum[:]), expectedSHA256) {
		return List{}, ErrIntegrity
	}
	dec := json.NewDecoder(strings.NewReader(string(data)))
	dec.DisallowUnknownFields()
	var l List
	if err := dec.Decode(&l); err != nil {
		return List{}, fmt.Errorf("sanctions: parse: %w", err)
	}
	if l.Source == "" || l.Version == "" || l.PublishedAt.IsZero() {
		return List{}, errors.New("sanctions: source, version and published_at are required")
	}
	if len(l.Entries) == 0 {
		return List{}, errors.New("sanctions: a list with no entries is refused; an empty list would screen everyone clear")
	}
	for i, e := range l.Entries {
		if e.ID == "" || e.Name == "" {
			return List{}, fmt.Errorf("sanctions: entry %d needs an id and a name", i)
		}
	}
	return l, nil
}

// Screener returns a screener over the list. It refuses to build from a list older than maxAge, and the screener
// itself fails closed with screening.ErrStale when it is used after the list has aged out.
func (l List) Screener(maxAge time.Duration, now func() time.Time) (*screening.Fixture, error) {
	var names []string
	for _, e := range l.Entries {
		names = append(names, e.Name)
		names = append(names, e.Aliases...)
	}
	return screening.NewFixture(names, l.PublishedAt, maxAge, now)
}
