-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS saga_instance (
    id               UUID         PRIMARY KEY,
    saga_type        VARCHAR(64)  NOT NULL,
    correlation_id   VARCHAR(64)  NOT NULL,
    current_step     INT          NOT NULL DEFAULT 0,
    total_steps      INT          NOT NULL DEFAULT 1,
    state            VARCHAR(32)  NOT NULL,
    payload          JSONB,
    compensation_log JSONB,
    created_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    retry_count      INT          NOT NULL DEFAULT 0,
    version          BIGINT       NOT NULL DEFAULT 0,
    CONSTRAINT uk_saga_type_correlation UNIQUE (saga_type, correlation_id)
);

CREATE INDEX IF NOT EXISTS idx_saga_instance_state       ON saga_instance(state);
CREATE INDEX IF NOT EXISTS idx_saga_instance_saga_type   ON saga_instance(saga_type);
CREATE INDEX IF NOT EXISTS idx_saga_instance_created     ON saga_instance(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_saga_instance_correlation ON saga_instance(correlation_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS saga_instance;
-- +goose StatementEnd
