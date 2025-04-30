-- +migrate Up
create table if not exists convoy.portal_tokens (
    id varchar primary key,
    portal_link_id varchar not null,
    token_mask_id text default '',
    token_hash text default '',
    token_salt text default '',
    token_expires_at timestamptz default null,
    created_at timestamptz default current_timestamp,
    updated_at timestamptz default current_timestamp,
    deleted_at timestamptz,
    constraint fk_portal_links
        foreign key (portal_link_id)
            references convoy.portal_links(id)
            on delete cascade
);

-- +migrate Down
drop table if exists convoy.portal_tokens;