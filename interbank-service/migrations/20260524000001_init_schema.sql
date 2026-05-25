-- +goose Up
-- +goose StatementBegin
-- PR_32 Phase 1: inter-bank protokol — pocetna sema (4 tabele).
--
-- Tabele:
--   1. interbank_messages       — idempotency log (svaki INBOUND/OUTBOUND poziv)
--   2. interbank_transactions   — 2PC state (PREPARED / COMMITTED / ABORTED)
--   3. interbank_negotiations   — OTC §3 negotiation state (Tim 2 protokol)
--   4. interbank_contracts      — OTC §3 finalizovani option contracts

CREATE TABLE interbank_messages (
    id                       BIGSERIAL                PRIMARY KEY,
    direction                VARCHAR(10)              NOT NULL,
    sender_routing_number    INT                      NOT NULL,
    locally_generated_key    VARCHAR(64)              NOT NULL,
    message_type             VARCHAR(32)              NOT NULL,
    status                   VARCHAR(32)              NOT NULL,
    request_body             TEXT                     NOT NULL,
    response_body            TEXT,
    http_status              INT,
    retry_count              INT                      NOT NULL DEFAULT 0 CHECK (retry_count >= 0),
    transaction_id_routing   INT,
    transaction_id_local     VARCHAR(64),
    last_attempt_at          TIMESTAMP WITH TIME ZONE,
    created_at               TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_interbank_messages
        UNIQUE (direction, sender_routing_number, locally_generated_key)
);

CREATE INDEX idx_interbank_messages_outbound_pending
    ON interbank_messages(status, last_attempt_at)
    WHERE direction = 'OUTBOUND' AND status IN ('PENDING_SEND', 'SENT') AND retry_count < 5;

CREATE INDEX idx_interbank_messages_tx
    ON interbank_messages(transaction_id_routing, transaction_id_local);

CREATE TABLE interbank_transactions (
    id                       BIGSERIAL                PRIMARY KEY,
    transaction_id_routing   INT                      NOT NULL,
    transaction_id_local     VARCHAR(64)              NOT NULL,
    status                   VARCHAR(32)              NOT NULL,
    postings_json            JSONB                    NOT NULL,
    reservation_refs         JSONB,
    message_meta             JSONB,
    created_at               TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    finalized_at             TIMESTAMP WITH TIME ZONE,
    CONSTRAINT uq_interbank_transactions
        UNIQUE (transaction_id_routing, transaction_id_local)
);

CREATE INDEX idx_interbank_transactions_status_age
    ON interbank_transactions(status, created_at)
    WHERE status = 'PREPARED';

CREATE TABLE interbank_negotiations (
    id                         VARCHAR(64)              PRIMARY KEY,
    buyer_routing_number       INT                      NOT NULL,
    buyer_id                   VARCHAR(64)              NOT NULL,
    seller_routing_number      INT                      NOT NULL,
    seller_id                  VARCHAR(64)              NOT NULL,
    stock_ticker               VARCHAR(16)              NOT NULL,
    amount                     INT                      NOT NULL CHECK (amount > 0),
    price_currency             VARCHAR(8)               NOT NULL,
    price_amount               NUMERIC(20,4)            NOT NULL CHECK (price_amount > 0),
    premium_currency           VARCHAR(8)               NOT NULL,
    premium_amount             NUMERIC(20,4)            NOT NULL CHECK (premium_amount >= 0),
    settlement_date            TIMESTAMP WITH TIME ZONE NOT NULL,
    last_modified_by_routing   INT                      NOT NULL,
    last_modified_by_id        VARCHAR(64)              NOT NULL,
    is_ongoing                 BOOLEAN                  NOT NULL DEFAULT true,
    is_authoritative           BOOLEAN                  NOT NULL,
    remote_negotiation_id      VARCHAR(64),
    linked_local_offer_id      BIGINT,
    version                    BIGINT                   NOT NULL DEFAULT 0,
    created_at                 TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    last_modified_at           TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_interbank_negotiations_remote
    ON interbank_negotiations(buyer_routing_number, buyer_id);

CREATE INDEX idx_interbank_negotiations_ongoing_settlement
    ON interbank_negotiations(is_ongoing, settlement_date)
    WHERE is_ongoing = true;

CREATE TABLE interbank_contracts (
    id                            VARCHAR(64)              PRIMARY KEY,
    negotiation_id                VARCHAR(64)              NOT NULL
        REFERENCES interbank_negotiations(id),
    buyer_routing_number          INT                      NOT NULL,
    buyer_id                      VARCHAR(64)              NOT NULL,
    seller_routing_number         INT                      NOT NULL,
    seller_id                     VARCHAR(64)              NOT NULL,
    stock_ticker                  VARCHAR(16)              NOT NULL,
    amount                        INT                      NOT NULL CHECK (amount > 0),
    strike_currency               VARCHAR(8)               NOT NULL,
    strike_amount                 NUMERIC(20,4)            NOT NULL CHECK (strike_amount > 0),
    settlement_date               TIMESTAMP WITH TIME ZONE NOT NULL,
    status                        VARCHAR(32)              NOT NULL,
    option_pseudo_owner_routing   INT                      NOT NULL,
    option_pseudo_owner_id        VARCHAR(64)              NOT NULL,
    version                       BIGINT                   NOT NULL DEFAULT 0,
    created_at                    TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    exercised_at                  TIMESTAMP WITH TIME ZONE,
    expired_at                    TIMESTAMP WITH TIME ZONE
);

CREATE INDEX idx_interbank_contracts_status_settle
    ON interbank_contracts(status, settlement_date);

CREATE INDEX idx_interbank_contracts_ticker_seller
    ON interbank_contracts(seller_routing_number, seller_id, stock_ticker, status);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS interbank_contracts;
DROP TABLE IF EXISTS interbank_negotiations;
DROP TABLE IF EXISTS interbank_transactions;
DROP TABLE IF EXISTS interbank_messages;
-- +goose StatementEnd
