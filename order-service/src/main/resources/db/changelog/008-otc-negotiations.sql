-- liquibase formatted sql

-- changeset order:8
CREATE TABLE otc_negotiation (
    id BIGSERIAL PRIMARY KEY,
    buyer_id BIGINT NOT NULL,
    seller_id BIGINT NOT NULL,
    seller_portfolio_id BIGINT NOT NULL REFERENCES portfolio(id),
    listing_id BIGINT NOT NULL,
    quantity INTEGER NOT NULL,
    price_per_unit NUMERIC(19, 4) NOT NULL,
    contract_expiry_date DATE NOT NULL,
    status VARCHAR(32) NOT NULL,
    created_by_user_id BIGINT NOT NULL,
    last_updated_by_user_id BIGINT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expiration_notified_at DATE
);

CREATE INDEX idx_otc_negotiation_buyer_id ON otc_negotiation (buyer_id);
CREATE INDEX idx_otc_negotiation_seller_id ON otc_negotiation (seller_id);
CREATE INDEX idx_otc_negotiation_status ON otc_negotiation (status);
CREATE INDEX idx_otc_negotiation_updated_at ON otc_negotiation (updated_at);
CREATE INDEX idx_otc_negotiation_expiry_date ON otc_negotiation (contract_expiry_date);

CREATE TABLE otc_negotiation_history (
    id BIGSERIAL PRIMARY KEY,
    negotiation_id BIGINT NOT NULL REFERENCES otc_negotiation(id),
    actor_user_id BIGINT NOT NULL,
    event_type VARCHAR(32) NOT NULL,
    previous_quantity INTEGER,
    new_quantity INTEGER,
    previous_price_per_unit NUMERIC(19, 4),
    new_price_per_unit NUMERIC(19, 4),
    previous_contract_expiry_date DATE,
    new_contract_expiry_date DATE,
    previous_status VARCHAR(32),
    resulting_status VARCHAR(32) NOT NULL,
    changed_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_otc_negotiation_history_negotiation_id ON otc_negotiation_history (negotiation_id);
CREATE INDEX idx_otc_negotiation_history_changed_at ON otc_negotiation_history (changed_at);
