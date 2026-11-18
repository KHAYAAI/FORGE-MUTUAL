-- Membership of a mutual bank (ARCHITECTURE.md 5.4). Built for the planned legal form; unused if Forge becomes a bank.
create table if not exists member (
  member_id    text primary key,
  party_id     text not null unique,
  member_class text not null check (member_class in ('INDIVIDUAL','BUSINESS','MFI','COOPERATIVE')),
  status       text not null check (status in ('ACTIVE','SUSPENDED','EXITED')),
  admitted_at  timestamptz not null
);

create table if not exists member_share (
  movement_key  text primary key,               -- idempotency key
  member_id     text not null references member,
  share_class   text not null,
  units         bigint not null,                -- positive issue, negative redemption
  paid_up_minor bigint not null,                -- ZAR cents, same sign as units
  at            timestamptz not null,
  ledger_transfer_id text                       -- set when booked to 8001 MEMBER_SHARE_CAPITAL
);
create index if not exists member_share_member on member_share (member_id);

create table if not exists meeting (
  meeting_id     text primary key,
  kind           text not null check (kind in ('AGM','SPECIAL')),
  notice_sent_at timestamptz not null,          -- the record date: only members admitted by then may vote
  quorum         int not null check (quorum > 0)
);

create table if not exists resolution (
  resolution_id text primary key,
  meeting_id    text not null references meeting,
  text_ref      text not null,
  kind          text not null check (kind in ('ORDINARY','SPECIAL')),
  voting_rule   text not null check (voting_rule in ('ONE_MEMBER_ONE_VOTE','BY_SHARES')),
  result        text check (result in ('PASSED','FAILED','INQUORATE')),
  for_weight    bigint, against_weight bigint, abstain_weight bigint, voters int
);

create table if not exists member_vote (
  resolution_id text not null references resolution,
  member_id     text not null references member,
  weight        bigint not null,
  choice        text not null check (choice in ('FOR','AGAINST','ABSTAIN')),
  channel       text not null,
  cast_at       timestamptz not null,
  primary key (resolution_id, member_id)        -- one vote per member, whatever the channel
);

create table if not exists member_distribution (
  period      text not null,
  member_id   text not null references member,
  basis       text not null check (basis in ('FEE_REBATE','INTEREST_UPLIFT','DIVIDEND')),
  amount_minor bigint not null check (amount_minor >= 0),
  approved_by text not null,
  paid_transfer_id text,
  primary key (period, member_id, basis)
);
