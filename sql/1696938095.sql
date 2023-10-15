-- +migrate Up
CREATE TABLE IF NOT EXISTS convoy.feature_flags (
    id CHAR(26) PRIMARY KEY,

    feature_key TEXT UNIQUE NOT NULL,
    type TEXT NOT NULL,

    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMPTZ
);


-- +migrate Down

DROP TABLE IF EXISTS convoy.feature_flags_organisations;
DROP TABLE IF EXISTS convoy.feature_flags;
