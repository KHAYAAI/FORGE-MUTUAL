-- Processed events: one row per consumer and event. A redelivered event is recognised by its row.
create table if not exists eventbus_processed (
  consumer      text        not null,
  event_id      text        not null,
  processed_at  timestamptz not null default now(),
  primary key (consumer, event_id)
);

-- Events that could not be applied, kept for an operator. Nothing is dropped silently.
create table if not exists eventbus_dead_letter (
  seq           bigserial   primary key,
  consumer      text        not null,
  topic         text        not null,
  partition     integer     not null,
  kafka_offset  bigint      not null,
  error         text        not null,
  value         bytea       not null,
  created_at    timestamptz not null default now()
);
