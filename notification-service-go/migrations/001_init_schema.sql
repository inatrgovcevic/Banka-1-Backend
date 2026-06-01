-- =============================================================================
-- Migration 001 — Initial schema for the Go Notification Service
-- Compatible with the existing Spring Boot schema (V1__init_schema.sql).
-- =============================================================================

CREATE TABLE IF NOT EXISTS notification_deliveries (
    delivery_id      UUID         PRIMARY KEY,
    recipient_email  VARCHAR(255) NOT NULL,
    subject          TEXT         NOT NULL,
    body             TEXT         NOT NULL,

    -- Lifecycle status: PENDING | PROCESSING | RETRY_SCHEDULED | SUCCEEDED | FAILED
    status           VARCHAR(50)  NOT NULL,

    notification_type VARCHAR(50) NOT NULL,

    -- retry_count maps to AttemptCount in Go; kept for Spring Boot column compatibility.
    retry_count      INT          NOT NULL DEFAULT 0,
    max_retries      INT          NOT NULL,

    last_error       TEXT,
    next_attempt_at  TIMESTAMPTZ,
    last_attempt_at  TIMESTAMPTZ,
    sent_at          TIMESTAMPTZ,

    created_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

ALTER TABLE notification_deliveries
    DROP CONSTRAINT IF EXISTS notification_deliveries_status_check;

ALTER TABLE notification_deliveries
    ADD CONSTRAINT notification_deliveries_status_check
        CHECK (status IN ('PENDING', 'PROCESSING', 'RETRY_SCHEDULED', 'SUCCEEDED', 'FAILED'));

CREATE INDEX IF NOT EXISTS idx_notification_deliveries_status
    ON notification_deliveries (status);

CREATE INDEX IF NOT EXISTS idx_notification_deliveries_next_attempt_at
    ON notification_deliveries (next_attempt_at);

-- Partial index for startup recovery and retry scheduler polling.
CREATE INDEX IF NOT EXISTS idx_notification_deliveries_recoverable
    ON notification_deliveries (status, created_at)
    WHERE status IN ('PENDING', 'RETRY_SCHEDULED');

-- =============================================================================
-- FCM device token registry (one active token per client).
-- =============================================================================
CREATE TABLE IF NOT EXISTS fcm_tokens (
    id         BIGSERIAL    PRIMARY KEY,
    client_id  BIGINT       NOT NULL UNIQUE,
    fcm_token  VARCHAR(512) NOT NULL,
    updated_at TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_fcm_tokens_client_id
    ON fcm_tokens (client_id);
