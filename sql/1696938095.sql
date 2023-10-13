-- +migrate Up
CREATE TABLE IF NOT EXISTS convoy.feature_flags (
    id CHAR(26) PRIMARY KEY,

    feature_key TEXT UNIQUE NOT NULL,
    type TEXT UNIQUE NOT NULL,

    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMPTZ,

    CONSTRAINT users_email_key UNIQUE NULLS NOT DISTINCT (email, deleted_at)
);


-- +migrate Down

DROP TABLE IF EXISTS convoy.feature_flags_organisations;
DROP TABLE IF EXISTS convoy.feature_flags;
