--liquibase formatted sql

-- changeset jovan:bank-seed-1 context:always
-- PR_13 C13.5 + PR_15 C15.5: seed za EXCHANGE racun (Banka berza) sa vlasnik=-3.
--
-- Bank RSD racun (vlasnik=-1, broj_racuna='1110001000000000012') vec postoji u
-- 002-seed-data.sql i koristi se za bank-side u BankToExchangeTransferService-u.
-- Ovde dodajemo SAMO exchange racun za drugu stranu.
--
-- Pre PR_15 ovaj fajl je referencirao tabelu 'accounts' sa 'owner_kind' kolonom
-- (kompletno netacno — prava tabela je 'account_table' sa 'vlasnik' kolonom).
-- Migracija je padala odmah na startu sa "relation 'accounts' does not exist".
--
-- Account number convention:
--   xxxxYYYYzzzzzzzzKKK
--   gde je xxxx kod banke (1110), YYYY tip vlasnika (0001=Banka, 0002=Drzava,
--   0003=Berza), ostalo serial + control digits.
--
-- Liquibase context:always znaci da seed ide u SVAKOJ DB-i (i prod i dev),
-- ne samo dev (za razliku od PR_01 C1.3 dev seed-a).

-- =========================
-- EXCHANGE RSD ACCOUNT (vlasnik=-3)
-- =========================

INSERT INTO account_table (
    version,
    account_type,
    broj_racuna,
    ime_vlasnika_racuna,
    prezime_vlasnika_racuna,
    naziv_racuna,
    vlasnik,
    zaposlen,
    stanje,
    raspolozivo_stanje,
    datum_i_vreme_kreiranja,
    datum_isteka,
    currency_id,
    status,
    dnevni_limit,
    mesecni_limit,
    dnevna_potrosnja,
    mesecna_potrosnja,
    company_id,
    account_concrete,
    odrzavanje_racuna,
    account_ownership_type
)
SELECT
    0,
    'CHECKING',
    '111000300000002012',
    'Banka',
    'Berza',
    'Exchange RSD Account',
    -3,
    -1,
    500000000.00,
    500000000.00,
    NOW(),
    NULL,
    c.id,
    'ACTIVE',
    999999999.99,
    999999999.99,
    0.00,
    0.00,
    NULL,
    'STANDARDNI',
    0.00,
    NULL
FROM currency_table c
WHERE c.oznaka = 'RSD'
  AND NOT EXISTS (SELECT 1 FROM account_table WHERE broj_racuna = '111000300000002012');

-- rollback DELETE FROM account_table WHERE broj_racuna = '111000300000002012';
