create table if not exists fx_quote (
  quote_id      text        primary key,
  party_id      text        not null,
  sell_asset    text        not null,
  buy_asset     text        not null,
  sell_amount   bigint      not null check (sell_amount > 0),
  buy_amount    bigint      not null check (buy_amount > 0),
  mid           bigint      not null,
  rate          bigint      not null,
  spread_bps    integer     not null,
  spread_minor  bigint      not null,
  expires_at    timestamptz not null,
  status        text        not null check (status in ('ISSUED', 'BOOKED'))
);

create table if not exists fx_deal (
  deal_id       text        primary key,
  quote_id      text        not null unique references fx_quote (quote_id),
  party_id      text        not null,
  sell_asset    text        not null,
  buy_asset     text        not null,
  sell_amount   bigint      not null,
  buy_amount    bigint      not null,
  spread_minor  bigint      not null,
  sell_account  text        not null,
  buy_account   text        not null,
  state         text        not null,
  reason        text        not null default '',
  sell_holds    text[]      not null default '{}',
  buy_holds     text[]      not null default '{}',
  updated_at    timestamptz not null default now()
);
create index if not exists fx_deal_open on fx_deal (updated_at) where state not in ('COMPLETED', 'VOIDED', 'REVERSED');
