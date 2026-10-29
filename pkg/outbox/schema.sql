create table if not exists outbox (
  seq           bigserial   primary key,
  event_id      uuid        not null unique,
  topic         text        not null,
  event_key     bytea       not null,
  payload       bytea       not null,
  created_at    timestamptz not null default now(),
  published_at  timestamptz
);

create index if not exists outbox_unpublished on outbox (seq) where published_at is null;
