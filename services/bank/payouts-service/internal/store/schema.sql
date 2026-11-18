create table if not exists payout_batch (
  batch_id        uuid primary key,
  idempotency_key text not null unique,
  request_hash    bytea not null,
  party_id        uuid not null,
  from_account_id text not null,
  asset           text not null,
  total_minor     bigint not null check (total_minor > 0),
  state           text not null check (state in ('DRAFT', 'RUNNING', 'COMPLETED', 'COMPLETED_WITH_EXCEPTIONS')),
  created_by      text not null,
  approved_by     text not null default '',
  run             integer not null default 0 check (run >= 0),
  created_at      timestamptz not null default now(),
  updated_at      timestamptz not null default now()
);

create table if not exists payout_item (
  batch_id         uuid not null references payout_batch (batch_id),
  item_key         text not null,
  seq              integer not null,
  amount_minor     bigint not null check (amount_minor > 0),
  beneficiary_name text not null,
  account_number   text not null,
  bank_code        text not null,
  reference        text not null,
  state            text not null check (state in ('PENDING', 'SENT', 'SETTLED', 'FAILED', 'UNCERTAIN')),
  payment_id       text not null default '',
  reason           text not null default '',
  updated_at       timestamptz not null default now(),
  primary key (batch_id, item_key),
  unique (batch_id, seq)
);

create index if not exists payout_item_open on payout_item (batch_id) where state in ('PENDING', 'SENT', 'UNCERTAIN');
