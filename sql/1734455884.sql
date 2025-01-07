-- +migrate Up
CREATE TABLE IF NOT EXISTS convoy.instance_defaults (
    id CHAR(26) PRIMARY KEY,
    scope_type VARCHAR(50) NOT NULL,
    key VARCHAR(255) NOT NULL ,
    default_value_cipher TEXT,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMPTZ,
    CONSTRAINT unique_defaults_key UNIQUE (scope_type, key)
);

CREATE TABLE IF NOT EXISTS convoy.instance_overrides (
    id CHAR(26) PRIMARY KEY,
    scope_type VARCHAR(50) NOT NULL,
    scope_id CHAR(26) NOT NULL,
    key VARCHAR(255) NOT NULL,
    value_cipher TEXT,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMPTZ,
    CONSTRAINT unique_scoped_key UNIQUE (scope_type, scope_id, key)
);

-- +migrate Down
DROP TABLE IF EXISTS convoy.instance_overrides;
DROP TABLE IF EXISTS convoy.instance_defaults;
