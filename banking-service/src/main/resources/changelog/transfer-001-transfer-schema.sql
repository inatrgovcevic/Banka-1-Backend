-- liquibase formatted sql

-- changeset transfer:1
CREATE TABLE transfers (
                           id BIGSERIAL PRIMARY KEY,
                           order_number VARCHAR(255) NOT NULL UNIQUE,
                           client_id BIGINT NOT NULL, -- Dodato zbog lakšeg pretraživanja istorije
                           from_account_number VARCHAR(255) NOT NULL,
                           to_account_number VARCHAR(255) NOT NULL,
                           initial_amount DECIMAL(19, 4) NOT NULL,
                           final_amount DECIMAL(19, 4) NOT NULL,
                           exchange_rate DECIMAL(19, 6),
                           commission DECIMAL(19, 4) NOT NULL,
                           timestamp TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
                           verification_session_id VARCHAR(255) NOT NULL UNIQUE,
    -- Standardna BaseEntity polja (po uzoru na employee-service)
                           version BIGINT DEFAULT 0,
                           deleted BOOLEAN NOT NULL DEFAULT FALSE,
                           created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
                           updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Indexi za brzo pretraživanje
CREATE INDEX idx_transfers_client_id ON transfers (client_id);
CREATE INDEX idx_transfers_order_number ON transfers (order_number);