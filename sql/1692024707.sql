-- +migrate Up
ALTER TABLE convoy.endpoints
    ADD CONSTRAINT endpoints_title_pk
        UNIQUE (title);

-- +migrate Down
ALTER TABLE convoy.endpoints
    DROP CONSTRAINT endpoints_title_pk;
