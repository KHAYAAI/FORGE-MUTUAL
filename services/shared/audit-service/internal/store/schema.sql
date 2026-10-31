-- Audit log (ARCHITECTURE.md 15.3). Append-only: UPDATE, DELETE and TRUNCATE are refused by trigger.
create table if not exists audit_entry (
  seq          bigint      primary key,
  at_micros    bigint      not null,
  actor        text        not null,
  action       text        not null,
  entity       text        not null,
  payload      bytea       not null,
  payload_hash bytea       not null,
  prev_hash    bytea       not null,
  hash         bytea       not null
);

create or replace function audit_entry_immutable() returns trigger
language plpgsql as $$
begin
  raise exception 'audit_entry is append-only: % refused', tg_op;
end;
$$;

drop trigger if exists audit_entry_no_update on audit_entry;
create trigger audit_entry_no_update before update or delete on audit_entry
  for each row execute function audit_entry_immutable();

drop trigger if exists audit_entry_no_truncate on audit_entry;
create trigger audit_entry_no_truncate before truncate on audit_entry
  for each statement execute function audit_entry_immutable();
