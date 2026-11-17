-- deposits-service. Balances live in the ledger; this holds what the ledger does not: which accounts are deposit
-- accounts, each day's accrual, each month's interest payment, and notices.
create table if not exists deposit_account (
  account_id   text        primary key,
  party_id     text        not null,
  product_id   text        not null,
  opened_at    timestamptz not null default now(),
  unique (party_id, product_id)
);

-- One row per account per day. The day's interest is computed once, on the balance read when the day is accrued.
create table if not exists accrual (
  account_id    text   not null references deposit_account (account_id),
  day           date   not null,
  balance_minor bigint not null check (balance_minor >= 0),
  rate_bps      int    not null check (rate_bps between 0 and 10000),
  accrued_nano  bigint not null check (accrued_nano >= 0),
  late          boolean not null default false, -- accrued after the day, on the balance at that time
  primary key (account_id, day)
);

-- One row per account per month. Written PENDING before the ledger is asked, so a crash is finished with the same key.
create table if not exists interest_payment (
  account_id      text   not null references deposit_account (account_id),
  period          text   not null, -- YYYY-MM
  accrued_nano    bigint not null,
  carry_in_nano   bigint not null,
  paid_minor      bigint not null check (paid_minor >= 0),
  carry_out_nano  bigint not null check (carry_out_nano >= 0 and carry_out_nano < 1000000000),
  status          text   not null check (status in ('PENDING', 'PAID')),
  primary key (account_id, period)
);

create table if not exists notice (
  notice_id       text        primary key,
  account_id      text        not null references deposit_account (account_id),
  party_id        text        not null,
  to_account_id   text        not null,
  amount_minor    bigint      not null check (amount_minor > 0),
  given_at        timestamptz not null,
  due_on          date        not null,
  status          text        not null check (status in ('PENDING', 'PAID', 'CANCELLED', 'FAILED')),
  idempotency_key text        not null unique
);
create index if not exists notice_due on notice (due_on) where status = 'PENDING';

-- Deposits and withdrawals by the client's idempotency key, scoped to the party.
create table if not exists move (
  idempotency_key text primary key,
  request_hash    text not null,
  state           text not null,
  notice_id       text not null default ''
);
