-- lending-service. Balances live in the ledger (a CUSTOMER_LOAN account per facility: a debit balance is what the
-- customer owes). This holds the application, the decision, the schedule and the work done on it.
create table if not exists facility (
  facility_id        text        primary key,
  party_id           text        not null,
  product_id         text        not null,
  status             text        not null check (status in ('ASSESSED', 'APPROVED', 'DECLINED', 'ACTIVE', 'SETTLED', 'CANCELLED')),
  requested_minor    bigint      not null check (requested_minor > 0),
  approved_minor     bigint      not null default 0,
  rate_bps           int         not null,
  term_months        int         not null default 0,
  fee_bps            int         not null default 0,
  sweep_bps          int         not null default 0,
  current_account_id text        not null,
  loan_account_id    text        not null default '',
  purpose            text        not null default '',
  applied_by         text        not null,
  applied_at         timestamptz not null,
  idempotency_key    text        not null unique,
  assessment         jsonb       not null,
  decided_by         text        not null default '',
  decided_at         timestamptz,
  decision_note      text        not null default '',
  accepted_at        timestamptz,
  fee_owed_minor     bigint      not null default 0,
  sweep_after_seq    bigint      not null default 0,
  days_past_due      int         not null default 0,
  check (decided_by = '' or decided_by <> applied_by)
);
create index if not exists facility_party on facility (party_id, applied_at desc);
create index if not exists facility_status on facility (status);

create table if not exists instalment (
  facility_id     text   not null references facility (facility_id),
  number          int    not null,
  due_on          date   not null,
  principal_minor bigint not null check (principal_minor >= 0),
  interest_minor  bigint not null check (interest_minor >= 0),
  status          text   not null check (status in ('DUE', 'PAID', 'ARREARS', 'WAIVED')),
  primary key (facility_id, number)
);

create table if not exists od_accrual (
  facility_id  text   not null references facility (facility_id),
  day          date   not null,
  drawn_minor  bigint not null,
  accrued_nano bigint not null,
  primary key (facility_id, day)
);

create table if not exists od_charge (
  facility_id    text   not null references facility (facility_id),
  period         text   not null,
  accrued_nano   bigint not null,
  carry_in_nano  bigint not null,
  charged_minor  bigint not null,
  carry_out_nano bigint not null,
  status         text   not null check (status in ('PENDING', 'CHARGED')),
  primary key (facility_id, period)
);

-- Draws and repayments by the client's idempotency key, scoped to the party.
create table if not exists op (
  idempotency_key text primary key,
  request_hash    text not null
);

-- What happened to each facility, for statements (NCA s.108). One row per money movement, keyed by its ledger key,
-- so a retried movement is recorded once.
create table if not exists facility_event (
  event_key       text        primary key,
  facility_id     text        not null references facility (facility_id),
  at              timestamptz not null,
  kind            text        not null,
  principal_minor bigint      not null default 0,
  interest_minor  bigint      not null default 0,
  description     text        not null default ''
);
create index if not exists facility_event_by_facility on facility_event (facility_id, at);

-- Default notices (NCA s.129). At most one outstanding per facility; cleared when the arrears are paid.
create table if not exists default_notice (
  notice_id     text        primary key,
  facility_id   text        not null references facility (facility_id),
  issued_at     timestamptz not null,
  respond_by    timestamptz not null,
  arrears_minor bigint      not null,
  days_past_due int         not null,
  cleared_at    timestamptz
);
create unique index if not exists default_notice_open on default_notice (facility_id) where cleared_at is null;
