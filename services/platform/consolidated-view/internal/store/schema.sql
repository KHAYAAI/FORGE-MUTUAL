-- Consolidated read model (ARCHITECTURE.md 6.1, 7.9). Built from ledger events; reconciliation makes the ledger
-- authoritative. An account is invisible to every party until a reconciliation has named its owner (fail closed).
create table if not exists cv_account (
  account_id      text        primary key,
  entity          text        not null,
  asset           text        not null,
  ledger          bigint      not null default 0,
  code            integer     not null default 0,
  owner_party_id  text        not null default '',
  product_id      text        not null default '',
  insured         boolean     not null default false,
  seen_at         timestamptz not null default now()
);

create table if not exists cv_balance (
  account_id    text        primary key references cv_account (account_id),
  debit_minor   bigint      not null default 0,
  credit_minor  bigint      not null default 0,
  source        text        not null check (source in ('EVENT', 'LEDGER')),
  as_of         timestamptz not null default now()
);

create table if not exists cv_activity (
  seq           bigserial   primary key,
  account_id    text        not null references cv_account (account_id),
  transfer_id   text        not null,
  direction     text        not null check (direction in ('IN', 'OUT')),
  amount_minor  bigint      not null check (amount_minor > 0),
  asset         text        not null,
  code          integer     not null,
  at            timestamptz not null,
  unique (account_id, transfer_id, direction)
);
create index if not exists cv_activity_account on cv_activity (account_id, seq);

create table if not exists cv_recon_run (
  run_id           bigserial   primary key,
  finished_at      timestamptz not null,
  accounts_checked integer     not null,
  drifted          integer     not null
);
