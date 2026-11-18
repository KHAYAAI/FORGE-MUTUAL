-- Monitoring events: the money movements the rules read, recorded from the event bus. One row per event, so a
-- redelivered event is recorded once.
create table if not exists monitor_event (
  event_id     text        primary key,
  party_id     text        not null,
  kind         text        not null check (kind in ('PAYMENT_OUT', 'DEPOSIT_IN')),
  at           timestamptz not null,
  amount_minor bigint      not null check (amount_minor > 0),
  beneficiary  text        not null default ''
);
create index if not exists monitor_event_party on monitor_event (party_id, at);
