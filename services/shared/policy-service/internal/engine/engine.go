// Package engine compiles a verified policy bundle and answers decision points.
//
// Every decision point is a Rego rule `decision` in package forge.<point>, returning an object
// {allow, stepup, reason}. Anything the engine cannot read as that object is a denial, so a rule
// that is undefined, errors or returns the wrong shape never allows (fail closed).
package engine

import (
	"context"
	"fmt"

	"github.com/open-policy-agent/opa/rego"
	"github.com/open-policy-agent/opa/storage/inmem"

	"github.com/khayaai/forge-mutual/services/shared/policy-service/internal/bundle"
)

// points maps each decision point to its query.
var points = map[string]string{
	"limits": "data.forge.limits.decision",
	"assets": "data.forge.assets.decision",
	"entity": "data.forge.entity.decision",
	"recon":  "data.forge.recon.decision",
	"sector": "data.forge.sector.decision",
}

// Points returns the decision point names this engine serves.
func Points() []string {
	out := make([]string, 0, len(points))
	for p := range points {
		out = append(out, p)
	}
	return out
}

// Known reports whether point is a decision point.
func Known(point string) bool {
	_, ok := points[point]
	return ok
}

// Decision is the outcome of one evaluation.
type Decision struct {
	Allow  bool
	StepUp bool
	Reason string
}

// Denied returns a deny decision with a reason.
func Denied(reason string) Decision { return Decision{Reason: reason} }

// Engine holds the prepared queries for one bundle version.
type Engine struct {
	version string
	queries map[string]rego.PreparedEvalQuery
}

// Compile prepares every decision point against the bundle.
func Compile(ctx context.Context, b *bundle.Bundle) (*Engine, error) {
	store := inmem.NewFromObject(b.Data)
	e := &Engine{version: b.Version, queries: make(map[string]rego.PreparedEvalQuery, len(points))}
	for point, query := range points {
		opts := []func(*rego.Rego){rego.Query(query), rego.Store(store)}
		for name, src := range b.Modules {
			opts = append(opts, rego.Module(name, src))
		}
		pq, err := rego.New(opts...).PrepareForEval(ctx)
		if err != nil {
			return nil, fmt.Errorf("engine: compile %s: %w", point, err)
		}
		e.queries[point] = pq
	}
	return e, nil
}

// Version returns the bundle version the engine was compiled from.
func (e *Engine) Version() string { return e.version }

// Evaluate runs one decision point. An error means the decision could not be read; callers must
// treat it as a denial.
func (e *Engine) Evaluate(ctx context.Context, point string, input map[string]any) (Decision, error) {
	pq, ok := e.queries[point]
	if !ok {
		return Decision{}, fmt.Errorf("engine: unknown decision point %q", point)
	}
	rs, err := pq.Eval(ctx, rego.EvalInput(input))
	if err != nil {
		return Decision{}, fmt.Errorf("engine: evaluate %s: %w", point, err)
	}
	if len(rs) != 1 || len(rs[0].Expressions) != 1 {
		return Decision{}, fmt.Errorf("engine: %s: unexpected result shape", point)
	}
	obj, ok := rs[0].Expressions[0].Value.(map[string]any)
	if !ok {
		return Decision{}, fmt.Errorf("engine: %s: decision is not an object", point)
	}
	allow, ok1 := obj["allow"].(bool)
	stepup, ok2 := obj["stepup"].(bool)
	reason, ok3 := obj["reason"].(string)
	if !ok1 || !ok2 || !ok3 {
		return Decision{}, fmt.Errorf("engine: %s: decision lacks allow, stepup or reason", point)
	}
	return Decision{Allow: allow, StepUp: stepup, Reason: reason}, nil
}
