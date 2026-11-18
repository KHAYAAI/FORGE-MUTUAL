// Package bundle builds, signs and verifies policy bundles (ARCHITECTURE.md section 10).
//
// A bundle is a directory holding Rego modules under forge/ and JSON data files at the top level
// (each file becomes data.<name>). manifest.json lists the SHA-256 digest of every bundle file, and
// manifest.sig is an Ed25519 signature over the exact manifest.json bytes. Load refuses any file
// that is missing, extra or altered.
package bundle

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	manifestFile  = "manifest.json"
	signatureFile = "manifest.sig"
	// DevUnsignedVersion is the version reported when verification is switched off for development.
	DevUnsignedVersion = "dev-unsigned"
)

// Manifest lists every file in a bundle with its digest.
type Manifest struct {
	Version string            `json:"version"`
	Files   map[string]string `json:"files"`
}

// Bundle is a verified bundle, ready to compile.
type Bundle struct {
	Version string
	Modules map[string]string // relative path to Rego source
	Data    map[string]any    // data file name without extension to decoded JSON
}

// ErrVerification is wrapped by every verification failure.
var ErrVerification = errors.New("policy bundle verification failed")

// included reports whether a relative path belongs to the signed bundle.
func included(rel string) bool {
	base := filepath.Base(rel)
	switch {
	case base == manifestFile, base == signatureFile, base == "README.md":
		return false
	case strings.HasSuffix(rel, "_test.rego"):
		return false
	case strings.HasSuffix(rel, ".rego"):
		return strings.HasPrefix(rel, "forge"+string(filepath.Separator))
	case strings.HasSuffix(rel, ".json"):
		return !strings.Contains(rel, string(filepath.Separator))
	}
	return false
}

// files returns the bundle files under dir as relative slash-separated paths with digests.
func files(dir string) (map[string]string, error) {
	out := make(map[string]string)
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		if !included(rel) {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(b)
		out[filepath.ToSlash(rel)] = hex.EncodeToString(sum[:])
		return nil
	})
	return out, err
}

// Sign writes manifest.json (listing dir's current files) and manifest.sig.
func Sign(dir, version string, priv ed25519.PrivateKey) error {
	if version == "" {
		return errors.New("bundle: version required")
	}
	fs, err := files(dir)
	if err != nil {
		return err
	}
	raw, err := json.MarshalIndent(Manifest{Version: version, Files: fs}, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	if err := os.WriteFile(filepath.Join(dir, manifestFile), raw, 0o644); err != nil {
		return err
	}
	sig := ed25519.Sign(priv, raw)
	return os.WriteFile(filepath.Join(dir, signatureFile), []byte(base64.StdEncoding.EncodeToString(sig)+"\n"), 0o644)
}

// Load verifies dir against the trusted public keys and reads the bundle. With devUnsigned set and
// no keys, verification is skipped; the caller must log that loudly and refuse it in production.
func Load(dir string, trusted []ed25519.PublicKey, devUnsigned bool) (*Bundle, error) {
	if devUnsigned && len(trusted) == 0 {
		return read(dir, DevUnsignedVersion)
	}
	raw, err := os.ReadFile(filepath.Join(dir, manifestFile))
	if err != nil {
		return nil, fmt.Errorf("%w: manifest: %v", ErrVerification, err)
	}
	sigText, err := os.ReadFile(filepath.Join(dir, signatureFile))
	if err != nil {
		return nil, fmt.Errorf("%w: signature: %v", ErrVerification, err)
	}
	sig, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(sigText)))
	if err != nil {
		return nil, fmt.Errorf("%w: signature encoding: %v", ErrVerification, err)
	}
	if !anyKeyVerifies(trusted, raw, sig) {
		return nil, fmt.Errorf("%w: no trusted key verifies the manifest", ErrVerification)
	}
	var m Manifest
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("%w: manifest json: %v", ErrVerification, err)
	}
	if m.Version == "" {
		return nil, fmt.Errorf("%w: manifest has no version", ErrVerification)
	}
	actual, err := files(dir)
	if err != nil {
		return nil, err
	}
	if err := compare(m.Files, actual); err != nil {
		return nil, err
	}
	return read(dir, m.Version)
}

func anyKeyVerifies(keys []ed25519.PublicKey, msg, sig []byte) bool {
	for _, k := range keys {
		if len(k) == ed25519.PublicKeySize && ed25519.Verify(k, msg, sig) {
			return true
		}
	}
	return false
}

// compare reports a verification error when the file sets or any digest differ.
func compare(want, got map[string]string) error {
	var problems []string
	for name, digest := range want {
		g, ok := got[name]
		switch {
		case !ok:
			problems = append(problems, "missing "+name)
		case g != digest:
			problems = append(problems, "altered "+name)
		}
	}
	for name := range got {
		if _, ok := want[name]; !ok {
			problems = append(problems, "unexpected "+name)
		}
	}
	if len(problems) > 0 {
		sort.Strings(problems)
		return fmt.Errorf("%w: %s", ErrVerification, strings.Join(problems, "; "))
	}
	return nil
}

func read(dir, version string) (*Bundle, error) {
	b := &Bundle{Version: version, Modules: map[string]string{}, Data: map[string]any{}}
	fsys, err := files(dir)
	if err != nil {
		return nil, err
	}
	for rel := range fsys {
		raw, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel)))
		if err != nil {
			return nil, err
		}
		if strings.HasSuffix(rel, ".rego") {
			b.Modules[rel] = string(raw)
			continue
		}
		var v any
		if err := json.Unmarshal(raw, &v); err != nil {
			return nil, fmt.Errorf("bundle: %s: %w", rel, err)
		}
		b.Data[strings.TrimSuffix(rel, ".json")] = v
	}
	if len(b.Modules) == 0 {
		return nil, errors.New("bundle: no rego modules")
	}
	return b, nil
}

// ParseKey decodes a base64 Ed25519 public key (32 bytes).
func ParseKey(s string) (ed25519.PublicKey, error) {
	b, err := base64.StdEncoding.DecodeString(strings.TrimSpace(s))
	if err != nil || len(b) != ed25519.PublicKeySize {
		return nil, errors.New("bundle: public key must be 32 bytes, base64")
	}
	return ed25519.PublicKey(b), nil
}
