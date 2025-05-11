-- +migrate Up
CREATE TABLE IF NOT EXISTS convoy.batch_retries (
    id VARCHAR PRIMARY KEY,
    project_id VARCHAR NOT NULL,
    status VARCHAR(50) NOT NULL,
    total_events INTEGER NOT NULL,
    processed_events INTEGER NOT NULL DEFAULT 0,
    failed_events INTEGER NOT NULL DEFAULT 0,
    filter JSONB,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL,
    completed_at TIMESTAMP WITH TIME ZONE,
    error TEXT,
    CONSTRAINT fk_batch_retries_project FOREIGN KEY (project_id) REFERENCES convoy.projects(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_batch_retries_project_id ON convoy.batch_retries(project_id);
CREATE INDEX IF NOT EXISTS idx_batch_retries_status ON convoy.batch_retries(status);

-- +migrate Down
drop table if exists convoy.batch_retries;