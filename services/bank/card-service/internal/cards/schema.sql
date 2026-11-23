-- Card issuing. Card numbers are never stored: the processor holds them. A card here is a token, the last four digits and
-- the expiry. Amounts are rand cents.
create table if not exists card (
  card_id         text primary key,
  party_id        text not null,
  account_id      text not null,
  idempotency_key text not null,
  token           text not null unique,
  last4           text not null,
  expiry_month    int  not null,
  expiry_year     int  not null,
  label           text not null,
  status          text not null check (status in ('ACTIVE','FROZEN','CLOSED')),
  per_txn_limit   bigint not null check (per_txn_limit > 0),
  daily_limit     bigint not null check (daily_limit > 0),
  created_at      timestamptz not null,
  unique (party_id, idempotency_key)
);
create index if not exists card_party on card (party_id);

create table if not exists card_auth (
  auth_id        text primary key,            -- the processor's authorisation id: replays return the same answer
  card_id        text not null references card,
  amount_minor   bigint not null check (amount_minor > 0),
  currency       text not null,
  merchant       text not null,
  mcc            text not null,
  country        text not null,
  status         text not null check (status in ('PENDING','HELD','CAPTURED','REVERSED','REFUNDED','DECLINED','EXPIRED','EXCEPTION')),
  decline_reason text not null default '',
  hold_transfer_id text not null default '',
  clearing_id    text not null default '',
  captured_minor bigint not null default 0,
  refunded_minor bigint not null default 0,
  exception      text not null default '',    -- set for EXCEPTION: what an operator must look at
  created_at     timestamptz not null,
  updated_at     timestamptz not null
);
create index if not exists card_auth_card on card_auth (card_id, created_at desc);
create index if not exists card_auth_open on card_auth (status, created_at) where status in ('PENDING','HELD');

create table if not exists card_refund (
  refund_id    text primary key,
  auth_id      text not null references card_auth,
  amount_minor bigint not null check (amount_minor > 0),
  at           timestamptz not null
);
