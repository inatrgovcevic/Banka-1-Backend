--liquibase formatted sql

--changeset codex:011-add-stock-option-market-data
ALTER TABLE stock_option ADD COLUMN last_price NUMERIC(19, 8) NOT NULL DEFAULT 0;
ALTER TABLE stock_option ADD COLUMN ask NUMERIC(19, 8) NOT NULL DEFAULT 0;
ALTER TABLE stock_option ADD COLUMN bid NUMERIC(19, 8) NOT NULL DEFAULT 0;
ALTER TABLE stock_option ADD COLUMN volume BIGINT NOT NULL DEFAULT 0;
