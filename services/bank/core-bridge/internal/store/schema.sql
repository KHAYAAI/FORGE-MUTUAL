-- Core bridge state (ARCHITECTURE.md 8). A journal is queued in the same transaction as the ledger event that caused
-- it, and posted to Fineract by the poster. Nothing is posted twice: the reference is the ledger transfer id.
create table if not exists cb_account (
  account_id   text        primary key,
  ledger_code  integer     not null,
  asset        text        not null,
  seen_at      timestamptz not null default now()
);

create table if not exists cb_journal (
  transfer_id     text        primary key,
  asset           text        not null,
  amount_minor    bigint      not null check (amount_minor > 0),
  debit_code      integer     not null,
  credit_code     integer     not null,
  occurred_at     timestamptz not null,
  status          text        not null check (status in ('PENDING', 'POSTED', 'FAILED')),
  attempts        integer     not null default 0,
  last_error      text        not null default '',
  fineract_txn    text        not null default '',
  created_at      timestamptz not null default now(),
  posted_at       timestamptz
);
create index if not exists cb_journal_pending on cb_journal (created_at) where status = 'PENDING';

create table if not exists cb_recon_run (
  run_id       bigserial   primary key,
  finished_at  timestamptz not null,
  checked      integer     not null,
  breaks       integer     not null,
  skipped      text        not null default '',
  detail       text        not null default ''
);
