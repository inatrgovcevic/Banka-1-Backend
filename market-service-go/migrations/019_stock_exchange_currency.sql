--liquibase formatted sql

--changeset codex:010-widen-stock-exchange-currency
ALTER TABLE stock_exchange ALTER COLUMN currency TYPE VARCHAR(100);
