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

-- Cross-border reporting (proto/forge/payments/v1/crossborder.proto). One row for every foreign-currency credit booked
-- to a customer, written in the same transaction as the booking. The customer gives the reason for the payment; the
-- row is the evidence for the authorised dealer's report to the SARB (VERIFY-FINSURV: not submitted).
create table if not exists cross_border_credit (
  asset             text        not null,
  movement_id       text        not null,
  party_id          text        not null,
  account_id        text        not null,
  amount_minor      bigint      not null check (amount_minor > 0),
  reference         text        not null,
  booked_at         timestamptz not null,
  status            text        not null default 'AWAITING_REASON' check (status in ('AWAITING_REASON', 'CLASSIFIED')),
  reason_code       text        not null default '',
  reason_note       text        not null default '',
  classified_by     text        not null default '',
  classified_at     timestamptz,
  primary key (asset, movement_id),
  check (status = 'AWAITING_REASON' or (reason_code <> '' and classified_by <> '' and classified_at is not null))
);

create index if not exists cross_border_credit_party on cross_border_credit (party_id, booked_at desc);
