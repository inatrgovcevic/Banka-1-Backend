--liquibase formatted sql
-- changeset interbank:32-interbank-003-dev-negotiations-seed context:dev
-- comment: DEV-ONLY rich seed za inter-bank handshake testiranje. Sample
--          interbank_negotiations + interbank_contracts za testiranje GET/PUT
--          protiv pre-postojecih pregovora bez moranja da Tim 2 prvo POST-uje.

-- ============================================================================
-- 4 sample inter-bank pregovora + 1 sklopljeni opcioni ugovor, sva 4 stanja:
--   neg-dev-001 — ongoing, ON nas turn (lastModifiedBy = Banka 2 = C-2)
--                  → Tim 2 moze da uradi GET /negotiations/111/neg-dev-001
--                  → Mi treba da uradimo PUT counter-offer (turn check passes)
--   neg-dev-002 — ongoing, ON njihov turn (lastModifiedBy = Banka 1 = C-5)
--                  → Tim 2 moze da uradi PUT counter-offer (turn check passes)
--                  → Mi NE smemo PUT (turn violation → 409 KRITICNO za Tim 2 §6.3)
--   neg-dev-003 — ongoing, blizu accept-a (price + premium podesni)
--                  → Tim 2 moze da uradi GET /accept (sinhroni 2PC)
--   neg-dev-004 — zatvoren (isOngoing=false) — istekli ili odustao
--                  → svaki PUT na ovaj treba 409 NegotiationClosedException
--   otc-dev-001 — sklopljen contract (status=ACTIVE), seller=C-5, buyer=Banka2:C-2
--                  → testira GET /interbank/user + dohvat seller-side contract listi
-- ============================================================================

-- neg-dev-001: ongoing, on nas turn (Banka 2 zadnji modifikator)
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

-- neg-dev-002: ongoing, on njihov turn (Banka 1 zadnji modifikator)
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

-- neg-dev-003: ongoing, blizu accept-a (Banka 2 zadnji, mi smemo accept)
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

-- neg-dev-004: zatvoren (isOngoing=false), testira NegotiationClosedException
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

-- otc-dev-001: sklopljen opcioni ugovor sa Banka 2 buyer-om (status=ACTIVE)
-- Linkovan na neg-dev-004 (koja je sad zatvorena posle accept-a).
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

-- ============================================================================
-- Sumarni snapshot:
--   interbank_negotiations: 4 redova (3 ongoing + 1 closed)
--   interbank_contracts: 1 red (ACTIVE, linkovan na closed neg-dev-004)
--
-- Tim 2 test scenariji koji ovo koriste:
--   curl GET /negotiations/111/neg-dev-001 → 200 + JSON (ongoing, lastMod=Banka2)
--   curl PUT /negotiations/111/neg-dev-002 → 204 (Banka 2 ima turn — Banka 1 zadnji)
--   curl PUT /negotiations/111/neg-dev-001 → 409 (turn violation — Banka 2 zadnji)
--   curl GET /negotiations/111/neg-dev-003/accept → 204 (Banka 2 prihvata)
--   curl PUT /negotiations/111/neg-dev-004 → 409 NegotiationClosedException
--   curl DELETE /negotiations/111/neg-dev-002 → 204 (zatvara se)
-- ============================================================================
