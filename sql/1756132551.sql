-- +migrate Up
create type convoy.portal_auth_types as enum ('static_token', 'refresh_token');
alter table convoy.portal_links add column if not exists auth_type convoy.portal_auth_types not null default 'static_token';

-- Assigns deterministic owner IDs (16-char SHA256 hash) to endpoints based on their associated portal IDs
with portals_with_ids as (select id, endpoints from convoy.portal_links where length(endpoints) > 0)
update convoy.endpoints e
set owner_id = substr(encode(sha256(p.id::bytea), 'hex'), 1, 16)
from portals_with_ids p, unnest(p.endpoints::text[]) as endpoint_id
where e.id = endpoint_id and (e.owner_id is null or length(trim(e.owner_id)) = 0);

update convoy.portal_links
set owner_id = substr(encode(sha256(id::bytea), 'hex'), 1, 16)
where length(endpoints) > 0
  and (owner_id is null or length(trim(owner_id)) = 0);

update convoy.portal_links
set auth_type = 'static_token'
where deleted_at is null;

-- +migrate Down
alter table convoy.portal_links drop column if exists auth_type;
drop type if exists convoy.portal_auth_types;
