--liquibase formatted sql

--changeset trading:007-stock-reservations
CREATE TABLE IF NOT EXISTS stock_reservations (
    reservation_id    UUID         PRIMARY KEY,
    correlation_id    VARCHAR(64),
    seller_id         BIGINT       NOT NULL,
    listing_id        BIGINT       NOT NULL,
    stock_ticker      VARCHAR(16)  NOT NULL,
    amount            INT          NOT NULL,
    status            VARCHAR(16)  NOT NULL DEFAULT 'HELD',
    created_at        TIMESTAMP    NOT NULL DEFAULT NOW(),
    released_at       TIMESTAMP
);

CREATE TABLE IF NOT EXISTS stock_ownership_transfers (
    transfer_id       UUID         PRIMARY KEY,
    reservation_id    UUID         NOT NULL,
    correlation_id    VARCHAR(64),
    seller_id         BIGINT       NOT NULL,
    buyer_id          BIGINT       NOT NULL,
    listing_id        BIGINT       NOT NULL,
    stock_ticker      VARCHAR(16)  NOT NULL,
    amount            INT          NOT NULL,
    status            VARCHAR(16)  NOT NULL DEFAULT 'COMPLETED',
    created_at        TIMESTAMP    NOT NULL DEFAULT NOW(),
    reversed_at       TIMESTAMP
);