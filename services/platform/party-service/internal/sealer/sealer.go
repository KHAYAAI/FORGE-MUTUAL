// Package sealer protects national ID numbers at rest (ARCHITECTURE.md 11.2, 18.3). Values are sealed
// with AES-256-GCM; a keyed HMAC index lets the service detect a duplicate without decrypting.
//
// The keys come from the environment in this build. Production must use OpenBao transit keys
// (VERIFY-OPENBAO); the interface here is what that adapter will implement.
package sealer

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"io"
	"strings"
)

var aad = []byte("forge.party.national_id.v1")

// Sealer seals and opens national ID numbers and computes their blind index.
type Sealer struct {
	aead     cipher.AEAD
	indexKey []byte
}

// New returns a sealer. encKey must be 32 bytes; indexKey must be at least 32 bytes and distinct from encKey.
func New(encKey, indexKey []byte) (*Sealer, error) {
	if len(encKey) != 32 {
		return nil, errors.New("sealer: encryption key must be 32 bytes")
	}
	if len(indexKey) < 32 {
		return nil, errors.New("sealer: index key must be at least 32 bytes")
	}
	block, err := aes.NewCipher(encKey)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Sealer{aead: aead, indexKey: append([]byte(nil), indexKey...)}, nil
}

// normalize removes spaces and upper-cases, so "8001 0100 1234 5" and "8001010012345" match.
func normalize(id string) string {
	return strings.ToUpper(strings.Join(strings.Fields(id), ""))
}

// Seal returns nonce || ciphertext.
func (s *Sealer) Seal(id string) ([]byte, error) {
	nonce := make([]byte, s.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	return s.aead.Seal(nonce, nonce, []byte(normalize(id)), aad), nil
}

// Open reverses Seal. A tampered value is an error.
func (s *Sealer) Open(sealed []byte) (string, error) {
	n := s.aead.NonceSize()
	if len(sealed) < n {
		return "", errors.New("sealer: value too short")
	}
	plain, err := s.aead.Open(nil, sealed[:n], sealed[n:], aad)
	if err != nil {
		return "", errors.New("sealer: value does not authenticate")
	}
	return string(plain), nil
}

// Index returns the deterministic blind index for duplicate detection.
func (s *Sealer) Index(id string) []byte {
	mac := hmac.New(sha256.New, s.indexKey)
	mac.Write([]byte(normalize(id)))
	return mac.Sum(nil)
}

// Mask keeps the last four characters, for display.
func Mask(id string) string {
	n := normalize(id)
	if len(n) <= 4 {
		return strings.Repeat("*", len(n))
	}
	return strings.Repeat("*", len(n)-4) + n[len(n)-4:]
}
