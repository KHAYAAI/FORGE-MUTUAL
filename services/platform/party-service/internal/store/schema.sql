-- Party schema (ARCHITECTURE.md 11.4). Enum values are stored as their protobuf names.
create table if not exists party (
  party_id             uuid        primary key,
  party_type           text        not null check (party_type in ('PARTY_TYPE_PERSON', 'PARTY_TYPE_COMPANY', 'PARTY_TYPE_FUND')),
  status               text        not null,
  legal_name           text        not null,
  registration_number  text,
  national_id_sealed   bytea,
  national_id_index    bytea,
  national_id_masked   text,
  country              text        not null,
  tier                 text        not null default '',
  created_at           timestamptz not null default now(),
  updated_at           timestamptz not null default now()
);

-- One company per registration number; one person per national ID (by blind index).
create unique index if not exists party_registration on party (registration_number) where registration_number is not null;
create unique index if not exists party_national_id on party (national_id_index) where national_id_index is not null;

create table if not exists relationship (
  relationship_id uuid     primary key,
  from_party_id   uuid     not null references party (party_id),
  to_party_id     uuid     not null references party (party_id),
  rel_type        text     not null,
  percentage_bps  integer  not null check (percentage_bps between 0 and 10000),
  active          boolean  not null default true,
  check (from_party_id <> to_party_id)
);

-- At most one active relationship of a type between two parties.
create unique index if not exists relationship_active on relationship (from_party_id, to_party_id, rel_type) where active;

-- Idempotency for creates: the same key with a different request is a conflict.
create table if not exists idempotency (
  scope        text  not null,
  idem_key     text  not null,
  request_hash bytea not null,
  resource_id  uuid  not null,
  primary key (scope, idem_key)
);
