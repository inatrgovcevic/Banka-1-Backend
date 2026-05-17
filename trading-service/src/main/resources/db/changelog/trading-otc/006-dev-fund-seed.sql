--liquibase formatted sql

-- changeset interbank:33-trading-006-dev-fund-seed context:dev
-- comment: PR_33 follow-up — DEV-ONLY investicioni fond + Mile-ova pozicija u njemu.
--          Sluzi da Fund discovery + "Moji fondovi" ekrani na frontend-u imaju
--          podatke za prikaz, umesto prazne liste koja moze izgledati kao bug.
--
--          Fond: "Konzervativni RSD" — minimum_contribution 5000 RSD, manager
--          je employee Petar Petrovic (id=2 iz employee 002-seed-data.sql,
--          SUPERVISOR, broker tim).
--
--          Mile Interbank (client_id=15) ima 75000 RSD ulozeno (total_invested).
--          Plus jedan fund_holdings red (AAPL × 10 prosecna 180 USD) — da
--          ukupna vrednost fonda > samo likvidna_sredstva (testira fund value
--          aggregator put fix-a iz PR_30 KRIT 6).

INSERT INTO investment_funds (naziv, opis, minimum_contribution, manager_id,
                              likvidna_sredstva, account_number, datum_kreiranja,
                              deleted, created_at, version)
SELECT 'Konzervativni RSD',
       'Niska volatilnost, RSD bazna valuta. Pretezno bankarski depoziti + odabrane stabilne akcije sa NYSE/NASDAQ-a.',
       5000.00, 2, 250000.00, '1110001999999911', CURRENT_DATE - INTERVAL '90 days',
       false, CURRENT_TIMESTAMP - INTERVAL '90 days', 0
WHERE NOT EXISTS (SELECT 1 FROM investment_funds WHERE naziv='Konzervativni RSD');

-- Drugi fond za varietet (Fund discovery treba bar 2 reda da filter/sort imaju smisla).
INSERT INTO investment_funds (naziv, opis, minimum_contribution, manager_id,
                              likvidna_sredstva, account_number, datum_kreiranja,
                              deleted, created_at, version)
SELECT 'Agresivni Tech USD',
       'Visok rizik, koncentracija na FAANG + EV. USD denominovan, hedzovano protiv RSD pomeranja.',
       50000.00, 2, 180000.00, '1110001999999921', CURRENT_DATE - INTERVAL '45 days',
       false, CURRENT_TIMESTAMP - INTERVAL '45 days', 0
WHERE NOT EXISTS (SELECT 1 FROM investment_funds WHERE naziv='Agresivni Tech USD');

-- Mile pozicija u "Konzervativni RSD" fondu — uloze 75000 RSD pre 30 dana.
INSERT INTO client_fund_positions (client_id, fund_id, total_invested, first_invested_at,
                                    last_modified_at, version)
SELECT 15, f.id, 75000.00,
       CURRENT_TIMESTAMP - INTERVAL '30 days', CURRENT_TIMESTAMP - INTERVAL '30 days', 0
FROM investment_funds f
WHERE f.naziv = 'Konzervativni RSD'
  AND NOT EXISTS (SELECT 1 FROM client_fund_positions WHERE client_id=15 AND fund_id=f.id);

-- Marko (client_id=1) takodje u tom fondu — vise klijenata u fondu testira agregaciju.
INSERT INTO client_fund_positions (client_id, fund_id, total_invested, first_invested_at,
                                    last_modified_at, version)
SELECT 1, f.id, 150000.00,
       CURRENT_TIMESTAMP - INTERVAL '60 days', CURRENT_TIMESTAMP - INTERVAL '5 days', 0
FROM investment_funds f
WHERE f.naziv = 'Konzervativni RSD'
  AND NOT EXISTS (SELECT 1 FROM client_fund_positions WHERE client_id=1 AND fund_id=f.id);

-- Holding fonda: 10 AAPL @ avg 180 USD (to napumpa totalValue iznad likvidna_sredstva
-- jednom kad InvestmentFundService bude koristio MarketPriceClient — PR_34 kandidat).
INSERT INTO fund_holdings (fund_id, stock_ticker, quantity, avg_unit_price, deleted,
                            created_at, version)
SELECT f.id, 'AAPL', 10, 180.0000, false, CURRENT_TIMESTAMP, 0
FROM investment_funds f
WHERE f.naziv = 'Konzervativni RSD'
  AND NOT EXISTS (SELECT 1 FROM fund_holdings WHERE fund_id=f.id AND stock_ticker='AAPL');

-- Client fund transaction istorija za Mile-a — COMPLETED uplate.
INSERT INTO client_fund_transactions (client_id, fund_id, amount, is_inflow, status,
                                       occurred_at, client_account_number)
SELECT 15, f.id, 75000.00, true, 'COMPLETED',
       CURRENT_TIMESTAMP - INTERVAL '30 days', '1110001777777777'
FROM investment_funds f
WHERE f.naziv = 'Konzervativni RSD'
  AND NOT EXISTS (SELECT 1 FROM client_fund_transactions WHERE client_id=15 AND fund_id=f.id);
