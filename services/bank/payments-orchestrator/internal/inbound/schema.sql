-- Inbound credits (ARCHITECTURE.md 9, 13). A credit is recorded when the bank reports it, then booked to the
-- customer its reference matches, or to SUSPENSE. PENDING credits are in flight: the bank has them, the books do not.
create table if not exists inbound_reference (
  reference         text        primary key,
  asset             text        not null,
  account_id        text        not null,
  party_id          text        not null,
  registered_at     timestamptz not null default now()
);

create table if not exists inbound_credit (
  asset             text        not null,
  movement_id       text        not null,
  amount_minor      bigint      not null check (amount_minor > 0),
  reference         text        not null,
  status            text        not null check (status in ('PENDING', 'BOOKED', 'SUSPENSE')),
  booked_to         text        not null default '',
  seen_at           timestamptz not null default now(),
  booked_at         timestamptz,
  primary key (asset, movement_id)
);

create index if not exists inbound_credit_pending on inbound_credit (asset) where status = 'PENDING';
