-- liquibase formatted sql

-- changeset account-service:13 context:dev
-- comment: PR_32 Phase 17 — Mile Interbank dedicated handshake test accounts.
--          Paritetan sa Banka 2 (C-6 ima accounts sa prefiksom 222000177777777...).
--          Kod nas: prefix 1110001777777777 (16 cifara) + 2 cifre type+id (ukupno 18).
--          Vlasnik ID = (SELECT id FROM clients WHERE email='mile.interbank@banka.rs')
--          — resolve-uje se pri INSERT-u jer Liquibase moze da menja redosled.
--          Velika stanja (5M RSD + 50K po deviznoj) da svi inter-bank scenariji prolaze.

-- ============================================================================
-- 1. RSD CHECKING — 5,000,000 RSD
-- ============================================================================
INSERT INTO account_table (
    version, account_type, broj_racuna,
    ime_vlasnika_racuna, prezime_vlasnika_racuna,
    email, username, naziv_racuna,
    vlasnik, zaposlen,
    stanje, raspolozivo_stanje,
    datum_i_vreme_kreiranja, datum_isteka,
    currency_id, status,
    dnevni_limit, mesecni_limit,
    dnevna_potrosnja, mesecna_potrosnja,
    company_id, account_concrete, odrzavanje_racuna, account_ownership_type
)
SELECT
    0, 'CHECKING', '111000177777777711',
    'Mile', 'Interbank',
    'mile.interbank@banka.rs', 'mile.interbank', 'Mile Interbank RSD',
    15, 1,
    5000000.00, 5000000.00,
    NOW(), '2031-03-25',
    c.id, 'ACTIVE',
    5000000.00, 5000000.00,
    0.00, 0.00,
    NULL, 'STANDARDNI', 255.00, NULL
FROM currency_table c WHERE c.oznaka = 'RSD'
AND NOT EXISTS (SELECT 1 FROM account_table WHERE broj_racuna = '111000177777777711');

-- ============================================================================
-- 2. EUR FX — 50,000 EUR
-- ============================================================================
INSERT INTO account_table (
    version, account_type, broj_racuna,
    ime_vlasnika_racuna, prezime_vlasnika_racuna,
    email, username, naziv_racuna,
    vlasnik, zaposlen,
    stanje, raspolozivo_stanje,
    datum_i_vreme_kreiranja, datum_isteka,
    currency_id, status,
    dnevni_limit, mesecni_limit,
    dnevna_potrosnja, mesecna_potrosnja,
    company_id, account_concrete, odrzavanje_racuna, account_ownership_type
)
SELECT
    0, 'FX', '111000177777777721',
    'Mile', 'Interbank',
    'mile.interbank@banka.rs', 'mile.interbank', 'Mile Interbank EUR',
    15, 1,
    50000.00, 50000.00,
    NOW(), '2031-03-25',
    c.id, 'ACTIVE',
    50000.00, 50000.00,
    0.00, 0.00,
    NULL, 'STANDARDNI', 255.00, NULL
FROM currency_table c WHERE c.oznaka = 'EUR'
AND NOT EXISTS (SELECT 1 FROM account_table WHERE broj_racuna = '111000177777777721');

-- ============================================================================
-- 3. USD FX — 50,000 USD
-- ============================================================================
INSERT INTO account_table (
    version, account_type, broj_racuna,
    ime_vlasnika_racuna, prezime_vlasnika_racuna,
    email, username, naziv_racuna,
    vlasnik, zaposlen,
    stanje, raspolozivo_stanje,
    datum_i_vreme_kreiranja, datum_isteka,
    currency_id, status,
    dnevni_limit, mesecni_limit,
    dnevna_potrosnja, mesecna_potrosnja,
    company_id, account_concrete, odrzavanje_racuna, account_ownership_type
)
SELECT
    0, 'FX', '111000177777777722',
    'Mile', 'Interbank',
    'mile.interbank@banka.rs', 'mile.interbank', 'Mile Interbank USD',
    15, 1,
    50000.00, 50000.00,
    NOW(), '2031-03-25',
    c.id, 'ACTIVE',
    50000.00, 50000.00,
    0.00, 0.00,
    NULL, 'STANDARDNI', 255.00, NULL
FROM currency_table c WHERE c.oznaka = 'USD'
AND NOT EXISTS (SELECT 1 FROM account_table WHERE broj_racuna = '111000177777777722');

-- ============================================================================
-- 4. CHF FX — 50,000 CHF
-- ============================================================================
INSERT INTO account_table (
    version, account_type, broj_racuna,
    ime_vlasnika_racuna, prezime_vlasnika_racuna,
    email, username, naziv_racuna,
    vlasnik, zaposlen,
    stanje, raspolozivo_stanje,
    datum_i_vreme_kreiranja, datum_isteka,
    currency_id, status,
    dnevni_limit, mesecni_limit,
    dnevna_potrosnja, mesecna_potrosnja,
    company_id, account_concrete, odrzavanje_racuna, account_ownership_type
)
SELECT
    0, 'FX', '111000177777777723',
    'Mile', 'Interbank',
    'mile.interbank@banka.rs', 'mile.interbank', 'Mile Interbank CHF',
    15, 1,
    50000.00, 50000.00,
    NOW(), '2031-03-25',
    c.id, 'ACTIVE',
    50000.00, 50000.00,
    0.00, 0.00,
    NULL, 'STANDARDNI', 255.00, NULL
FROM currency_table c WHERE c.oznaka = 'CHF'
AND NOT EXISTS (SELECT 1 FROM account_table WHERE broj_racuna = '111000177777777723');
