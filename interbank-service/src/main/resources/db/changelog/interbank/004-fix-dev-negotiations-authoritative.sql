--liquibase formatted sql

-- changeset interbank:33-followup-004-fix-dev-negotiations-authoritative context:dev
-- comment: PR_33 follow-up bugfix — dev seed 003-dev-negotiations-seed.sql
--          postavio je `is_authoritative=true` za sve cross-bank pregovore,
--          ali Banka 2 (buyer_routing_number=222) ih je u stvarnosti inicirala
--          → buyer bank je authoritative per protokol §3.2.
--
--          Posledica buga: wrapper helper `remoteForeignBankIdOf` u
--          `InterbankOtcOutboundService` (PR_33 verzija) je vracao
--          `(myRouting=111, e.getId())` kad je `isAuthoritative=true` →
--          frontend mapirao `routingNumber=111 → "Naša banka"` badge.
--
--          Pravilan flag: false (mi smo replica/mirror — partner authoritative).

UPDATE interbank_negotiations
SET is_authoritative = false
WHERE buyer_routing_number = 222 AND seller_routing_number = 111;

-- Sanity: settle handshake target negotiations (s9, s13) explicitly to false too.
UPDATE interbank_negotiations
SET is_authoritative = false
WHERE id IN ('neg-handshake-s9', 'neg-handshake-s13');
