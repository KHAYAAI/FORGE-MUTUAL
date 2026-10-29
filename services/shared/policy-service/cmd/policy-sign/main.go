// Command policy-sign writes manifest.json and manifest.sig for a policy bundle, and can generate a
// key pair. The private key is read from POLICY_SIGNING_KEY (base64) and must never be committed.
package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/khayaai/forge-mutual/services/shared/policy-service/internal/bundle"
)

func main() {
	dir := flag.String("dir", "policy", "bundle directory")
	version := flag.String("version", "", "bundle version, for example 2026.10.29-1")
	genKey := flag.Bool("gen-key", false, "print a new Ed25519 key pair and exit")
	flag.Parse()

	if *genKey {
		pub, priv, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			fail(err)
		}
		fmt.Printf("POLICY_SIGNING_KEY=%s\nPOLICY_TRUSTED_KEYS=%s\n",
			base64.StdEncoding.EncodeToString(priv), base64.StdEncoding.EncodeToString(pub))
		return
	}
	raw := strings.TrimSpace(os.Getenv("POLICY_SIGNING_KEY"))
	if raw == "" {
		fail(fmt.Errorf("POLICY_SIGNING_KEY is not set"))
	}
	key, err := base64.StdEncoding.DecodeString(raw)
	if err != nil || len(key) != ed25519.PrivateKeySize {
		fail(fmt.Errorf("POLICY_SIGNING_KEY must be a base64 Ed25519 private key"))
	}
	if err := bundle.Sign(*dir, *version, ed25519.PrivateKey(key)); err != nil {
		fail(err)
	}
	fmt.Printf("signed %s as version %s\n", *dir, *version)
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "policy-sign:", err)
	os.Exit(1)
}
