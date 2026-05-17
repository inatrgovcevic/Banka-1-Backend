--liquibase formatted sql
-- changeset interbank:32-trading-004-mile-interbank-portfolio context:dev
-- comment: PR_32 Phase 17b — Mile Interbank (C-15) AAPL portfolio za handshake
--          paritet sa Tim 2 (oni dodali svog Mile C-6 sa 100 AAPL public 25).
--          Tim 2 ce kao buyer slati POST /negotiations sa
--          sellerId={111, "C-15"}, pa nas seed mora da ima 100 AAPL javnih 25.
--
--          NAPOMENA: Mile-ov client_id je 15 (ne 11) jer 009-distinct-test-clients.sql
--          je dodao 5 dodatnih klijenata (Mira C-11, Dejan C-12, Tijana C-13,
--          Ognjen C-14, Petar C-10) pre nego sto 010-mile-interbank dodaje Mile.
--          Trading-service je drugacija baza pa cross-db SELECT FROM clients ne radi —
--          hardkode-ovan ID. Ako sledeci PR doda klijenta izmedju 009 i 010,
--          azurirati ovaj ID + dokumentaciju.

INSERT INTO portfolio (user_id, listing_id, listing_type, quantity, average_purchase_price,
                       is_public, public_quantity, last_modified, reserved_quantity)
SELECT 15, 1, 'STOCK', 100, 178.5000, true, 25, CURRENT_TIMESTAMP, 0
WHERE NOT EXISTS (SELECT 1 FROM portfolio WHERE user_id=15 AND listing_id=1);

-- Sumarno za AAPL posle ovog seed-a (totalna javna AAPL kolicina u Banka 1):
--   C-1 (Marko)  — 100 javnih
--   C-3 (Jovana) —  20 javnih
--   C-5 (Milica) —  80 javnih
--   C-15 (Mile)  —  25 javnih  ← NOVI dedicated test target
--   = 4 seller-a, ukupno 225 javnih AAPL
