-- =============================================================================
-- Migration 001 — Initial schema for the Go Notification Service
--
-- Compatible with the existing Spring Boot schema (V1__init_schema.sql).
-- The notification_deliveries and fcm_tokens tables are schema-identical so
-- both services can share one PostgreSQL database during the migration period.
-- =============================================================================

-- Persistent delivery tracking — source of truth for the retry lifecycle.
CREATE TABLE IF NOT EXISTS notification_deliveries (
    delivery_id      UUID         PRIMARY KEY,
    recipient_email  VARCHAR(255) NOT NULL,
    subject          TEXT         NOT NULL,
    body             TEXT         NOT NULL,

    -- Lifecycle status: PENDING | PROCESSING | RETRY_SCHEDULED | SUCCEEDED | FAILED
    -- Go adds the PROCESSING status compared to the original Spring Boot schema.
    status           VARCHAR(50)  NOT NULL,

    notification_type VARCHAR(50) NOT NULL,

    -- Renamed from attempt_count in Go domain → retry_count in the DB to
    -- maintain backward compatibility with the Spring Boot column name.
    retry_count      INT          NOT NULL DEFAULT 0,
    max_retries      INT          NOT NULL,

    last_error       TEXT,
    next_attempt_at  TIMESTAMPTZ,
    last_attempt_at  TIMESTAMPTZ,
    sent_at          TIMESTAMPTZ,

    -- Immutable; set once on INSERT.
    created_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    -- Refreshed on every UPDATE.
    updated_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- Supports the retry scheduler's polling query:
--   WHERE status = 'RETRY_SCHEDULED' AND next_attempt_at <= NOW()
CREATE INDEX IF NOT EXISTS idx_notification_deliveries_status
    ON notification_deliveries (status);

CREATE INDEX IF NOT EXISTS idx_notification_deliveries_next_attempt_at
    ON notification_deliveries (next_attempt_at);

-- Partial index for high-throughput startup recovery:
--   WHERE status IN ('PENDING','RETRY_SCHEDULED')
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

-- =============================================================================
-- Legacy support tables (kept for Spring Boot compatibility during migration)
-- =============================================================================

-- Email template catalogue — read-only at runtime; populated via seed migration.
CREATE TABLE IF NOT EXISTS email_templates (
    id            SERIAL PRIMARY KEY,
    subject       TEXT   NOT NULL,
    body_template TEXT   NOT NULL
);

-- Raw inbound request audit (optional — Go service may skip writing here).
CREATE TABLE IF NOT EXISTS notification_requests (
    id                 SERIAL PRIMARY KEY,
    username           VARCHAR(255),
    user_email         VARCHAR(255) NOT NULL,
    template_variables JSONB,
    created_at         TIMESTAMPTZ  DEFAULT NOW()
);
