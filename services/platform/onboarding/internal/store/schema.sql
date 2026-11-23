-- KYC reviews: a verification the provider could not decide, waiting for a person. One open review per subject.
-- A decision is recorded before the onboarding workflow is told about it. signalled_at is set once the workflow has
-- been told (or has ended and cannot be); a decided review with no signalled_at is waiting for delivery.
create table if not exists kyc_review (
  review_id         text        primary key,
  onboarding_id     text        not null,
  subject_party_id  text        not null,
  reason            text        not null,
  status            text        not null check (status in ('OPEN', 'DECIDED')),
  opened_at         timestamptz not null,
  decision          text        not null default '' check (decision in ('', 'APPROVED', 'DECLINED')),
  reviewer          text        not null default '',
  note              text        not null default '',
  decided_at        timestamptz,
  signalled_at      timestamptz
);
-- A table made before delivery was tracked gets the column. Its decided rows read as undelivered and are told to the
-- workflow once more, which is harmless: the workflow ignores a decision for a subject it is not waiting on.
alter table kyc_review add column if not exists signalled_at timestamptz;
create unique index if not exists kyc_review_one_open on kyc_review (onboarding_id, subject_party_id) where status = 'OPEN';
create index if not exists kyc_review_open on kyc_review (opened_at) where status = 'OPEN';
create index if not exists kyc_review_undelivered on kyc_review (decided_at) where status = 'DECIDED' and signalled_at is null;

-- Verification sessions started with the provider: one per subject per onboarding. The hosted verification link lives
-- here, so that a retried activity does not start (and, in live mode, pay for) a second session, and so that whoever
-- gives the person their link can read it.
create table if not exists kyc_session (
  onboarding_id     text        not null,
  subject_party_id  text        not null,
  kind              text        not null check (kind in ('business', 'person')),
  session_id        text        not null,
  url               text        not null default '',
  created_at        timestamptz not null,
  primary key (onboarding_id, subject_party_id)
);

-- Further currency accounts opened for an active customer (OpenCurrencyAccount). One per party and currency. The row
-- is written before anything outside is asked, and the partner account is opened under its lock, so a retry or a
-- second request never opens a second account at the partner bank.
create table if not exists currency_account (
  party_id          text        not null,
  asset             text        not null check (asset in ('ZAR', 'USD', 'EUR', 'GBP')),
  account_id        text        not null default '',
  partner           text        not null default '',
  partner_account   text        not null default '',
  reference         text        not null default '',
  registered        boolean     not null default false,
  created_at        timestamptz not null default now(),
  primary key (party_id, asset)
);
