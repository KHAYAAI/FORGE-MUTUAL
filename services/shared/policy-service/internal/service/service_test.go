package service

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"os"
	"path/filepath"
	"testing"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"

	eventsv1 "github.com/khayaai/forge-mutual/pkg/api/forge/events/v1"
	policyv1 "github.com/khayaai/forge-mutual/pkg/api/forge/policy/v1"
	"github.com/khayaai/forge-mutual/pkg/outbox"
	"github.com/khayaai/forge-mutual/services/shared/policy-service/internal/bundle"
	"github.com/khayaai/forge-mutual/services/shared/policy-service/internal/engine"
)

// These tests need PostgreSQL: set POLICY_TEST_DSN to a throwaway database.

func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("POLICY_TEST_DSN")
	if dsn == "" {
		t.Skip("POLICY_TEST_DSN not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := outbox.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `truncate outbox`); err != nil {
		t.Fatal(err)
	}
	return pool
}

// compiledBundle signs a copy of the repository policy and compiles it, as the service does at start.
func compiledBundle(t *testing.T) *engine.Engine {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	src, _ := filepath.Abs("../../../../../policy")
	copyTree(t, src, dir)
	if err := bundle.Sign(dir, "svc-test", priv); err != nil {
		t.Fatal(err)
	}
	b, err := bundle.Load(dir, []ed25519.PublicKey{priv.Public().(ed25519.PublicKey)}, false)
	if err != nil {
		t.Fatal(err)
	}
	e, err := engine.Compile(context.Background(), b)
	if err != nil {
		t.Fatal(err)
	}
	return e
}

func copyTree(t *testing.T, from, to string) {
	t.Helper()
	err := filepath.WalkDir(from, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(from, path)
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		target := filepath.Join(to, rel)
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return os.WriteFile(target, b, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
}

func evaluate(t *testing.T, s *Service, point string, input map[string]any) *policyv1.EvaluateResponse {
	t.Helper()
	st, err := structpb.NewStruct(input)
	if err != nil {
		t.Fatal(err)
	}
	res, err := s.Evaluate(context.Background(), connect.NewRequest(&policyv1.EvaluateRequest{
		DecisionPoint: point, EntityCode: "BANK", RequestId: "req-1", Input: st,
	}))
	if err != nil {
		t.Fatalf("evaluate %s: %v", point, err)
	}
	return res.Msg
}

func auditRows(t *testing.T, pool *pgxpool.Pool) [][]byte {
	t.Helper()
	rows, err := pool.Query(context.Background(), `select payload from outbox order by seq`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out [][]byte
	for rows.Next() {
		var p []byte
		if err := rows.Scan(&p); err != nil {
			t.Fatal(err)
		}
		out = append(out, p)
	}
	return out
}

func TestEveryDecisionIsAudited(t *testing.T) {
	pool := testPool(t)
	s := New(pool, compiledBundle(t))
	resp := evaluate(t, s, "sector", map[string]any{"sector_code": "software_saas"})
	if !resp.Allow || resp.BundleVersion != "svc-test" || resp.DecisionId == "" {
		t.Fatalf("response = %+v", resp)
	}
	rows := auditRows(t, pool)
	if len(rows) != 1 {
		t.Fatalf("audit rows = %d, want 1", len(rows))
	}
	var ev eventsv1.PolicyDecision
	if err := proto.Unmarshal(rows[0], &ev); err != nil {
		t.Fatal(err)
	}
	if ev.DecisionId != resp.DecisionId || !ev.Allow || ev.DecisionPoint != "sector" || ev.RequestId != "req-1" {
		t.Fatalf("audit event = %+v", &ev)
	}
	if len(ev.InputSha256) != 32 {
		t.Fatal("input must be recorded as a digest, not stored")
	}
}

func TestDenialsAreAuditedToo(t *testing.T) {
	pool := testPool(t)
	s := New(pool, compiledBundle(t))
	resp := evaluate(t, s, "sector", map[string]any{"sector_code": "defence"})
	if resp.Allow || resp.Reason != "sector_not_eligible" {
		t.Fatalf("defence must be denied: %+v", resp)
	}
	if len(auditRows(t, pool)) != 1 {
		t.Fatal("denial was not audited")
	}
}

func TestNoBundleDeniesAndIsAudited(t *testing.T) {
	pool := testPool(t)
	s := New(pool, nil)
	resp := evaluate(t, s, "limits", map[string]any{})
	if resp.Allow || resp.Reason != "policy_bundle_unavailable" || resp.BundleVersion != "none" {
		t.Fatalf("no bundle must deny: %+v", resp)
	}
	if len(auditRows(t, pool)) != 1 {
		t.Fatal("no-bundle denial was not audited")
	}
}

func TestUnknownPointIsRejectedWithoutAudit(t *testing.T) {
	pool := testPool(t)
	s := New(pool, compiledBundle(t))
	_, err := s.Evaluate(context.Background(), connect.NewRequest(&policyv1.EvaluateRequest{DecisionPoint: "payments", EntityCode: "BANK"}))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("want InvalidArgument, got %v", err)
	}
	if len(auditRows(t, pool)) != 0 {
		t.Fatal("rejected request must not write an audit row")
	}
}

func TestDecisionWithoutAuditIsNotReturned(t *testing.T) {
	pool := testPool(t)
	s := New(pool, compiledBundle(t))
	pool.Close() // the audit write can no longer succeed
	st, _ := structpb.NewStruct(map[string]any{"sector_code": "software_saas"})
	_, err := s.Evaluate(context.Background(), connect.NewRequest(&policyv1.EvaluateRequest{
		DecisionPoint: "sector", EntityCode: "BANK", Input: st,
	}))
	if connect.CodeOf(err) != connect.CodeUnavailable {
		t.Fatalf("an unaudited decision must not be returned; got %v", err)
	}
}
