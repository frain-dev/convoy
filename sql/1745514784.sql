-- +migrate Up
alter table convoy.portal_links add column if not exists token_expires_at timestamptz default null;
alter table convoy.portal_links add column if not exists token_mask_id text default '';
alter table convoy.portal_links add column if not exists token_hash text default '';
alter table convoy.portal_links add column if not exists token_salt text default '';

-- +migrate Down
alter table convoy.portal_links drop column if exists token_expires_at;
alter table convoy.portal_links drop column if exists token_mask_id;
alter table convoy.portal_links drop column if exists token_hash;
alter table convoy.portal_links drop column if exists token_salt;