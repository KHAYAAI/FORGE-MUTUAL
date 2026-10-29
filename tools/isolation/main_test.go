package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func write(t *testing.T, root, rel, body string) {
	t.Helper()
	p := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestPlantedCrossEntityImportFails(t *testing.T) {
	root := t.TempDir()
	write(t, root, "services/bank/a.go", `package bank
import _ "github.com/khayaai/forge-mutual/services/platform/x"
`)
	v, err := check(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(v) != 1 || !strings.Contains(v[0], "platform") {
		t.Fatalf("want one platform violation, got %v", v)
	}
}

func TestSharedAndOwnEntityAllowed(t *testing.T) {
	root := t.TempDir()
	write(t, root, "services/bank/a.go", `package bank
import (
	_ "github.com/khayaai/forge-mutual/services/bank/inner"
	_ "github.com/khayaai/forge-mutual/services/shared/policy"
)
`)
	v, err := check(root)
	if err != nil || len(v) != 0 {
		t.Fatalf("want no violations, got %v (err %v)", v, err)
	}
}
