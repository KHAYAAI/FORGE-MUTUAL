-- Third-party offers. Forge introduces; the provider provides, is licensed, and monitors. No money or assets are held here.
create table if not exists provider (
  provider_id         text primary key,
  name                text not null,
  licence_type        text not null check (licence_type in ('FSP','CASP')),
  regulator           text not null,
  licence_number      text not null,
  licence_source_url  text not null,
  status              text not null check (status in ('PENDING','ACTIVE','SUSPENDED','WITHDRAWN')),
  registered_by       text not null,
  licence_verified_at timestamptz,
  licence_verified_by text,
  licence_expires_at  timestamptz,
  status_reason       text not null default '',
  created_at          timestamptz not null
);

create table if not exists provider_event (
  seq         bigserial primary key,
  provider_id text not null references provider,
  at          timestamptz not null,
  actor       text not null,
  action      text not null,
  detail      text not null default ''
);

create table if not exists offer (
  offer_id       text primary key,
  provider_id    text not null references provider,
  category       text not null check (category in ('INVESTMENTS','CRYPTO_ASSETS')),
  title          text not null,
  summary        text not null,
  handoff_url    text not null,
  fee_disclosure text not null,
  status         text not null check (status in ('DRAFT','ACTIVE','WITHDRAWN')),
  updated_at     timestamptz not null
);

create table if not exists referral (
  referral_id     text primary key,
  idempotency_key text not null,
  party_id        text not null,
  offer_id        text not null references offer,
  statement_hash  text not null,       -- the exact text the party acknowledged
  consent_id      text not null,
  created_at      timestamptz not null,
  unique (party_id, idempotency_key)
);
create index if not exists referral_party on referral (party_id, created_at desc);

-- Sending a referred customer's details, referral fees, and the automatic licence check.
alter table provider add column if not exists delivery_url text not null default '';
alter table provider add column if not exists licence_checked_at timestamptz;
alter table offer add column if not exists fee_cap_minor bigint not null default 0;

-- One row per referral: has the customer's name been sent to the provider? Only the hash of what was sent is kept, never the details.
create table if not exists referral_delivery (
  referral_id  text primary key references referral,
  status       text not null check (status in ('PENDING','SENT','FAILED','CANCELLED','NOT_CONFIGURED')),
  attempts     int  not null default 0,
  next_at      timestamptz not null,
  detail       text not null default '',
  payload_hash text not null default '',
  sent_at      timestamptz
);
create index if not exists referral_delivery_due on referral_delivery (next_at) where status = 'PENDING';

-- Referral fees the provider paid, booked as income against the bank receipt. A bank reference is booked once.
create table if not exists referral_fee (
  bank_reference text primary key,
  referral_id    text not null references referral,
  amount_minor   bigint not null check (amount_minor > 0),
  recorded_by    text not null,
  approved_by    text not null,
  recorded_at    timestamptz not null,
  ledger_key     text not null
);
create index if not exists referral_fee_by_referral on referral_fee (referral_id);
-- A fee one person has entered from a bank statement and a different person has yet to confirm. Nothing is booked from it.
create table if not exists referral_fee_proposal (
  bank_reference text primary key,
  referral_id    text not null references referral,
  amount_minor   bigint not null check (amount_minor > 0),
  proposed_by    text not null,
  proposed_at    timestamptz not null
);
