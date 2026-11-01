-- Contact addresses are sealed with AES-256-GCM before they are stored. Nothing here is plaintext personal data.
create table if not exists notif_contact (
  party_id        text        not null,
  channel         text        not null check (channel in ('EMAIL', 'SMS')),
  address_sealed  bytea       not null,
  updated_at      timestamptz not null default now(),
  primary key (party_id, channel)
);

-- One row per event and channel. A message for a channel with no contact is recorded as SKIPPED, not dropped.
create table if not exists notif_message (
  message_id   text        primary key,
  event_id     text        not null,
  party_id     text        not null,
  channel      text        not null check (channel in ('EMAIL', 'SMS')),
  subject      text        not null default '',
  body         text        not null,
  status       text        not null check (status in ('PENDING', 'SENT', 'FAILED', 'SKIPPED')),
  attempts     integer     not null default 0,
  last_error   text        not null default '',
  created_at   timestamptz not null default now(),
  sent_at      timestamptz,
  unique (event_id, channel)
);
create index if not exists notif_message_pending on notif_message (created_at) where status = 'PENDING';
