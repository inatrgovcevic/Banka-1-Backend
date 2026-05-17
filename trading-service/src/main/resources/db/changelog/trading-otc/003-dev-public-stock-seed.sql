--liquibase formatted sql
-- changeset interbank:32-trading-003-dev-public-stock-seed context:dev
-- comment: DEV-ONLY rich seed za inter-bank handshake testiranje. Aktivira se
--          kad je LIQUIBASE_CONTEXTS=dev. Skipped u prod profilu.
--          Cilj: GET /public-stock vraca raznoliku listu tickera + prodavaca.

-- Public stock portfolios: 18 redova spread across 7 klijenata (id 1-7) i 8 tickera
-- (AAPL=1, MSFT=2, GOOGL=3, AMZN=4, TSLA=5, IBM=6, GS=7, JPM=8).
-- Quantity i publicQuantity su raznoliki da Tim 2 moze da pravi pregovore razlicitih
-- velicina. average_purchase_price je za izgled portfolija.

-- ============================================================================
-- Klijent 1 (Marko Markovic, C-1) — najveci portfolio, "whale" seller
-- ============================================================================
INSERT INTO portfolio (user_id, listing_id, listing_type, quantity, average_purchase_price,
                       is_public, public_quantity, last_modified, reserved_quantity)
SELECT 1, 1, 'STOCK', 200, 175.5000, true, 100, CURRENT_TIMESTAMP, 0
WHERE NOT EXISTS (SELECT 1 FROM portfolio WHERE user_id=1 AND listing_id=1);

INSERT INTO portfolio (user_id, listing_id, listing_type, quantity, average_purchase_price,
                       is_public, public_quantity, last_modified, reserved_quantity)
SELECT 1, 2, 'STOCK', 150, 415.2500, true, 75, CURRENT_TIMESTAMP, 0
WHERE NOT EXISTS (SELECT 1 FROM portfolio WHERE user_id=1 AND listing_id=2);

INSERT INTO portfolio (user_id, listing_id, listing_type, quantity, average_purchase_price,
                       is_public, public_quantity, last_modified, reserved_quantity)
SELECT 1, 5, 'STOCK', 80, 245.0000, true, 50, CURRENT_TIMESTAMP, 0
WHERE NOT EXISTS (SELECT 1 FROM portfolio WHERE user_id=1 AND listing_id=5);

-- ============================================================================
-- Klijent 2 (Ana Anic, C-2) — diversifikovan, srednji portfolio
-- ============================================================================
INSERT INTO portfolio (user_id, listing_id, listing_type, quantity, average_purchase_price,
                       is_public, public_quantity, last_modified, reserved_quantity)
SELECT 2, 3, 'STOCK', 50, 142.7500, true, 25, CURRENT_TIMESTAMP, 0
WHERE NOT EXISTS (SELECT 1 FROM portfolio WHERE user_id=2 AND listing_id=3);

INSERT INTO portfolio (user_id, listing_id, listing_type, quantity, average_purchase_price,
                       is_public, public_quantity, last_modified, reserved_quantity)
SELECT 2, 4, 'STOCK', 30, 190.5000, true, 15, CURRENT_TIMESTAMP, 0
WHERE NOT EXISTS (SELECT 1 FROM portfolio WHERE user_id=2 AND listing_id=4);

INSERT INTO portfolio (user_id, listing_id, listing_type, quantity, average_purchase_price,
                       is_public, public_quantity, last_modified, reserved_quantity)
SELECT 2, 8, 'STOCK', 25, 220.0000, true, 10, CURRENT_TIMESTAMP, 0
WHERE NOT EXISTS (SELECT 1 FROM portfolio WHERE user_id=2 AND listing_id=8);

-- ============================================================================
-- Klijent 3 (Jovana Jovanovic, C-3) — mali balansiran portfolio
-- ============================================================================
INSERT INTO portfolio (user_id, listing_id, listing_type, quantity, average_purchase_price,
                       is_public, public_quantity, last_modified, reserved_quantity)
SELECT 3, 1, 'STOCK', 40, 180.0000, true, 20, CURRENT_TIMESTAMP, 0
WHERE NOT EXISTS (SELECT 1 FROM portfolio WHERE user_id=3 AND listing_id=1);

INSERT INTO portfolio (user_id, listing_id, listing_type, quantity, average_purchase_price,
                       is_public, public_quantity, last_modified, reserved_quantity)
SELECT 3, 6, 'STOCK', 60, 165.5000, true, 30, CURRENT_TIMESTAMP, 0
WHERE NOT EXISTS (SELECT 1 FROM portfolio WHERE user_id=3 AND listing_id=6);

-- ============================================================================
-- Klijent 4 (Stefan Stefanovic, C-4) — fokus na finansije
-- ============================================================================
INSERT INTO portfolio (user_id, listing_id, listing_type, quantity, average_purchase_price,
                       is_public, public_quantity, last_modified, reserved_quantity)
SELECT 4, 7, 'STOCK', 35, 425.0000, true, 20, CURRENT_TIMESTAMP, 0
WHERE NOT EXISTS (SELECT 1 FROM portfolio WHERE user_id=4 AND listing_id=7);

INSERT INTO portfolio (user_id, listing_id, listing_type, quantity, average_purchase_price,
                       is_public, public_quantity, last_modified, reserved_quantity)
SELECT 4, 8, 'STOCK', 45, 215.0000, true, 25, CURRENT_TIMESTAMP, 0
WHERE NOT EXISTS (SELECT 1 FROM portfolio WHERE user_id=4 AND listing_id=8);

-- ============================================================================
-- Klijent 5 (Milica Milic, C-5) — tech-heavy, AAPL whale (Tim 2 spec primer)
-- ============================================================================
INSERT INTO portfolio (user_id, listing_id, listing_type, quantity, average_purchase_price,
                       is_public, public_quantity, last_modified, reserved_quantity)
SELECT 5, 1, 'STOCK', 100, 178.2500, true, 80, CURRENT_TIMESTAMP, 0
WHERE NOT EXISTS (SELECT 1 FROM portfolio WHERE user_id=5 AND listing_id=1);

INSERT INTO portfolio (user_id, listing_id, listing_type, quantity, average_purchase_price,
                       is_public, public_quantity, last_modified, reserved_quantity)
SELECT 5, 2, 'STOCK', 60, 412.5000, true, 40, CURRENT_TIMESTAMP, 0
WHERE NOT EXISTS (SELECT 1 FROM portfolio WHERE user_id=5 AND listing_id=2);

INSERT INTO portfolio (user_id, listing_id, listing_type, quantity, average_purchase_price,
                       is_public, public_quantity, last_modified, reserved_quantity)
SELECT 5, 3, 'STOCK', 35, 144.5000, true, 25, CURRENT_TIMESTAMP, 0
WHERE NOT EXISTS (SELECT 1 FROM portfolio WHERE user_id=5 AND listing_id=3);

-- ============================================================================
-- Klijent 6 (Nikola Nikolic, C-6) — privatni portfolio (NIJE javan, kontrol)
-- ============================================================================
-- Ovaj NE postavlja public — testira da li GET /public-stock pravilno filtrira
INSERT INTO portfolio (user_id, listing_id, listing_type, quantity, average_purchase_price,
                       is_public, public_quantity, last_modified, reserved_quantity)
SELECT 6, 4, 'STOCK', 50, 188.0000, false, 0, CURRENT_TIMESTAMP, 0
WHERE NOT EXISTS (SELECT 1 FROM portfolio WHERE user_id=6 AND listing_id=4);

INSERT INTO portfolio (user_id, listing_id, listing_type, quantity, average_purchase_price,
                       is_public, public_quantity, last_modified, reserved_quantity)
SELECT 6, 5, 'STOCK', 20, 248.0000, false, 0, CURRENT_TIMESTAMP, 0
WHERE NOT EXISTS (SELECT 1 FROM portfolio WHERE user_id=6 AND listing_id=5);

-- ============================================================================
-- Klijent 7 (Jelena Jelic, C-7) — small positions, multi-ticker
-- ============================================================================
INSERT INTO portfolio (user_id, listing_id, listing_type, quantity, average_purchase_price,
                       is_public, public_quantity, last_modified, reserved_quantity)
SELECT 7, 2, 'STOCK', 15, 418.0000, true, 10, CURRENT_TIMESTAMP, 0
WHERE NOT EXISTS (SELECT 1 FROM portfolio WHERE user_id=7 AND listing_id=2);

INSERT INTO portfolio (user_id, listing_id, listing_type, quantity, average_purchase_price,
                       is_public, public_quantity, last_modified, reserved_quantity)
SELECT 7, 4, 'STOCK', 20, 191.5000, true, 12, CURRENT_TIMESTAMP, 0
WHERE NOT EXISTS (SELECT 1 FROM portfolio WHERE user_id=7 AND listing_id=4);

INSERT INTO portfolio (user_id, listing_id, listing_type, quantity, average_purchase_price,
                       is_public, public_quantity, last_modified, reserved_quantity)
SELECT 7, 7, 'STOCK', 18, 428.0000, true, 12, CURRENT_TIMESTAMP, 0
WHERE NOT EXISTS (SELECT 1 FROM portfolio WHERE user_id=7 AND listing_id=7);

-- ============================================================================
-- Sumarni snapshot za debugging — koliko public stocks ima na kraju
-- ============================================================================
-- Expected output: 16 javnih portfolio redova (svi osim user_id=6 koji ima 2 privatna).
-- GET /public-stock kroz interbank-service ce po tikeru agregirati prodavce:
--   AAPL (C-1=100, C-3=20, C-5=80) — 3 prodavca, ukupno 200 jedinica
--   MSFT (C-1=75, C-5=40, C-7=10)  — 3 prodavca, 125 jedinica
--   GOOGL (C-2=25, C-5=25)         — 2 prodavca, 50 jedinica
--   AMZN (C-2=15, C-7=12)          — 2 prodavca, 27 jedinica
--   TSLA (C-1=50)                   — 1 prodavac, 50 jedinica
--   IBM (C-3=30)                    — 1 prodavac, 30 jedinica
--   GS (C-4=20, C-7=12)             — 2 prodavca, 32 jedinica
--   JPM (C-2=10, C-4=25)            — 2 prodavca, 35 jedinica
-- Plus 1 actuary (E-1, Admin) ima jos jedan portfolio za supervisor flow:
INSERT INTO portfolio (user_id, listing_id, listing_type, quantity, average_purchase_price,
                       is_public, public_quantity, last_modified, reserved_quantity)
SELECT 1, 4, 'STOCK', 25, 192.0000, true, 15, CURRENT_TIMESTAMP, 0
WHERE NOT EXISTS (SELECT 1 FROM portfolio WHERE user_id=1 AND listing_id=4);

