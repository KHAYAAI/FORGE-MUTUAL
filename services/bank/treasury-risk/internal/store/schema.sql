-- Treasury snapshots and alerts (ARCHITECTURE.md 13). An alert is raised once and cleared once; a threshold that stays
-- crossed updates the open alert rather than raising a new one.
create table if not exists treasury_snapshot (
  asset            text        not null,
  at               timestamptz not null,
  deposits         bigint      not null,
  largest          bigint      not null,
  largest_account  text        not null default '',
  pool             bigint      not null,
  suspense         bigint      not null,
  primary key (asset, at)
);

create table if not exists treasury_alert (
  alert_id       text        primary key,
  kind           text        not null,
  asset          text        not null,
  severity       text        not null,
  state          text        not null check (state in ('RAISED', 'RESOLVED')),
  detail         text        not null,
  metric_bps     bigint      not null default 0,
  threshold_bps  bigint      not null default 0,
  raised_at      timestamptz not null,
  resolved_at    timestamptz
);
create unique index if not exists treasury_alert_one_raised on treasury_alert (asset, kind) where state = 'RAISED';
