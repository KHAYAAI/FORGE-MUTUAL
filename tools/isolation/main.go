// Command isolation enforces two boundaries that code review alone would let slip.
//
// Legal entities (ARCHITECTURE.md section 5): a package under services/<entity>/ may import only its own entity, or
// services/shared. Any other cross-entity import fails the check.
//
// Callers (callers.go): a service is allowed to call only the methods of another service that its code really calls.
package main

import (
	"fmt"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Entities are the top-level directories under services/.
var entities = map[string]bool{"platform": true, "bank": true, "dasset": true, "mfi": true, "shared": true}

const modulePrefix = "github.com/khayaai/forge-mutual/services/"

func main() {
	root := "."
	if len(os.Args) > 1 {
		root = os.Args[1]
	}
	violations, err := check(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "isolation:", err)
		os.Exit(2)
	}
	grants, err := checkCallers(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "callers:", err)
		os.Exit(2)
	}
	for _, v := range append(violations, grants...) {
		fmt.Println("VIOLATION:", v)
	}
	if len(violations)+len(grants) > 0 {
		os.Exit(1)
	}
	fmt.Println("isolation: ok")
	fmt.Println("callers: ok (every grant in deploy/compose/config/callers is a call the caller's code makes)")
}

// check returns one message per forbidden cross-entity import under root/services.
func check(root string) ([]string, error) {
	var out []string
	base := filepath.Join(root, "services")
	err := filepath.WalkDir(base, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Ext(path) != ".go" {
			return err
		}
		own := entityOf(path, base)
		f, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, imp := range f.Imports {
			ip := strings.Trim(imp.Path.Value, `"`)
			if !strings.HasPrefix(ip, modulePrefix) {
				continue
			}
			target := strings.SplitN(strings.TrimPrefix(ip, modulePrefix), "/", 2)[0]
			if target != own && target != "shared" && entities[target] {
				out = append(out, fmt.Sprintf("%s imports %s (entity %q may not import %q)", path, ip, own, target))
			}
		}
		return nil
	})
	return out, err
}

// entityOf returns the entity directory name for a file under base.
func entityOf(path, base string) string {
	rel, err := filepath.Rel(base, path)
	if err != nil {
		return ""
	}
	return strings.SplitN(rel, string(filepath.Separator), 2)[0]
}
