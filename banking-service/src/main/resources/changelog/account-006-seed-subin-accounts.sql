-- liquibase formatted sql

-- changeset account-service:6
-- comment: Seed RSD and EUR accounts for Mateja Subin (subin.mateja@gmail.com, client_id=9)

-- CHECKING account, RSD, 100 billion balance
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
    0, 'CHECKING', '1110001900000000111',
    'Mateja', 'Subin',
    'subin.mateja@gmail.com', 'subin.mateja', 'Tekuci racun',
    9, 1,
    100000000000.00, 100000000000.00,
    NOW(), '2031-03-25',
    c.id, 'ACTIVE',
    100000000000.00, 100000000000.00,
    0.00, 0.00,
    NULL, 'STANDARDNI', 255.00, NULL
FROM currency_table c WHERE c.oznaka = 'RSD';

-- FX account, EUR, 100 billion balance
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
    0, 'FX', '1110001900000000221',
    'Mateja', 'Subin',
    NULL, NULL, 'Devizni racun EUR',
    9, 1,
    100000000000.00, 100000000000.00,
    NOW(), '2031-03-25',
    c.id, 'ACTIVE',
    100000000000.00, 100000000000.00,
    0.00, 0.00,
    NULL, NULL, NULL, 'PERSONAL'
FROM currency_table c WHERE c.oznaka = 'EUR';
