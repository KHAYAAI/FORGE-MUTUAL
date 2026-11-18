// Package crypt seals contact addresses at rest with AES-256-GCM. The key comes from the environment and never from
// the repository. A tampered ciphertext fails to open.
package crypt

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
)

// Key is a 32-byte AES-256 key.
type Key []byte

// ParseKey decodes a base64 key of exactly 32 bytes.
func ParseKey(b64 string) (Key, error) {
	k, err := base64.StdEncoding.DecodeString(b64)
	if err != nil || len(k) != 32 {
		return nil, errors.New("crypt: the key must be 32 bytes, base64 encoded")
	}
	return Key(k), nil
}

func (k Key) aead() (cipher.AEAD, error) {
	block, err := aes.NewCipher(k)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

// Seal encrypts plaintext. The nonce is random and is stored in front of the ciphertext.
func (k Key) Seal(plaintext []byte) ([]byte, error) {
	g, err := k.aead()
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, g.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return g.Seal(nonce, nonce, plaintext, nil), nil
}

// Open decrypts and authenticates a sealed value.
func (k Key) Open(sealed []byte) ([]byte, error) {
	g, err := k.aead()
	if err != nil {
		return nil, err
	}
	if len(sealed) < g.NonceSize() {
		return nil, errors.New("crypt: sealed value is too short")
	}
	pt, err := g.Open(nil, sealed[:g.NonceSize()], sealed[g.NonceSize():], nil)
	if err != nil {
		return nil, fmt.Errorf("crypt: the value does not authenticate: %w", err)
	}
	return pt, nil
}
