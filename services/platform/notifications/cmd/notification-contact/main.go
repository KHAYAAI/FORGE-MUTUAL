// Command notification-contact stores a party's contact address, sealed. The address is read from standard input, so
// it never appears in the process list or the shell history.
//
//	printf '%s' "$ADDRESS" | notification-contact -party <id> -channel EMAIL
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/khayaai/forge-mutual/services/platform/notifications/internal/crypt"
	"github.com/khayaai/forge-mutual/services/platform/notifications/internal/store"
)

func main() {
	party := flag.String("party", "", "party id")
	channel := flag.String("channel", "", "EMAIL or SMS")
	flag.Parse()
	if err := run(*party, *channel); err != nil {
		fmt.Fprintln(os.Stderr, "notification-contact:", err)
		os.Exit(1)
	}
	fmt.Println("contact stored")
}

func run(party, channel string) error {
	if party == "" || (channel != "EMAIL" && channel != "SMS") {
		return fmt.Errorf("usage: -party <id> -channel EMAIL|SMS, address on stdin")
	}
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		return fmt.Errorf("DATABASE_URL is required")
	}
	key, err := crypt.ParseKey(os.Getenv("NOTIFICATIONS_KEY"))
	if err != nil {
		return fmt.Errorf("NOTIFICATIONS_KEY must be a base64 32-byte key")
	}
	raw, err := io.ReadAll(io.LimitReader(os.Stdin, 1024))
	if err != nil {
		return err
	}
	address := strings.TrimSpace(string(raw))
	if address == "" {
		return fmt.Errorf("no address on standard input")
	}
	sealed, err := key.Seal([]byte(address))
	if err != nil {
		return err
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return err
	}
	defer pool.Close()
	if err := store.Open(ctx, pool); err != nil {
		return err
	}
	return store.SetContact(ctx, pool, party, channel, sealed)
}
