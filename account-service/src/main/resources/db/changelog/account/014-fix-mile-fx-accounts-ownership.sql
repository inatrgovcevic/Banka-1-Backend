-- liquibase formatted sql

-- changeset account-service:14 context:dev
-- comment: PR_33 follow-up bugfix — 013-mile-interbank-test-accounts.sql je za
--          Mile-ove FX racune (EUR/USD/CHF) postavio:
--            account_concrete = 'STANDARDNI'    <- pogresno (FX ne koristi enum)
--            account_ownership_type = NULL       <- pogresno (FX zahteva non-null)
--          Hibernate AccountResponseDto konstruktor zatim baca NPE na
--          `fa.getAccountOwnershipType().name()` za FX redove → 500 na
--          GET /accounts/client/accounts.
--
--          Pravilan pattern (per 003-seed-client-accounts.sql Stefan FX seed):
--            FX:       account_concrete = NULL, account_ownership_type = 'PERSONAL'
--            CHECKING: account_concrete = 'STANDARDNI', account_ownership_type = NULL
--
--          Ne menjamo 013 (checksum drift bi sprecio reapply na postojecim
--          instancama); umesto toga UPDATE-ujemo Mile FX redove na pravu vrednost.

UPDATE account_table
SET account_concrete       = NULL,
    account_ownership_type = 'PERSONAL'
WHERE broj_racuna IN ('111000177777777721', '111000177777777722', '111000177777777723')
  AND account_type = 'FX';
