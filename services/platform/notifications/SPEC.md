# notifications (services/platform/notifications)

Entity: PLAT. Architecture: ARCHITECTURE.md 6.1 (the notifications row). v1 scope: email, SMS and push. This build does
email and SMS messages for transactional events only; push is not built.

## What it does

- **Events in.** Consumes four topics through `pkg/eventbus`: payment settled and failed, party status changed, and consent
  revoked. Each event is rendered and enqueued in the same transaction that records it, so an event produces its messages
  exactly once.
- **Messages.** A settled or failed payment goes by email and SMS; the other events by email. Each message carries the
  amount and the reference, never an account number. A failed payment says that no money has left the account.
- **Contacts.** Addresses are sealed with AES-256-GCM before they are stored. The key comes from `NOTIFICATIONS_KEY`, from
  the secret store. The address is opened only when a message is sent, and it is never logged.
- **Missing contacts are recorded.** A party with no address on a channel gets a SKIPPED message with the reason. The gap
  is visible; nothing is dropped silently.
- **Delivery.** The sender claims pending messages, retries a failed delivery up to five attempts, then marks the message
  FAILED and keeps it visible.
- **Transactional only.** Marketing messages are not sent. A consent check is not needed for these messages, because they
  are about the customer's own account or about a consent they just changed.

## Contacts

Set a contact with the CLI. The address is read from standard input, so it never appears in the process list:

    printf '%s' "$ADDRESS" | notification-contact -party <id> -channel EMAIL

## Limits, stated

- **No real provider.** Messages go to a development file sink, which holds plaintext addresses and must not run in a
  shared environment. SMTP and SMS providers are not chosen (VERIFY). The binary refuses to start without a sink path.
- **No push channel.**
- **No contact API.** Contacts are set by the CLI. Party onboarding does not yet collect them.
- **Templates are in code.** Localisation is not built.

## Configuration

| Variable | Meaning |
|---|---|
| `DATABASE_URL` | Postgres (required) |
| `KAFKA_BROKERS` | Broker list (required) |
| `NOTIFICATIONS_KEY` | Base64 32-byte key for sealing addresses (required, from the secret store) |
| `NOTIFICATIONS_SINK_FILE` | Development sink file (required until a provider is configured) |
| `NOTIFICATIONS_SEND_INTERVAL` | Delivery pass interval (default 5s) |
| `NOTIFICATIONS_ADDR` | Health listener (default `:8097`) |

## Tests

`make test-notifications`. Database tests need `NOTIFICATIONS_TEST_DSN`.
