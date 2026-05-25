-- +goose Up
-- +goose StatementBegin
-- DEV-ONLY rich seed za inter-bank handshake testiranje.
-- 4 sample inter-bank pregovora + 1 sklopljeni opcioni ugovor, sva 4 stanja:
--   neg-dev-001 — ongoing, ON nas turn (lastModifiedBy = Banka 2 = C-2)
--   neg-dev-002 — ongoing, ON njihov turn (lastModifiedBy = Banka 1 = C-5)
--   neg-dev-003 — ongoing, blizu accept-a (price + premium podesni)
--   neg-dev-004 — zatvoren (isOngoing=false)
--   otc-dev-001 — sklopljen contract (status=ACTIVE), seller=C-5, buyer=Banka2:C-2

INSERT INTO interbank_negotiations (
    id, buyer_routing_number, buyer_id, seller_routing_number, seller_id,
    stock_ticker, amount, price_currency, price_amount, premium_currency, premium_amount,
    settlement_date, last_modified_by_routing, last_modified_by_id,
    is_ongoing, is_authoritative, remote_negotiation_id, linked_local_offer_id,
    version, created_at, last_modified_at
)
SELECT 'neg-dev-001', 222, 'C-2', 111, 'C-5',
       'AAPL', 10, 'USD', 195.0000, 'USD', 500.0000,
       NOW() + INTERVAL '30 days', 222, 'C-2',
       true, true, NULL, NULL,
       0, NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM interbank_negotiations WHERE id = 'neg-dev-001');

INSERT INTO interbank_negotiations (
    id, buyer_routing_number, buyer_id, seller_routing_number, seller_id,
    stock_ticker, amount, price_currency, price_amount, premium_currency, premium_amount,
    settlement_date, last_modified_by_routing, last_modified_by_id,
    is_ongoing, is_authoritative, remote_negotiation_id, linked_local_offer_id,
    version, created_at, last_modified_at
)
SELECT 'neg-dev-002', 222, 'C-2', 111, 'C-1',
       'MSFT', 5, 'USD', 420.0000, 'USD', 800.0000,
       NOW() + INTERVAL '45 days', 111, 'C-1',
       true, true, NULL, NULL,
       0, NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM interbank_negotiations WHERE id = 'neg-dev-002');

INSERT INTO interbank_negotiations (
    id, buyer_routing_number, buyer_id, seller_routing_number, seller_id,
    stock_ticker, amount, price_currency, price_amount, premium_currency, premium_amount,
    settlement_date, last_modified_by_routing, last_modified_by_id,
    is_ongoing, is_authoritative, remote_negotiation_id, linked_local_offer_id,
    version, created_at, last_modified_at
)
SELECT 'neg-dev-003', 222, 'C-4', 111, 'C-3',
       'GOOGL', 8, 'USD', 145.0000, 'USD', 300.0000,
       NOW() + INTERVAL '60 days', 222, 'C-4',
       true, true, NULL, NULL,
       0, NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM interbank_negotiations WHERE id = 'neg-dev-003');

INSERT INTO interbank_negotiations (
    id, buyer_routing_number, buyer_id, seller_routing_number, seller_id,
    stock_ticker, amount, price_currency, price_amount, premium_currency, premium_amount,
    settlement_date, last_modified_by_routing, last_modified_by_id,
    is_ongoing, is_authoritative, remote_negotiation_id, linked_local_offer_id,
    version, created_at, last_modified_at
)
SELECT 'neg-dev-004', 222, 'C-2', 111, 'C-5',
       'TSLA', 20, 'USD', 250.0000, 'USD', 1200.0000,
       NOW() + INTERVAL '15 days', 111, 'C-5',
       false, true, NULL, NULL,
       0, NOW() - INTERVAL '2 days', NOW() - INTERVAL '1 day'
WHERE NOT EXISTS (SELECT 1 FROM interbank_negotiations WHERE id = 'neg-dev-004');

INSERT INTO interbank_contracts (
    id, negotiation_id, buyer_routing_number, buyer_id, seller_routing_number, seller_id,
    stock_ticker, amount, strike_currency, strike_amount, settlement_date, status,
    option_pseudo_owner_routing, option_pseudo_owner_id,
    version, created_at, exercised_at, expired_at
)
SELECT 'otc-dev-001', 'neg-dev-004', 222, 'C-2', 111, 'C-5',
       'TSLA', 20, 'USD', 250.0000,
       NOW() + INTERVAL '15 days', 'ACTIVE',
       222, 'C-2',
       0, NOW() - INTERVAL '1 day', NULL, NULL
WHERE NOT EXISTS (SELECT 1 FROM interbank_contracts WHERE id = 'otc-dev-001');
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM interbank_contracts WHERE id = 'otc-dev-001';
DELETE FROM interbank_negotiations WHERE id IN ('neg-dev-001', 'neg-dev-002', 'neg-dev-003', 'neg-dev-004');
-- +goose StatementEnd
