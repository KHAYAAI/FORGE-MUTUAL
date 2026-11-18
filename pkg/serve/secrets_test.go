package serve

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func resetSecrets(t *testing.T) {
	t.Helper()
	secretsMu.Lock()
	fromFile = map[string]bool{}
	secretLoadErr = nil
	secretsMu.Unlock()
	for _, n := range SecretNames {
		t.Setenv(n, "")
		t.Setenv(n+"_FILE", "")
	}
}

func TestASecretFileIsReadIntoItsSetting(t *testing.T) {
	resetSecrets(t)
	path := filepath.Join(t.TempDir(), "seal")
	if err := os.WriteFile(path, []byte("c2VjcmV0LWtleS1ieXRlcw==\n"), 0o400); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PARTY_SEAL_KEY_FILE", path)
	if err := loadSecretFiles(); err != nil {
		t.Fatal(err)
	}
	if got := os.Getenv("PARTY_SEAL_KEY"); got != "c2VjcmV0LWtleS1ieXRlcw==" {
		t.Fatalf("PARTY_SEAL_KEY = %q, want the file's contents without the newline", got)
	}
	if err := checkSecrets(true); err != nil {
		t.Fatalf("a secret from a file must be accepted in production: %v", err)
	}
}

func TestProductionRefusesASecretInTheEnvironmentAndNamesItButNotItsValue(t *testing.T) {
	resetSecrets(t)
	t.Setenv("DATABASE_URL", "postgres://svc:hunter2@db/ledger")
	t.Setenv("DIDIT_API_KEY", "sk-live-very-secret")
	if err := loadSecretFiles(); err != nil {
		t.Fatal(err)
	}
	if err := checkSecrets(false); err != nil {
		t.Fatalf("development accepts the environment: %v", err)
	}
	err := checkSecrets(true)
	if err == nil || !strings.Contains(err.Error(), "DATABASE_URL, DIDIT_API_KEY") {
		t.Fatalf("production: %v", err)
	}
	for _, leak := range []string{"hunter2", "sk-live"} {
		if strings.Contains(err.Error(), leak) {
			t.Fatalf("the error holds a secret: %v", err)
		}
	}
}

func TestBothFormsAMissingFileAndAnEmptyFileAreErrors(t *testing.T) {
	dir := t.TempDir()
	empty := filepath.Join(dir, "empty")
	if err := os.WriteFile(empty, []byte("\n"), 0o400); err != nil {
		t.Fatal(err)
	}
	for name, set := range map[string]func(){
		"both":    func() { t.Setenv("KYC_WEBHOOK_SECRET", "x"); t.Setenv("KYC_WEBHOOK_SECRET_FILE", empty) },
		"missing": func() { t.Setenv("KYC_WEBHOOK_SECRET_FILE", filepath.Join(dir, "nope")) },
		"empty":   func() { t.Setenv("KYC_WEBHOOK_SECRET_FILE", empty) },
	} {
		resetSecrets(t)
		set()
		if err := loadSecretFiles(); err == nil || !strings.Contains(err.Error(), "KYC_WEBHOOK_SECRET") {
			t.Errorf("%s: %v", name, err)
		}
	}
}

// FromEnv is where every service starts, so it is where the refusal must happen.
func TestFromEnvRefusesASecretInTheEnvironmentInProduction(t *testing.T) {
	resetSecrets(t)
	dir := t.TempDir()
	for _, f := range []string{"c", "k", "ca"} {
		_ = os.WriteFile(filepath.Join(dir, f), []byte("x"), 0o400)
	}
	t.Setenv("FORGE_ENV", "production")
	t.Setenv("TLS_CERT_FILE", filepath.Join(dir, "c"))
	t.Setenv("TLS_KEY_FILE", filepath.Join(dir, "k"))
	t.Setenv("TLS_CA_FILE", filepath.Join(dir, "ca"))
	t.Setenv("PARTY_INDEX_KEY", "direct")
	if _, err := FromEnv(":0"); err == nil || !strings.Contains(err.Error(), "PARTY_INDEX_KEY") {
		t.Fatalf("FromEnv: %v", err)
	}
}
