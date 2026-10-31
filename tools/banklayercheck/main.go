// Command banklayercheck enforces that only pkg/banklayer reads the licence route (ARCHITECTURE.md 5.2, rule 1).
// Any selector named Route, RouteSponsor or RouteOwnLicence outside pkg/banklayer fails the check. Behaviour that
// depends on the route must be expressed as Capabilities and evaluated by OPA (package forge.banklayer).
package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// forbidden are selector names that only pkg/banklayer may use.
var forbidden = map[string]bool{"Route": true, "RouteSponsor": true, "RouteOwnLicence": true}

func main() {
	root := "."
	if len(os.Args) > 1 {
		root = os.Args[1]
	}
	violations, err := check(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "banklayercheck:", err)
		os.Exit(2)
	}
	for _, v := range violations {
		fmt.Println("VIOLATION:", v)
	}
	if len(violations) > 0 {
		os.Exit(1)
	}
	fmt.Println("banklayercheck: ok")
}

// exempt reports whether a path (relative to root, slash form) may read the route.
func exempt(rel string) bool {
	return rel == "pkg/banklayer" || strings.HasPrefix(rel, "pkg/banklayer/") ||
		rel == "tools/banklayercheck" || strings.HasPrefix(rel, "tools/banklayercheck/")
}

// check returns one message per forbidden selector outside the exempt paths under root.
func check(root string) ([]string, error) {
	var out []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, rerr := filepath.Rel(root, path)
		if rerr != nil {
			return rerr
		}
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			if d.Name() == ".git" || d.Name() == "node_modules" || exempt(rel) {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".go" {
			return nil
		}
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if !ok || !forbidden[sel.Sel.Name] {
				return true
			}
			pos := fset.Position(sel.Sel.Pos())
			out = append(out, fmt.Sprintf("%s:%d reads the licence route (%s); only pkg/banklayer may", pos.Filename, pos.Line, sel.Sel.Name))
			return true
		})
		return nil
	})
	return out, err
}
