package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The vectors are signed by Didit's own reference code (internal/kyc/testdata/gen_didit_vectors.py), so these tests
// check the command against an independent implementation, not against the code it calls.
type vectors struct {
	Secret  string `json:"secret"`
	Vectors []struct {
		Name      string `json:"name"`
		Body      string `json:"body"`
		Signature string `json:"signature"`
	} `json:"vectors"`
}

func load(t *testing.T) vectors {
	t.Helper()
	raw, err := os.ReadFile("../../internal/kyc/testdata/didit_v2_vectors.json")
	if err != nil {
		t.Fatal(err)
	}
	var v vectors
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatal(err)
	}
	return v
}

// record writes a delivery the way Didit's API returns one: the headers and the body as JSON text.
func record(t *testing.T, v vectors, name, stamp string, edit func(body string) string) string {
	t.Helper()
	for _, x := range v.Vectors {
		if x.Name != name {
			continue
		}
		body := x.Body
		if edit != nil {
			body = edit(body)
		}
		header, _ := json.Marshal(map[string]string{"X-Signature-V2": x.Signature, "X-Timestamp": stamp, "Content-Type": "application/json"})
		out, _ := json.Marshal(map[string]string{"request_header": string(header), "request_body": body})
		path := filepath.Join(t.TempDir(), "delivery.json")
		if err := os.WriteFile(path, out, 0o600); err != nil {
			t.Fatal(err)
		}
		return path
	}
	t.Fatalf("no vector named %q", name)
	return ""
}

func TestARecordedDeliveryVerifiesAgainstTheReferenceSignature(t *testing.T) {
	v := load(t)
	t.Setenv("DIDIT_WEBHOOK_SECRET", v.Secret)
	t.Setenv("DIDIT_ENVIRONMENT", "live")

	// An old delivery is checked at its own time, so it is not rejected as stale.
	if code, err := run(record(t, v, "status updated, approved (shape from Didit's docs) [style 0]", "1774970000", nil)); code != 0 || err != nil {
		t.Fatalf("approved: exit %d, %v", code, err)
	}
	if code, err := run(record(t, v, "status updated, declined [style 1]", "1774970000", nil)); code != 0 || err != nil {
		t.Fatalf("declined, indented body: exit %d, %v", code, err)
	}
	if code, err := run(record(t, v, "status updated, in progress (not a verdict) [style 2]", "1774970000", nil)); code != 3 || err != nil {
		t.Fatalf("not a verdict: exit %d, %v", code, err)
	}
}

func TestADeliveryThatDoesNotVerifyIsReported(t *testing.T) {
	v := load(t)
	t.Setenv("DIDIT_WEBHOOK_SECRET", v.Secret)
	t.Setenv("DIDIT_ENVIRONMENT", "live")
	name := "status updated, approved (shape from Didit's docs) [style 0]"

	code, err := run(record(t, v, name, "1774970000", func(b string) string { return strings.Replace(b, `"Approved"`, `"Declined"`, 1) }))
	if code != 1 || err == nil || !strings.Contains(err.Error(), "does not verify") {
		t.Fatalf("altered body: exit %d, %v", code, err)
	}
	t.Setenv("DIDIT_WEBHOOK_SECRET", "whsec_a_different_secret_0123456789")
	if code, err = run(record(t, v, name, "1774970000", nil)); code != 1 || err == nil {
		t.Fatalf("wrong secret: exit %d, %v", code, err)
	}
	if strings.Contains(err.Error(), v.Secret) || strings.Contains(err.Error(), "whsec_a_different") {
		t.Fatalf("the error carries a secret: %v", err)
	}
}

func TestACrossEnvironmentDeliveryIsRefusedEvenWhenItVerifies(t *testing.T) {
	v := load(t)
	t.Setenv("DIDIT_WEBHOOK_SECRET", v.Secret)
	t.Setenv("DIDIT_ENVIRONMENT", "sandbox") // the recorded delivery says "live"
	if code, _ := run(record(t, v, "status updated, approved (shape from Didit's docs) [style 0]", "1774970000", nil)); code != 1 {
		t.Fatalf("a live delivery at a sandbox checker must not pass: exit %d", code)
	}
}

func TestInputThatIsNotADeliveryIsRefused(t *testing.T) {
	t.Setenv("DIDIT_WEBHOOK_SECRET", "whsec_anything_long_enough_0123")
	bad := filepath.Join(t.TempDir(), "x.json")
	for name, content := range map[string]string{
		"not json": "hello", "no fields": `{}`, "no timestamp": `{"request_header":"{\"X-Signature-V2\":\"a\"}","request_body":"{}"}`,
		"header not an object": `{"request_header":"[1]","request_body":"{}"}`,
	} {
		if err := os.WriteFile(bad, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		if code, err := run(bad); code != 1 || err == nil {
			t.Errorf("%s: exit %d, %v", name, code, err)
		}
	}
	t.Setenv("DIDIT_WEBHOOK_SECRET", "")
	if code, err := run(bad); code != 1 || err == nil {
		t.Errorf("no secret: exit %d, %v", code, err)
	}
}
