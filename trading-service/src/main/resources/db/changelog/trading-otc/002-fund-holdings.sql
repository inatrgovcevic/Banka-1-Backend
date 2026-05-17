--liquibase formatted sql

-- changeset jovan:14-c7-1
-- PR_14 C14.7: hartije u portfoliju investicionog fonda.
--
-- Pre PR_14: tabela nije postojala. {@code InvestmentFund.likvidnaSredstva}
-- je drzao samo gotovinu, a spec formula
--   "vrednost fonda = likvidna sredstva + suma vrednosti hartija"
-- bila je uvek jednaka samo prvom delu (suma=0). Sada FUND_INVEST saga moze
-- da kupuje hartije za fond koje se ovde belezem, a FUND_REDEEM_WITH_LIQUIDATION
-- ih prodaje nazad pre nego sto isplati klijenta.

CREATE TABLE IF NOT EXISTS fund_holdings (
    id              BIGSERIAL    PRIMARY KEY,
    fund_id         BIGINT       NOT NULL REFERENCES investment_funds(id),
    stock_ticker    VARCHAR(16)  NOT NULL,
    quantity        INTEGER      NOT NULL CHECK (quantity >= 0),
    avg_unit_price  NUMERIC(19,4) NOT NULL CHECK (avg_unit_price >= 0),
    deleted         BOOLEAN      NOT NULL DEFAULT false,
    created_at      TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMP,
    version         BIGINT       NOT NULL DEFAULT 0,
    CONSTRAINT uk_fund_holdings_fund_ticker UNIQUE (fund_id, stock_ticker)
);

CREATE INDEX IF NOT EXISTS idx_fund_holdings_fund_id      ON fund_holdings(fund_id);
CREATE INDEX IF NOT EXISTS idx_fund_holdings_stock_ticker ON fund_holdings(stock_ticker);

-- rollback DROP TABLE IF EXISTS fund_holdings;
