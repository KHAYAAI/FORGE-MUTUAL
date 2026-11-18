-- Schema for the ledger gateway (ARCHITECTURE.md 7.3, 7.6, 7.8).
-- Every statement is idempotent so the migration can run at every start.

create table if not exists idempotency_record (
  caller           text        not null,
  idempotency_key  text        not null,
  request_hash     bytea       not null,
  status           text        not null check (status in ('PENDING', 'COMMITTED')),
  ledger_ids       text[]      not null,
  result           bytea,
  created_at       timestamptz not null default now(),
  updated_at       timestamptz not null default now(),
  primary key (caller, idempotency_key)
);

create table if not exists account_directory (
  account_id      text        primary key,        -- 32 hex characters
  entity_code     text        not null,
  owner_party_id  text,
  asset           text        not null,
  ledger          bigint      not null,
  code            integer     not null,
  product_id      text,
  insured         boolean     not null,
  status          text        not null default 'open',
  created_at      timestamptz not null default now()
);

create table if not exists outbox (
  seq           bigserial   primary key,
  event_id      uuid        not null unique,
  topic         text        not null,
  event_key     bytea       not null,
  payload       bytea       not null,
  created_at    timestamptz not null default now(),
  published_at  timestamptz
);

create index if not exists outbox_unpublished on outbox (seq) where published_at is null;
