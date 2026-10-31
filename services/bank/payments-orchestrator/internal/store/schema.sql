-- Payment records (ARCHITECTURE.md 9, 13). A row is updated at each step, and its state drives the daily
-- velocity that policy reads: money on hold or settled today counts against the daily limit.
create table if not exists payment (
  payment_id          uuid        primary key,
  party_id            uuid        not null,
  from_account_id     text        not null,
  asset               text        not null,
  amount_minor        bigint      not null check (amount_minor > 0),
  state               text        not null,
  reason              text        not null default '',
  rail_status         text        not null default '',
  reserve_transfer_id text,
  initiated_by        text        not null,
  created_at          timestamptz not null default now(),
  updated_at          timestamptz not null default now()
);

create index if not exists payment_velocity on payment (party_id, asset, created_at)
  where state in ('RESERVED', 'SETTLED', 'REPAIR_REQUIRED');
