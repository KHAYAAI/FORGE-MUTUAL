-- Reconciliation store (ARCHITECTURE.md section 13). Runs and breaks are written in one transaction.

create table if not exists recon_run (
  run_id         text        primary key,
  asset          text        not null,
  started_at     timestamptz not null,
  finished_at    timestamptz not null,
  outcome        text        not null check (outcome in ('CONCLUSIVE', 'INCONCLUSIVE')),
  all_matched    boolean     not null,
  failed_checks  text[]      not null default '{}',
  checks         jsonb       not null default '[]'
);
create index if not exists recon_run_asset_time on recon_run (asset, finished_at desc);

create table if not exists recon_break (
  break_id          text        primary key,
  asset             text        not null,
  check_name        text        not null,
  difference_minor  text        not null,
  detail            text        not null,
  status            text        not null check (status in ('OPEN', 'CLEARED')),
  opened_at         timestamptz not null,
  last_seen_at      timestamptz not null,
  cleared_at        timestamptz,
  investigation_note text       not null default '',
  investigated_by   text        not null default '',
  investigated_at   timestamptz
);
-- At most one open break per asset and check. A repeat mismatch updates it instead of adding another.
create unique index if not exists recon_break_one_open on recon_break (asset, check_name) where status = 'OPEN';

-- Append-only investigation notes. recon_break keeps the latest, this table keeps all of them.
create table if not exists recon_investigation (
  seq         bigserial   primary key,
  break_id    text        not null references recon_break (break_id),
  actor       text        not null,
  note        text        not null check (length(note) between 1 and 4000),
  created_at  timestamptz not null
);
