-- KYC reviews: a verification the provider could not decide, waiting for a person. One open review per subject.
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
  decided_at        timestamptz
);
create unique index if not exists kyc_review_one_open on kyc_review (onboarding_id, subject_party_id) where status = 'OPEN';
create index if not exists kyc_review_open on kyc_review (opened_at) where status = 'OPEN';
