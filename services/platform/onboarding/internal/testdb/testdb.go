// Package testdb gives each database-backed test its own database. Test packages run in parallel, so two of them
// sharing one database race on the same tables; a fresh database per test removes the race.
package testdb

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

// Scratch creates a new database on the server named by the base DSN in env, and returns its DSN. The database is
// dropped when the test ends. The test is skipped when env is not set.
func Scratch(t *testing.T, env string) string {
	t.Helper()
	base := os.Getenv(env)
	if base == "" {
		t.Skip(env + " not set")
	}
	suffix := make([]byte, 6)
	_, _ = rand.Read(suffix)
	name := "onb_t_" + hex.EncodeToString(suffix) + "_" + time.Now().Format("150405")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	admin, err := pgx.Connect(ctx, base)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close(ctx)
	if _, err := admin.Exec(ctx, `create database "`+name+`"`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		c, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		conn, err := pgx.Connect(c, base)
		if err != nil {
			return
		}
		defer conn.Close(c)
		_, _ = conn.Exec(c, `drop database if exists "`+name+`" with (force)`)
	})

	u, err := url.Parse(base)
	if err != nil {
		t.Fatal(err)
	}
	u.Path = "/" + name
	return u.String()
}
