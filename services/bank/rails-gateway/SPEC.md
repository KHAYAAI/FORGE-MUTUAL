# SPEC: rails-gateway

Status: built and tested locally. The only rail is the simulator. A sponsor adapter is not implemented.
Entity: BANK. Sections referenced: ARCHITECTURE.md 12.1, 6.2, P1-07B.

## 1. Purpose

Sends payment instructions to a payment rail and reports their status. The rail boundary is `internal/rail.Rail`.
`rail.Sim` implements it, with deterministic scenarios so every outcome can be tested.

## 2. Interface

ConnectRPC `forge.rails.v1.RailService`:

- `Initiate`: idempotent by `instruction_id`. The same instruction returns the current status. A changed instruction
  under the same ID returns `ALREADY_EXISTS`.
- `Status`: one poll. Unknown instructions return `NOT_FOUND`.

Statuses: ACCEPTED (taken, no outcome), PENDING, SETTLED, REJECTED, RETURNED. SETTLED, REJECTED and RETURNED are final.

## 3. Rules

- An instruction the rail cannot accept is refused on initiation (status REJECTED): EFT carries ZAR only, the account
  number must be 6 to 16 digits, the amount must be a positive integer of minor units, and the beneficiary name and
  bank code must be present.
- Scenarios (simulation only). `settle_after:N`, `reject_after:N`, `return_after:N`, `stall` and
  `reject_at_initiation`. A scenario becomes final on the (N+1)th status poll. `stall` never becomes final.
- Scenarios are refused unless `RAILS_SIMULATION=true`. A scenario sent to a production rail is `INVALID_ARGUMENT`.
- The caller allow-list applies (`pkg/callerauth`). Deny by default.

## 4. Tests

- `internal/rail`: default and scripted scenarios, scenario grammar, refusal of scenarios on a production rail,
  reject and return timing, stall, idempotency and conflicts, instruction validation, unknown instructions.
- `internal/service`: the wire error codes.

## 5. Open items

- **No sponsor adapter.** The sponsor's EFT interface (format, certification, statement schema) is VERIFY-SPONSOR.
  Until it exists, nothing reaches a real bank.
- **Instruction state is in memory.** A restart loses the instructions. The simulator tolerates that; a real rail
  adapter must keep instructions durably, because idempotency is the only protection against a double payment.
- **Bank code format.** The bank code is accepted as 1 to 12 alphanumeric characters. The real format is a VERIFY item.
