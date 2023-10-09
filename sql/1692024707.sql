-- +migrate Up
ALTER TABLE convoy.endpoints
    ADD CONSTRAINT endpoints_title_project_id_pk
        UNIQUE (title, project_id, deleted_at);

-- +migrate Down
ALTER TABLE convoy.endpoints
    DROP CONSTRAINT endpoints_title_project_id_pk;
