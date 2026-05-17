--liquibase formatted sql

-- changeset jovan:1
-- PR_04 C4.5: OTC offers + option contracts.

CREATE TABLE otc_offers (
    id              BIGSERIAL    PRIMARY KEY,
    stock_ticker    VARCHAR(16)  NOT NULL,
    buyer_id        BIGINT       NOT NULL,
    seller_id       BIGINT       NOT NULL,
    amount          INTEGER      NOT NULL CHECK (amount >= 1),
    price_per_stock NUMERIC(19,2) NOT NULL CHECK (price_per_stock > 0),
    premium         NUMERIC(19,2) NOT NULL CHECK (premium >= 0),
    settlement_date DATE         NOT NULL,
    status          VARCHAR(24)  NOT NULL,
    modified_by     VARCHAR(64),
    last_modified   TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_at      TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    version         BIGINT       NOT NULL DEFAULT 0
);
CREATE INDEX idx_otc_offers_buyer_id        ON otc_offers(buyer_id);
CREATE INDEX idx_otc_offers_seller_id       ON otc_offers(seller_id);
CREATE INDEX idx_otc_offers_status          ON otc_offers(status);
CREATE INDEX idx_otc_offers_settlement_date ON otc_offers(settlement_date);

-- rollback DROP TABLE IF EXISTS otc_offers;


-- changeset jovan:2
-- PR_04 C4.5: option contracts (sklopljeni ugovori posle ACCEPTED ponude).

CREATE TABLE option_contracts (
    id              BIGSERIAL    PRIMARY KEY,
    offer_id        BIGINT       NOT NULL REFERENCES otc_offers(id),
    stock_ticker    VARCHAR(16)  NOT NULL,
    buyer_id        BIGINT       NOT NULL,
    seller_id       BIGINT       NOT NULL,
    amount          INTEGER      NOT NULL CHECK (amount >= 1),
    price_per_stock NUMERIC(19,2) NOT NULL CHECK (price_per_stock > 0),
    settlement_date DATE         NOT NULL,
    status          VARCHAR(16)  NOT NULL,
    created_at      TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    exercised_at    TIMESTAMP,
    version         BIGINT       NOT NULL DEFAULT 0
);
CREATE INDEX idx_option_contracts_buyer_id        ON option_contracts(buyer_id);
CREATE INDEX idx_option_contracts_seller_id       ON option_contracts(seller_id);
CREATE INDEX idx_option_contracts_status          ON option_contracts(status);
CREATE INDEX idx_option_contracts_settlement_date ON option_contracts(settlement_date);

-- rollback DROP TABLE IF EXISTS option_contracts;


-- changeset jovan:3
-- PR_04 C4.6: investment funds.

CREATE TABLE investment_funds (
    id                  BIGSERIAL    PRIMARY KEY,
    naziv               VARCHAR(64)  NOT NULL,
    opis                VARCHAR(1024),
    minimum_contribution NUMERIC(19,2) NOT NULL CHECK (minimum_contribution >= 0),
    manager_id          BIGINT       NOT NULL,
    likvidna_sredstva   NUMERIC(19,2) NOT NULL DEFAULT 0 CHECK (likvidna_sredstva >= 0),
    account_number      VARCHAR(16)  NOT NULL UNIQUE CHECK (account_number ~ '^[0-9]{16}$'),
    datum_kreiranja     DATE         NOT NULL DEFAULT CURRENT_DATE,
    deleted             BOOLEAN      NOT NULL DEFAULT false,
    created_at          TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    version             BIGINT       NOT NULL DEFAULT 0
);
CREATE INDEX idx_investment_funds_manager_id     ON investment_funds(manager_id);

-- rollback DROP TABLE IF EXISTS investment_funds;


-- changeset jovan:4
-- PR_04 C4.7: client fund positions (1 red po (client, fund) paru).

CREATE TABLE client_fund_positions (
    id                BIGSERIAL    PRIMARY KEY,
    client_id         BIGINT       NOT NULL,
    fund_id           BIGINT       NOT NULL REFERENCES investment_funds(id),
    total_invested    NUMERIC(19,2) NOT NULL DEFAULT 0 CHECK (total_invested >= 0),
    first_invested_at TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_modified_at  TIMESTAMP,
    version           BIGINT       NOT NULL DEFAULT 0,
    CONSTRAINT uk_client_fund_position_client_fund UNIQUE (client_id, fund_id)
);
CREATE INDEX idx_cfp_client_id ON client_fund_positions(client_id);
CREATE INDEX idx_cfp_fund_id   ON client_fund_positions(fund_id);

-- rollback DROP TABLE IF EXISTS client_fund_positions;


-- changeset jovan:5
-- PR_04 C4.8: client fund transactions (uplate i isplate, audit log).

CREATE TABLE client_fund_transactions (
    id                    BIGSERIAL    PRIMARY KEY,
    client_id             BIGINT       NOT NULL,
    fund_id               BIGINT       NOT NULL REFERENCES investment_funds(id),
    amount                NUMERIC(19,2) NOT NULL CHECK (amount > 0),
    is_inflow             BOOLEAN      NOT NULL,
    status                VARCHAR(16)  NOT NULL DEFAULT 'PENDING',
    occurred_at           TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    client_account_number VARCHAR(16)  NOT NULL,
    failure_reason        VARCHAR(255)
);
CREATE INDEX idx_cft_client_id ON client_fund_transactions(client_id);
CREATE INDEX idx_cft_fund_id   ON client_fund_transactions(fund_id);
CREATE INDEX idx_cft_status    ON client_fund_transactions(status);

-- rollback DROP TABLE IF EXISTS client_fund_transactions;
