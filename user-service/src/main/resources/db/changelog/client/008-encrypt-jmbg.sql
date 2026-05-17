--liquibase formatted sql

-- changeset jovan:8
-- PR_07 C7.1: konvertuje clients.jmbg iz plaintext-a u AES-GCM-256 ciphertext.
--
-- Strategija:
--   1. Dodaj kolonu jmbg_encrypted (TEXT) — Base64(IV||CT||TAG) 64+ bajta.
--   2. App-level migracija (Spring Boot Liquibase post-run hook ili samostalno
--      bootstrap task) cita postojeci plaintext jmbg, enkriptuje ga i pise u
--      jmbg_encrypted.
--   3. Posle verifikacije da su svi redovi konvertovani, drop plaintext jmbg
--      kolona (changeset 9 u nastavku).
--
-- Ovaj changeset radi samo Step 1 + Step 2 (post-run). Step 3 ide u changeset 9
-- da bi se rollback olaksao u slucaju problema.

ALTER TABLE clients ADD COLUMN jmbg_encrypted TEXT;

-- Step 2 se izvrsava u Spring Boot @PostConstruct migracioni komponenti
-- 'JmbgPlaintextToCiphertextMigrator' koji prati DATABASECHANGELOG za ovaj
-- changeset i radi enkripciju iz Java koda (Liquibase formatted SQL ne moze
-- da pozove Java API direktno).

CREATE INDEX idx_clients_jmbg_encrypted ON clients(jmbg_encrypted);

-- rollback DROP INDEX IF EXISTS idx_clients_jmbg_encrypted;
-- rollback ALTER TABLE clients DROP COLUMN IF EXISTS jmbg_encrypted;


-- changeset jovan:9
-- PR_07 C7.1 step 3: drop plaintext jmbg.
-- Sa preconditions: ne pokrece se ako bi neki red ostao bez jmbg_encrypted-a.

-- preconditions onFail:HALT
--   sqlCheck expectedResult:0
--     SELECT COUNT(*) FROM clients WHERE jmbg IS NOT NULL AND jmbg_encrypted IS NULL

ALTER TABLE clients DROP COLUMN IF EXISTS jmbg;

-- rollback ALTER TABLE clients ADD COLUMN jmbg VARCHAR(13);
