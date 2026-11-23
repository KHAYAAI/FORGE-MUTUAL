// Command didit-verify runs one recorded Didit webhook delivery through this service's verifier.
//
// It is the first check to make against a real Didit account. Didit records every delivery it makes, with the exact
// headers and body it sent, even when the receiving address cannot be reached, and returns them from
// GET /v3/session/{sessionId}/webhook/{webhook_uuid} (docs.didit.me, "Get Session Webhook Delivery API"). Feeding one
// to this command shows whether the signature scheme in kyc/didit.go matches what Didit really sends, without a public
// endpoint and without exposing anything.
//
//	DIDIT_WEBHOOK_SECRET=... DIDIT_ENVIRONMENT=sandbox didit-verify -file delivery.json
//
// The input is the JSON of one delivery. Its request_header and request_body may be JSON text or objects. The clock is
// set to the delivery's own X-Timestamp, so an old delivery is not rejected as stale: the live handler enforces the
// five-minute window, and the unit tests cover it.
//
// The secret is read from the environment and is never printed. Exit status: 0 for a verified verdict, 3 for a verified
// delivery that is not a verdict, 1 for anything else.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/khayaai/forge-mutual/services/platform/onboarding/internal/kyc"
)

func main() {
	file := flag.String("file", "", "the delivery JSON (default: standard input)")
	flag.Parse()
	code, err := run(*file)
	if err != nil {
		fmt.Fprintln(os.Stderr, "didit-verify:", err)
	}
	os.Exit(code)
}

func run(file string) (int, error) {
	secret := os.Getenv("DIDIT_WEBHOOK_SECRET")
	if secret == "" {
		return 1, errors.New("DIDIT_WEBHOOK_SECRET is required")
	}
	env := os.Getenv("DIDIT_ENVIRONMENT")
	if env == "" {
		env = "sandbox"
	}
	var in io.Reader = os.Stdin
	if file != "" {
		f, err := os.Open(file)
		if err != nil {
			return 1, err
		}
		defer f.Close()
		in = f
	}
	raw, err := io.ReadAll(io.LimitReader(in, 8<<20))
	if err != nil {
		return 1, err
	}
	header, body, err := parseDelivery(raw)
	if err != nil {
		return 1, err
	}
	stamp, err := strconv.ParseInt(strings.TrimSpace(header.Get("X-Timestamp")), 10, 64)
	if err != nil {
		return 1, errors.New("the delivery has no numeric X-Timestamp header")
	}

	d, err := kyc.NewDidit(kyc.DiditConfig{
		// Only verification is used here, so the API key and workflows are placeholders.
		APIKey: "unused", KYCWorkflowID: "00000000-0000-4000-8000-000000000001", KYBWorkflowID: "00000000-0000-4000-8000-000000000002",
		WebhookSecret: secret, PreviousWebhookSecret: os.Getenv("DIDIT_WEBHOOK_SECRET_PREVIOUS"), Environment: env,
		Now: func() time.Time { return time.Unix(stamp, 0) },
	})
	if err != nil {
		return 1, err
	}

	got := header.Get("X-Signature-V2")
	want, canonErr := kyc.SignDidit(secret, body)
	ev, err := d.VerifyWebhook(body, header)
	switch {
	case err == nil:
		fmt.Printf("ok: the signature verifies\n  verdict:    %s\n  onboarding: %s\n  subject:    %s\n  session:    %s\n  event:      %s\n",
			ev.Verdict, ev.OnboardingID, ev.SubjectID, ev.SessionID, ev.EventID)
		return 0, nil
	case errors.Is(err, kyc.ErrIgnored):
		fmt.Println("ok: the signature verifies, and the delivery is not a verdict (a status such as In Progress, or another event type)")
		return 3, nil
	case errors.Is(err, kyc.ErrSignature):
		if canonErr != nil {
			return 1, fmt.Errorf("the body is not a JSON document: %w", canonErr)
		}
		// Enough of each to see whether they are close, never enough to forge one.
		return 1, fmt.Errorf("the signature does not verify: X-Signature-V2 starts %q, this verifier computed %q. "+
			"Check the secret and the destination it belongs to, then the canonical form in kyc/didit_canon.go", prefix(got), prefix(want))
	default:
		fmt.Printf("the signature verifies, but the delivery is refused: %v\n", err)
		return 1, nil
	}
}

func prefix(s string) string {
	if len(s) > 8 {
		return s[:8] + "..."
	}
	return s
}

// parseDelivery reads a recorded delivery. request_header and request_body are JSON text in Didit's response; an object
// is accepted too.
func parseDelivery(raw []byte) (http.Header, []byte, error) {
	var d struct {
		Header json.RawMessage `json:"request_header"`
		Body   json.RawMessage `json:"request_body"`
	}
	if err := json.Unmarshal(raw, &d); err != nil {
		return nil, nil, fmt.Errorf("the input is not JSON: %w", err)
	}
	if len(d.Header) == 0 || len(d.Body) == 0 {
		return nil, nil, errors.New("the input has no request_header and request_body: use the output of GET /v3/session/{sessionId}/webhook/{webhook_uuid}")
	}
	headerText, err := asText(d.Header)
	if err != nil {
		return nil, nil, fmt.Errorf("request_header: %w", err)
	}
	var fields map[string]any
	if err := json.Unmarshal([]byte(headerText), &fields); err != nil {
		return nil, nil, fmt.Errorf("request_header is not a JSON object: %w", err)
	}
	h := http.Header{}
	for k, v := range fields {
		if s, ok := v.(string); ok {
			h.Set(k, s)
		}
	}
	body, err := asText(d.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("request_body: %w", err)
	}
	return h, []byte(body), nil
}

// asText returns a JSON string's value, or an object or array as its JSON text.
func asText(v json.RawMessage) (string, error) {
	if len(v) > 0 && v[0] == '"' {
		var s string
		if err := json.Unmarshal(v, &s); err != nil {
			return "", err
		}
		return s, nil
	}
	return string(v), nil
}
