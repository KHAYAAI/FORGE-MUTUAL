package serve

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
)

// Secrets come from files, not from the environment.
//
// An environment variable is visible to anything that can read the process's environment or the container's
// definition: `docker inspect`, a Kubernetes pod spec, a crash dump, a support bundle. A mounted file is visible only
// to the process that mounts it, and it is where secret stores deliver: Kubernetes secrets, Vault Agent, cloud secret
// CSI drivers. So for each secret NAME a service reads, it accepts NAME_FILE, the path of a file that holds the value,
// and production refuses NAME given directly.
//
// Services keep reading os.Getenv(NAME): this package reads every NAME_FILE once, when the process starts, and sets
// NAME from it, before any service code runs. Trailing newlines are removed, because most tools that write secret
// files add one.

// SecretNames are the settings that carry secrets. Any of them may be given as NAME_FILE, and in production must be.
var SecretNames = []string{
	"DATABASE_URL", // holds the database password
	"PARTY_SEAL_KEY",
	"PARTY_INDEX_KEY",
	"NOTIFICATIONS_KEY",
	"KYC_WEBHOOK_SECRET",
	"DIDIT_API_KEY",
	"DIDIT_WEBHOOK_SECRET",
	"DIDIT_WEBHOOK_SECRET_PREVIOUS",
	"FINERACT_PASSWORD",
	"OIDC_CLIENT_SECRET",
	"POLICY_SIGNING_KEY",
}

var (
	secretsMu     sync.Mutex
	fromFile      = map[string]bool{} // NAME was set from NAME_FILE
	secretLoadErr error
)

func init() { secretLoadErr = loadSecretFiles() }

// loadSecretFiles sets each NAME from NAME_FILE. Setting both is an error: which one is meant would be a guess.
func loadSecretFiles() error {
	secretsMu.Lock()
	defer secretsMu.Unlock()
	var problems []string
	for _, name := range SecretNames {
		path := os.Getenv(name + "_FILE")
		if path == "" {
			continue
		}
		if os.Getenv(name) != "" && !fromFile[name] {
			problems = append(problems, fmt.Sprintf("%s and %s_FILE are both set; set only %s_FILE", name, name, name))
			continue
		}
		b, err := os.ReadFile(path)
		if err != nil {
			problems = append(problems, fmt.Sprintf("%s_FILE: %v", name, err))
			continue
		}
		value := strings.TrimRight(string(b), "\r\n")
		if value == "" {
			problems = append(problems, fmt.Sprintf("%s_FILE: %s is empty", name, path))
			continue
		}
		if err := os.Setenv(name, value); err != nil {
			problems = append(problems, fmt.Sprintf("%s: %v", name, err))
			continue
		}
		fromFile[name] = true
	}
	if len(problems) > 0 {
		return fmt.Errorf("secret files: %s", strings.Join(problems, "; "))
	}
	return nil
}

// checkSecrets reports a secret file that could not be read, and in production any secret given directly in the
// environment. The error names the settings, never their values.
func checkSecrets(production bool) error {
	secretsMu.Lock()
	defer secretsMu.Unlock()
	if secretLoadErr != nil {
		return secretLoadErr
	}
	if !production {
		return nil
	}
	var direct []string
	for _, name := range SecretNames {
		if os.Getenv(name) != "" && !fromFile[name] {
			direct = append(direct, name)
		}
	}
	if len(direct) > 0 {
		sort.Strings(direct)
		return fmt.Errorf("production refuses secrets in the environment: give %s as a file with the _FILE setting instead", strings.Join(direct, ", "))
	}
	return nil
}
