-- Consent schema (ARCHITECTURE.md 11.5). A consent is never deleted; revocation sets revoked_at.
create table if not exists consent (
  consent_id    uuid        primary key,
  party_id      uuid        not null,
  purpose       text        not null,
  scopes        text[]      not null check (cardinality(scopes) > 0),
  data_sources  text[]      not null check (cardinality(data_sources) > 0),
  granted_at    timestamptz not null default now(),
  expires_at    timestamptz,
  revoked_at    timestamptz,
  evidence_ref  text        not null,
  granted_by    text        not null
);

create index if not exists consent_party_purpose on consent (party_id, purpose);

create table if not exists idempotency (
  scope        text  not null,
  idem_key     text  not null,
  request_hash bytea not null,
  resource_id  uuid  not null,
  primary key (scope, idem_key)
);
