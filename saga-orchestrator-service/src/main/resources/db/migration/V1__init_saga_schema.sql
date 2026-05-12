-- Issue #214: SagaInstance entitet sa poljima id, sagaType, currentStep, state,
-- payload (jsonb), compensationLog (jsonb), createdAt, updatedAt, retryCount.
CREATE TABLE IF NOT EXISTS saga_instance (
    id                UUID         PRIMARY KEY,
    saga_type         VARCHAR(64)  NOT NULL,
    current_step      INT          NOT NULL DEFAULT 0,
    state             VARCHAR(32)  NOT NULL,
    payload           JSONB,
    compensation_log  JSONB,
    created_at        TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ  NOT NULL DEFAULT now(),
    retry_count       INT          NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_saga_instance_state     ON saga_instance(state);
CREATE INDEX IF NOT EXISTS idx_saga_instance_saga_type ON saga_instance(saga_type);
CREATE INDEX IF NOT EXISTS idx_saga_instance_created   ON saga_instance(created_at DESC);
