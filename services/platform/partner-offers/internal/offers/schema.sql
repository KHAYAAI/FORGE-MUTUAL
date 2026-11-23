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
