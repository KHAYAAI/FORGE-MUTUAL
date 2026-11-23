-- acquiring-service. Card details are never here: only what the merchant asked for and what the acquirer reported.
create table if not exists payment_link (
  link_id               text        primary key,
  token                 text        not null unique,
  party_id              text        not null,
  settlement_account_id text        not null,
  amount_minor          bigint      not null check (amount_minor > 0),
  description           text        not null,
  reference             text        not null default '',
  channel               text        not null check (channel in ('link', 'pos')),
  merchant_name         text        not null,
  status                text        not null check (status in ('OPEN', 'PAID', 'EXPIRED', 'CANCELLED')),
  created_at            timestamptz not null,
  expires_at            timestamptz not null,
  paid_at               timestamptz,
  acquirer_ref          text        not null default '',
  refunded_minor        bigint      not null default 0 check (refunded_minor >= 0 and refunded_minor <= amount_minor),
  settlement_id         text        not null default '',
  idempotency_key       text        not null unique
);
create index if not exists payment_link_party on payment_link (party_id, created_at desc);
create index if not exists payment_link_unsettled on payment_link (paid_at) where status = 'PAID' and settlement_id = '';

create table if not exists settlement (
  settlement_id   text   primary key,
  party_id        text   not null,
  account_id      text   not null,
  day             date   not null,
  gross_minor     bigint not null,
  fee_minor       bigint not null,
  net_minor       bigint not null,
  payments        int    not null,
  status          text   not null check (status in ('PENDING', 'PAID')),
  unique (party_id, account_id, day)
);

create table if not exists refund (
  idempotency_key text        primary key,
  link_id         text        not null references payment_link (link_id),
  amount_minor    bigint      not null check (amount_minor > 0),
  request_hash    text        not null,
  status          text        not null check (status in ('BOOKED', 'SENT')),
  created_at      timestamptz not null
);
