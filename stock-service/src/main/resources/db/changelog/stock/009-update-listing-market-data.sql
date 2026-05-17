--liquibase formatted sql

--changeset codex:009-update-listing-market-data
ALTER TABLE listing
    ADD COLUMN change NUMERIC(19, 8) NOT NULL DEFAULT 0;

ALTER TABLE listing_daily_price_info
    ADD CONSTRAINT uk_listing_daily_price_info_listing_date UNIQUE (listing_id, date);
