// Command forge-audit verifies the audit log. It walks the whole chain and checks every anchor. Exit status 0 means
// the log is intact; 1 means it is broken or rewritten; 2 means the command could not run.
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/khayaai/forge-mutual/services/shared/audit-service/internal/anchor"
	"github.com/khayaai/forge-mutual/services/shared/audit-service/internal/store"
)

func main() {
	os.Exit(run(context.Background(), os.Args[1:], os.Getenv("DATABASE_URL"), os.Stdout, os.Stderr))
}

func run(ctx context.Context, args []string, dsn string, out, errOut *os.File) int {
	if len(args) < 1 || args[0] != "verify" {
		fmt.Fprintln(errOut, "usage: forge-audit verify --anchors <file>   (DATABASE_URL must be set)")
		return 2
	}
	anchorPath := ""
	for i := 1; i < len(args); i++ {
		if args[i] == "--anchors" && i+1 < len(args) {
			anchorPath = args[i+1]
			i++
		}
	}
	if dsn == "" {
		fmt.Fprintln(errOut, "forge-audit: DATABASE_URL is required")
		return 2
	}
	if anchorPath == "" {
		fmt.Fprintln(errOut, "forge-audit: --anchors is required; without anchors a full rewrite cannot be detected")
		return 2
	}
	anchors, err := anchor.Read(anchorPath)
	if err != nil {
		fmt.Fprintf(errOut, "forge-audit: read anchors: %v\n", err)
		return 2
	}
	st, err := store.Open(ctx, dsn)
	if err != nil {
		fmt.Fprintf(errOut, "forge-audit: %v\n", err)
		return 2
	}
	defer st.Close()

	n, head, err := st.VerifyAll(ctx)
	if err != nil {
		fmt.Fprintf(errOut, "BROKEN: %v\n", err)
		return 1
	}
	if err := anchor.Check(ctx, st, anchors); err != nil {
		fmt.Fprintf(errOut, "BROKEN: %v\n", err)
		return 1
	}
	fmt.Fprintf(out, "ok: %d entries verified, %d anchors matched, head %x\n", n, len(anchors), head[:8])
	return 0
}
