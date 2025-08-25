-- +migrate Up
create type convoy.portal_auth_types as enum ('none', 'static_token', 'owner_id', 'refresh_token');
alter table convoy.portal_links add column auth_type convoy.portal_auth_types not null default 'refresh_token';

-- convert portal links with endpoint arrays, for each endpoint in that array set the owner_id of the endpoints to the same value
with portals_with_ids as (select id, endpoints from convoy.portal_links where length(endpoints) > 0)
update convoy.endpoints e
set owner_id = substr(encode(sha256(p.id::bytea), 'hex'), 1, 16)
from portals_with_ids p, unnest(p.endpoints::text[]) as endpoint_id
where e.id = endpoint_id and (e.owner_id is null or length(trim(e.owner_id)) = 0);

update convoy.portal_links
set owner_id = substr(encode(sha256(id::bytea), 'hex'), 1, 16)
where length(endpoints) > 0
  and (owner_id is null or length(trim(owner_id)) = 0);

-- +migrate Down
alter table convoy.portal_links drop column auth_type;
drop type convoy.portal_auth_types;
