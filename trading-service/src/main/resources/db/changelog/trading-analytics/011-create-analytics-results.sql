-- noinspection SqlNoDataSourceInspectionForFile
-- liquibase formatted sql

-- changeset codex:011-create-analytics-job-runs
CREATE TABLE analytics_job_runs (
    run_id       VARCHAR(36)   PRIMARY KEY,
    job_name     VARCHAR(80)   NOT NULL,
    status       VARCHAR(20)   NOT NULL,
    started_at   TIMESTAMP     NOT NULL,
    completed_at TIMESTAMP,
    message      VARCHAR(512)
);

CREATE INDEX idx_analytics_job_runs_status_completed
    ON analytics_job_runs (status, completed_at DESC);

-- rollback DROP TABLE IF EXISTS analytics_job_runs;


-- changeset codex:011-create-analytics-client-segments
CREATE TABLE analytics_client_segments (
    id                    BIGSERIAL       PRIMARY KEY,
    run_id                VARCHAR(36)     NOT NULL,
    user_id               BIGINT          NOT NULL,
    cluster_id            INTEGER         NOT NULL,
    segment_label         VARCHAR(48)     NOT NULL,
    total_portfolio_value NUMERIC(19, 4)  NOT NULL DEFAULT 0,
    total_cost_basis      NUMERIC(19, 4)  NOT NULL DEFAULT 0,
    unrealized_pnl        NUMERIC(19, 4)  NOT NULL DEFAULT 0,
    holdings_count        INTEGER         NOT NULL DEFAULT 0,
    max_holding_percent   NUMERIC(9, 4)   NOT NULL DEFAULT 0,
    order_count           INTEGER         NOT NULL DEFAULT 0,
    average_order_value   NUMERIC(19, 4)  NOT NULL DEFAULT 0,
    buy_sell_ratio        NUMERIC(19, 4)  NOT NULL DEFAULT 0,
    risk_score            NUMERIC(9, 4)   NOT NULL DEFAULT 0,
    computed_at           TIMESTAMP       NOT NULL
);

CREATE INDEX idx_analytics_client_segments_run
    ON analytics_client_segments (run_id);
CREATE INDEX idx_analytics_client_segments_run_risk
    ON analytics_client_segments (run_id, risk_score DESC, user_id ASC);
CREATE UNIQUE INDEX uk_analytics_client_segments_run_user
    ON analytics_client_segments (run_id, user_id);

-- rollback DROP TABLE IF EXISTS analytics_client_segments;


-- changeset codex:011-create-analytics-portfolio-risk
CREATE TABLE analytics_portfolio_risk (
    id                    BIGSERIAL       PRIMARY KEY,
    run_id                VARCHAR(36)     NOT NULL,
    user_id               BIGINT          NOT NULL,
    total_market_value    NUMERIC(19, 4)  NOT NULL DEFAULT 0,
    total_cost_basis      NUMERIC(19, 4)  NOT NULL DEFAULT 0,
    unrealized_pnl        NUMERIC(19, 4)  NOT NULL DEFAULT 0,
    holdings_count        INTEGER         NOT NULL DEFAULT 0,
    max_holding_percent   NUMERIC(9, 4)   NOT NULL DEFAULT 0,
    diversification_score NUMERIC(9, 4)   NOT NULL DEFAULT 0,
    risk_score            NUMERIC(9, 4)   NOT NULL DEFAULT 0,
    risk_level            VARCHAR(16)     NOT NULL,
    computed_at           TIMESTAMP       NOT NULL
);

CREATE INDEX idx_analytics_portfolio_risk_run
    ON analytics_portfolio_risk (run_id);
CREATE INDEX idx_analytics_portfolio_risk_run_score
    ON analytics_portfolio_risk (run_id, risk_score DESC, user_id ASC);
CREATE UNIQUE INDEX uk_analytics_portfolio_risk_run_user
    ON analytics_portfolio_risk (run_id, user_id);

-- rollback DROP TABLE IF EXISTS analytics_portfolio_risk;


-- changeset codex:011-create-analytics-top-tickers
CREATE TABLE analytics_top_tickers (
    id                BIGSERIAL       PRIMARY KEY,
    run_id            VARCHAR(36)     NOT NULL,
    ticker_rank       INTEGER         NOT NULL,
    listing_id        BIGINT          NOT NULL,
    ticker            VARCHAR(64)     NOT NULL,
    traded_quantity   BIGINT          NOT NULL DEFAULT 0,
    traded_notional   NUMERIC(19, 4)  NOT NULL DEFAULT 0,
    order_count       INTEGER         NOT NULL DEFAULT 0,
    transaction_count INTEGER         NOT NULL DEFAULT 0,
    computed_at       TIMESTAMP       NOT NULL
);

CREATE INDEX idx_analytics_top_tickers_run
    ON analytics_top_tickers (run_id, ticker_rank ASC);
CREATE UNIQUE INDEX uk_analytics_top_tickers_run_rank
    ON analytics_top_tickers (run_id, ticker_rank);

-- rollback DROP TABLE IF EXISTS analytics_top_tickers;
