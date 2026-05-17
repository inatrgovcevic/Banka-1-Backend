-- liquibase formatted sql

-- changeset client-service:12 context:dev
-- comment: PR_32 Phase 17 — Mile Interbank dedicated handshake test client.
--          Paritetan sa Banka 2 (C-6 "Mile Interbank") koji su oni dodali u svoj
--          seed za handshake testiranje. Kod nas postaje C-11 (sledeci slobodan ID
--          posle 10 postojecih). Koristi se kao "test target" seller u
--          POST /negotiations + 2PC accept scenarijima.
--
--          Email matchuje Tim 2 simboliku (mile.interbank@banka.rs).
--          Password 'admin123' (isti Argon2 hash kao ostali dev klijenti).
--          Role CLIENT_TRADING + OTC_TRADE permission da moze da ucestvuje u OTC.

INSERT INTO clients (ime, prezime, datum_rodjenja, pol, email, broj_telefona, adresa,
                     jmbg, password, role, version, deleted, aktivan)
SELECT 'Mile', 'Interbank', 568079999000, 'M', 'mile.interbank@banka.rs',
       '+381641111011', 'Inter-bank Test, Beograd', '1505988710011',
       '$argon2id$v=19$m=65536,t=3,p=1$cml4YnF1MGJOaG5md1cxOQ$kTOwNnDZmFymtQgsCUgpYFUJC9eV8wmpBCEldnS3XeE',
       'CLIENT_TRADING', 0, false, true
WHERE NOT EXISTS (SELECT 1 FROM clients WHERE email = 'mile.interbank@banka.rs');

-- MARGIN_TRADE + (role implicit CLIENT_TRADING vec daje BANKING_BASIC, SECURITIES_TRADE_LIMITED, OTC_TRADE)
INSERT INTO client_permissions (client_id, permission)
SELECT c.id, 'MARGIN_TRADE'
FROM clients c
WHERE c.email = 'mile.interbank@banka.rs'
  AND NOT EXISTS (
      SELECT 1 FROM client_permissions cp
      WHERE cp.client_id = c.id AND cp.permission = 'MARGIN_TRADE'
  );
