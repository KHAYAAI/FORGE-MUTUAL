-- Compliance cases (ARCHITECTURE.md 15.1). One open case per party and rule. The person who opened a case cannot close it.
create table if not exists compliance_case (
  case_id        text        primary key,
  party_id       text        not null,
  rule           text        not null,
  severity       text        not null,
  detail         text        not null,
  evidence       text[]      not null default '{}',
  status         text        not null check (status in ('OPEN', 'CLOSED')),
  opened_by      text        not null,
  opened_at      timestamptz not null,
  sla_due_at     timestamptz not null,
  closed_by      text,
  closed_at      timestamptz,
  closing_note   text
);
create unique index if not exists compliance_case_one_open on compliance_case (party_id, rule) where status = 'OPEN';
create index if not exists compliance_case_overdue on compliance_case (sla_due_at) where status = 'OPEN';
