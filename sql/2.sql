-- +migrate Up
ALTER TABLE convoy.event_deliveries ALTER COLUMN attempts TYPE varchar USING attempts::varchar;
ALTER TABLE convoy.event_deliveries ALTER COLUMN attempts TYPE bytea USING attempts::bytea;


-- +migrate Down
ALTER TABLE convoy.event_deliveries ALTER COLUMN attempts TYPE varchar USING attempts::varchar;
ALTER TABLE convoy.event_deliveries ALTER COLUMN attempts TYPE jsonb USING attempts::jsonb;
