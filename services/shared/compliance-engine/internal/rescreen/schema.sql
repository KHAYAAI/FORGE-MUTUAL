-- Sanctions re-screening runs: one per list version per day.
create table if not exists rescreen_run (
  run_key      text        primary key,
  list_version text        not null,
  started_at   timestamptz not null,
  finished_at  timestamptz,
  screened     int         not null default 0,
  matched      int         not null default 0
);
