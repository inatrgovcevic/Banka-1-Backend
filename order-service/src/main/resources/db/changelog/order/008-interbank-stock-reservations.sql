--liquibase formatted sql

-- changeset interbank:32-order-008-interbank-stock-reservations
-- PR_32 Phase 12: 2PC rezervacije akcija u portfoliu za interbank OTC opcione
-- ugovore. Razlikuje se od portfolio.reserved_quantity (007) jer:
--
--   * portfolio.reserved_quantity je agregat - drzi koliko jedinica je trenutno
--     vezano za otvorene obaveze (intra-bank SELL orderi + intra-bank OTC).
--   * interbank_stock_reservations je per-rezervaciju red - drzi 2PC stanje
--     specifican za inter-banka transakcije. Kljuc je (transaction_id_routing,
--     transaction_id_local) per Tim 2 §4.6.
--
-- Pattern: reserveStock() inkrementuje portfolio.reservedQuantity, commitStock
-- skida i quantity i reservedQuantity, releaseStock samo skida reservedQuantity.
-- Idempotentno prema reservation_id.

CREATE TABLE interbank_stock_reservations (
    id BIGSERIAL PRIMARY KEY,
    reservation_id UUID NOT NULL UNIQUE,
    transaction_id_routing INT NOT NULL,
    transaction_id_local VARCHAR(64) NOT NULL,
    portfolio_id BIGINT NOT NULL,
    ticker VARCHAR(16) NOT NULL,
    quantity INT NOT NULL CHECK (quantity > 0),
    status VARCHAR(16) NOT NULL,                     -- HELD / COMMITTED / RELEASED
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    finalized_at TIMESTAMP WITH TIME ZONE
);

CREATE INDEX idx_interbank_stock_reservations_tx
    ON interbank_stock_reservations(transaction_id_routing, transaction_id_local);

CREATE INDEX idx_interbank_stock_reservations_portfolio_status
    ON interbank_stock_reservations(portfolio_id, status);

-- rollback DROP TABLE IF EXISTS interbank_stock_reservations;
