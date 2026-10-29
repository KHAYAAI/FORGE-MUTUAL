package bundle

import (
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func write(t *testing.T, dir, rel, body string) {
	t.Helper()
	p := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func newBundle(t *testing.T) (string, ed25519.PublicKey, ed25519.PrivateKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	write(t, dir, "forge/demo/demo.rego", "package forge.demo\n\nimport rego.v1\n\ndefault decision := {\"allow\": false}\n")
	write(t, dir, "forge/demo/demo_test.rego", "package forge.demo\n\nimport rego.v1\n")
	write(t, dir, "tiers.json", `{"standard": {"per_txn": {"ZAR": 5}}}`)
	write(t, dir, "README.md", "not part of the bundle")
	if err := Sign(dir, "2026.10.29-1", priv); err != nil {
		t.Fatal(err)
	}
	return dir, pub, priv
}

func TestSignedBundleLoads(t *testing.T) {
	dir, pub, _ := newBundle(t)
	b, err := Load(dir, []ed25519.PublicKey{pub}, false)
	if err != nil {
		t.Fatal(err)
	}
	if b.Version != "2026.10.29-1" || len(b.Modules) != 1 {
		t.Fatalf("bundle = %+v", b)
	}
	if _, ok := b.Data["tiers"]; !ok {
		t.Fatal("tiers data not loaded")
	}
}

func TestAlteredFileIsRefused(t *testing.T) {
	dir, pub, _ := newBundle(t)
	write(t, dir, "forge/demo/demo.rego", "package forge.demo\n\nimport rego.v1\n\ndefault decision := {\"allow\": true}\n")
	if _, err := Load(dir, []ed25519.PublicKey{pub}, false); !errors.Is(err, ErrVerification) {
		t.Fatalf("altered rule must be refused, got %v", err)
	}
}

func TestExtraRegoFileIsRefused(t *testing.T) {
	dir, pub, _ := newBundle(t)
	write(t, dir, "forge/demo/extra.rego", "package forge.demo\n")
	if _, err := Load(dir, []ed25519.PublicKey{pub}, false); !errors.Is(err, ErrVerification) {
		t.Fatalf("unsigned extra module must be refused, got %v", err)
	}
}

func TestWrongKeyIsRefused(t *testing.T) {
	dir, _, _ := newBundle(t)
	other, _, _ := ed25519.GenerateKey(rand.Reader)
	if _, err := Load(dir, []ed25519.PublicKey{other}, false); !errors.Is(err, ErrVerification) {
		t.Fatalf("wrong key must be refused, got %v", err)
	}
}

func TestMissingSignatureIsRefused(t *testing.T) {
	dir, pub, _ := newBundle(t)
	if err := os.Remove(filepath.Join(dir, signatureFile)); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(dir, []ed25519.PublicKey{pub}, false); !errors.Is(err, ErrVerification) {
		t.Fatalf("unsigned bundle must be refused, got %v", err)
	}
}

func TestDevUnsignedOnlyWithoutKeys(t *testing.T) {
	dir, pub, _ := newBundle(t)
	b, err := Load(dir, nil, true)
	if err != nil || b.Version != DevUnsignedVersion {
		t.Fatalf("dev mode: %v %v", b, err)
	}
	// With trusted keys configured, dev mode must not bypass verification.
	write(t, dir, "forge/demo/demo.rego", "package forge.demo\n\nimport rego.v1\n")
	if _, err := Load(dir, []ed25519.PublicKey{pub}, true); !errors.Is(err, ErrVerification) {
		t.Fatalf("keys present: verification must run, got %v", err)
	}
}
