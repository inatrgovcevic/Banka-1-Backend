-- liquibase formatted sql

-- changeset client-service:11 context:dev
-- PR_31: Distinktni dev klijenti koji NISU istovremeno u employee seed-u.
-- Razlog: ranije seed (004-seed-data.sql) je koristio iste email-ove kao employee seed
-- (npr. marko.markovic@banka.com je BIO i klijent i zaposleni). Login flow u FE prvo
-- pokusava klijent endpoint pa zaposleni kao fallback — sa istim email-om uvek hita
-- klijent prvi i nikad ne testira zaposleni put kroz isti email. Ovi novi klijenti
-- imaju eksplicitno @klijent.rs domen pa nema kolizije.
--
-- Lozinka za sve: 'admin123' (Argon2 hash je ista konstanta kao u 004-seed-data.sql).
-- NEVER seed in production — context:dev.
INSERT INTO clients (ime, prezime, datum_rodjenja, pol, email, broj_telefona, adresa, jmbg, password, role, version, deleted, aktivan)
VALUES
    ('Petar',   'Petrovic',  694310400000, 'M', 'petar.test@klijent.rs',   '+381641111001', 'Test 1, Beograd',    '0107991710101', '$argon2id$v=19$m=65536,t=3,p=1$cml4YnF1MGJOaG5md1cxOQ$kTOwNnDZmFymtQgsCUgpYFUJC9eV8wmpBCEldnS3XeE', 'CLIENT_BASIC',   0, false, true),
    ('Mira',    'Miric',     757382400000, 'Z', 'mira.test@klijent.rs',    '+381641111002', 'Test 2, Beograd',    '0511994785102', '$argon2id$v=19$m=65536,t=3,p=1$cml4YnF1MGJOaG5md1cxOQ$kTOwNnDZmFymtQgsCUgpYFUJC9eV8wmpBCEldnS3XeE', 'CLIENT_BASIC',   0, false, true),
    ('Dejan',   'Dejanovic', 883612800000, 'M', 'dejan.test@klijent.rs',   '+381641111003', 'Test 3, Novi Sad',   '2207998785103', '$argon2id$v=19$m=65536,t=3,p=1$cml4YnF1MGJOaG5md1cxOQ$kTOwNnDZmFymtQgsCUgpYFUJC9eV8wmpBCEldnS3XeE', 'CLIENT_TRADING', 0, false, true),
    ('Tijana',  'Tijanic',   946684800000, 'Z', 'tijana.test@klijent.rs',  '+381641111004', 'Test 4, Nis',        '1804000710104', '$argon2id$v=19$m=65536,t=3,p=1$cml4YnF1MGJOaG5md1cxOQ$kTOwNnDZmFymtQgsCUgpYFUJC9eV8wmpBCEldnS3XeE', 'CLIENT_TRADING', 0, false, true),
    ('Ognjen',  'Ognjenovic',631152000000, 'M', 'ognjen.test@klijent.rs',  '+381641111005', 'Test 5, Kragujevac', '1402990785105', '$argon2id$v=19$m=65536,t=3,p=1$cml4YnF1MGJOaG5md1cxOQ$kTOwNnDZmFymtQgsCUgpYFUJC9eV8wmpBCEldnS3XeE', 'CLIENT_BASIC',   0, false, true);

-- Permission seed: dejan dobija MARGIN_TRADE permisiju (kao u 007-subin).
-- Napomena: client-service Permission enum ima samo MARGIN_TRADE — ostale permisije
-- (BANKING_BASIC, SECURITIES_TRADE_LIMITED, OTC_TRADE) se izvlace iz `role` polja
-- na backend-u (CLIENT_BASIC vs CLIENT_TRADING). Pa nije potrebno unositi ih.
INSERT INTO client_permissions (client_id, permission)
SELECT c.id, 'MARGIN_TRADE'
FROM clients c
WHERE c.email IN ('dejan.test@klijent.rs');
