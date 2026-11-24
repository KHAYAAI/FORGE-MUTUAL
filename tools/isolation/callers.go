package main

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// The caller allow-lists say which service may call which method of which other service. They are deny by default,
// and they are only worth something if each list is as short as it can be: a list that grants every method to every
// caller authenticates callers and then authorises nothing. checkCallers holds the lists to least privilege. A grant
// is a violation unless the caller's own code calls that method of that service.
//
// It reads <root>/deploy/compose/config/callers/<target>.json, the only allow-lists in the repository. For each grant
// it looks, in the caller's source, for a call to a method of that name in a file that imports the target's API
// package. That is a search by name, not by type, so it can miss an unused grant when the same file calls a method of
// the same name on another service. It cannot report a grant that is used.

const apiPrefix = "github.com/khayaai/forge-mutual/pkg/api/forge/"

// callerSource is where each caller's code lives, relative to the repository root. Shared packages that make calls on a
// service's behalf are listed with the services that use them.
var callerSource = map[string][]string{
	"svc-backoffice-api":    {"services/shared/backoffice-api"},
	"svc-compliance":        {"services/shared/compliance-engine"},
	"svc-consolidated-view": {"services/platform/consolidated-view"},
	"svc-core-bridge":       {"services/bank/core-bridge"},
	"svc-fx":                {"services/bank/fx-service", "pkg/systemaccounts"},
	"svc-deposits":          {"services/bank/deposits-service", "pkg/systemaccounts"},
	"svc-lending":           {"services/bank/lending-service", "pkg/systemaccounts"},
	"svc-acquiring":         {"services/bank/acquiring-service", "pkg/systemaccounts"},
	"svc-cards":             {"services/bank/card-service", "pkg/systemaccounts"},
	"svc-partner-offers":    {"services/platform/partner-offers", "pkg/systemaccounts"},
	"svc-onboarding":        {"services/platform/onboarding"},
	"svc-payments":          {"services/bank/payments-orchestrator", "pkg/systemaccounts"},
	"svc-payouts":           {"services/bank/payouts-service"},
	"svc-reconciliation":    {"services/bank/reconciliation"},
	"svc-treasury-risk":     {"services/bank/treasury-risk"},
	"svc-workspace":         {"services/platform/workspace-api"},
}

// harnessSource is where a test harness's calls live. A harness is not Go: its grant is used if the method's name
// appears in the file.
var harnessSource = map[string]string{
	"svc-stack-journeys": "deploy/compose/journeys/seed.cjs",
}

func checkCallers(root string) ([]string, error) {
	dir := filepath.Join(root, "deploy", "compose", "config", "callers")
	files, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, nil // a tree with no allow-lists has nothing to hold to account
	}
	sort.Strings(files)
	called := map[string]map[string]map[string]bool{} // caller -> target -> method names called
	var out []string
	for _, file := range files {
		target := strings.TrimSuffix(filepath.Base(file), ".json")
		raw, err := os.ReadFile(file)
		if err != nil {
			return nil, err
		}
		var grants map[string][]string
		if err := json.Unmarshal(raw, &grants); err != nil {
			return nil, fmt.Errorf("%s: %w", file, err)
		}
		callers := make([]string, 0, len(grants))
		for c := range grants {
			callers = append(callers, c)
		}
		sort.Strings(callers)
		for _, caller := range callers {
			methods := grants[caller]
			if len(methods) == 0 {
				out = append(out, fmt.Sprintf("%s: %s is listed with no methods; remove it", file, caller))
				continue
			}
			if h, ok := harnessSource[caller]; ok {
				text, err := os.ReadFile(filepath.Join(root, h))
				if err != nil {
					return nil, err
				}
				for _, m := range methods {
					if !strings.Contains(string(text), `"`+m+`"`) {
						out = append(out, fmt.Sprintf("%s: %s is granted %s, which %s never calls", file, caller, m, h))
					}
				}
				continue
			}
			dirs, ok := callerSource[caller]
			if !ok {
				out = append(out, fmt.Sprintf("%s: unknown caller %s; add its source directory to tools/isolation/callers.go", file, caller))
				continue
			}
			if called[caller] == nil {
				called[caller], err = callsByTarget(root, dirs)
				if err != nil {
					return nil, err
				}
			}
			for _, m := range methods {
				if !called[caller][target][m] {
					out = append(out, fmt.Sprintf("%s: %s is granted %s, which its code (%s) never calls on %s",
						file, caller, m, strings.Join(dirs, ", "), target))
				}
			}
		}
	}
	return out, nil
}

// callsByTarget returns, for each API package the code under dirs imports, the names of the methods called in the
// files that import it. Test files are left out: a grant must be justified by what the service does, not by its tests.
func callsByTarget(root string, dirs []string) (map[string]map[string]bool, error) {
	out := map[string]map[string]bool{}
	for _, d := range dirs {
		err := filepath.WalkDir(filepath.Join(root, d), func(path string, e fs.DirEntry, err error) error {
			if err != nil || e.IsDir() || filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
				return err
			}
			f, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
			if err != nil {
				return err
			}
			var targets []string
			for _, imp := range f.Imports {
				ip := strings.Trim(imp.Path.Value, `"`)
				if strings.HasPrefix(ip, apiPrefix) {
					targets = append(targets, strings.SplitN(strings.TrimPrefix(ip, apiPrefix), "/", 2)[0])
				}
			}
			if len(targets) == 0 {
				return nil
			}
			ast.Inspect(f, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
					for _, t := range targets {
						if out[t] == nil {
							out[t] = map[string]bool{}
						}
						out[t][sel.Sel.Name] = true
					}
				}
				return true
			})
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}
