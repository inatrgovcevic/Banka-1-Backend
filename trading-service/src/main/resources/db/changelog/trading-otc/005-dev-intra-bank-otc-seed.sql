--liquibase formatted sql

-- changeset interbank:33-trading-005-dev-intra-bank-otc-seed context:dev
-- comment: PR_33 follow-up — INTRA-bank OTC seed za demo/testiranje sa frontend-a.
--          Mile Interbank (user_id=15, "C-15") postavlja se u SVE 4 uloge da bi mogao
--          da testira ceo OTC lifecycle iz UI-a (offers, contracts, exercise).
--          Plus 2 vec sklopljena ugovora (ACCEPTED + ACTIVE option contract) da
--          contracts tab ima sta da prikaze.
--
--          User mapping (out of 003-dev-public-stock-seed + Mile 004):
--            1 = Marko Markovic    (AAPL 200, MSFT 150, TSLA 80, AMZN 25)
--            2 = Ana Anic          (GOOGL 50, AMZN 30, JPM 25)
--            3 = Milica Milic      (AAPL 40, IBM 60)
--            5 = Jovana Jovanovic  (AAPL 100, MSFT 60, GOOGL 35)
--           15 = Mile Interbank    (AAPL 100, public 25)
--
--          Tickers iz market seed-a: AAPL, MSFT, GOOGL, AMZN, TSLA, IBM, GS, JPM.
--          Stock prices su "danasnje" ~realne vrednosti (~Q1 2026).
--
--          Statusi:
--            PENDING_BUYER  — ceka kupca da odgovori (counter ili accept)
--            PENDING_SELLER — ceka prodavca da odgovori
--            ACCEPTED       — sklopljen ugovor (option_contracts row prati)
--            REJECTED       — odbijena ponuda

-- ============================================================================
-- Scenario 1: Mile (15) ponudio Marko-u (1) da PRODA AAPL × 10 @ 195 USD
--             Status PENDING_BUYER — Marko (kupac) je na redu da odgovori.
--             Mile sad iz UI-a: vidi ponudu u "Aktivne ponude" tab, ali je `Status: PENDING_BUYER`
--             tj. ceka drugu stranu. Nema accept/counter dugmadi za Mile-a (on je
--             vec napravio ponudu).
-- ============================================================================
INSERT INTO otc_offers (stock_ticker, buyer_id, seller_id, amount, price_per_stock, premium,
                        settlement_date, status, modified_by, last_modified, created_at, version)
SELECT 'AAPL', 1, 15, 10, 195.00, 80.00,
       (CURRENT_DATE + INTERVAL '14 days')::date, 'PENDING_BUYER', 'SELLER:15',
       CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 0
WHERE NOT EXISTS (SELECT 1 FROM otc_offers
                  WHERE stock_ticker='AAPL' AND seller_id=15 AND buyer_id=1
                    AND price_per_stock=195.00 AND amount=10);

-- ============================================================================
-- Scenario 2: Marko (1) ponudio Mile-u (15) da KUPI AAPL × 5 @ 198 USD
--             Status PENDING_SELLER — Marko (prodavac) je na redu (counter-offer
--             u toku). Mile sad ne treba da odgovara.
--             Wait — ovo je gresno semanticki: ako Marko prodaje, on je seller.
--             Status PENDING_SELLER znaci da seller treba da odgovori (counter).
--             Znaci Marko sebi proba prodaju Mile-u, sad on (seller) na redu da
--             potvrdi/counter-uje sa svoje strane (npr. posle Mile counter-offer-a).
--             Mile vidi: PENDING_SELLER, ne moze da accept/counter dok seller ne odgovori.
-- ============================================================================
INSERT INTO otc_offers (stock_ticker, buyer_id, seller_id, amount, price_per_stock, premium,
                        settlement_date, status, modified_by, last_modified, created_at, version)
SELECT 'AAPL', 15, 1, 5, 198.00, 50.00,
       (CURRENT_DATE + INTERVAL '21 days')::date, 'PENDING_SELLER', 'BUYER:15',
       CURRENT_TIMESTAMP - INTERVAL '2 hours', CURRENT_TIMESTAMP - INTERVAL '1 day', 0
WHERE NOT EXISTS (SELECT 1 FROM otc_offers
                  WHERE stock_ticker='AAPL' AND seller_id=1 AND buyer_id=15
                    AND price_per_stock=198.00 AND amount=5);

-- ============================================================================
-- Scenario 3: Ana (2) ponudila Mile-u (15) da KUPI GOOGL × 5 @ 145 USD
--             Status PENDING_SELLER — Mile (seller) na redu da odgovori.
--             Ovo je glavni demo scenario — Mile moze klik Prihvati ili Protivponuda
--             ili Odustani u UI-u.
--             Mile NEMA GOOGL portfolio! Posto je intra-bank, ako prihvati,
--             trading-service ce traziti GOOGL u Mile portfoliu i pasti.
--             Resenje: kupac je 15 (Mile), seller je 2 (Ana). Ana ima GOOGL 50.
--             Drugacije: Ana prodaje Mile-u, Ana je seller. Mile je kupac. Mile ne mora
--             da ima GOOGL — kupuje opciju da kupi GOOGL od Ane na settlement date.
--             OTC opcija = pravo, ne obaveza, da kupi/proda. Premium se placa
--             odmah (Mile placa Ani premium 35 USD da rezervise pravo).
--
--             Frontend buyer_id=15 → "PENDING_SELLER" znaci seller (Ana) na redu.
--             Hmm sad cekam koji status implicitno znaci "Mile na redu".
--             Per OtcServiceImpl: offer kreira buyer ili seller. modified_by polje
--             pokazuje ko je poslednji menjao. status flip-uje koji je sledeci na redu.
--             Da Mile bude na redu kao seller, status=PENDING_SELLER, buyer_id=2,
--             seller_id=15.
-- ============================================================================
INSERT INTO otc_offers (stock_ticker, buyer_id, seller_id, amount, price_per_stock, premium,
                        settlement_date, status, modified_by, last_modified, created_at, version)
SELECT 'AAPL', 2, 15, 8, 192.50, 60.00,
       (CURRENT_DATE + INTERVAL '10 days')::date, 'PENDING_SELLER', 'BUYER:2',
       CURRENT_TIMESTAMP - INTERVAL '4 hours', CURRENT_TIMESTAMP - INTERVAL '4 hours', 0
WHERE NOT EXISTS (SELECT 1 FROM otc_offers
                  WHERE stock_ticker='AAPL' AND seller_id=15 AND buyer_id=2
                    AND price_per_stock=192.50 AND amount=8);

-- ============================================================================
-- Scenario 4: Mile (15) trazi da KUPI MSFT × 3 od Jovana (5)
--             Status PENDING_BUYER — Mile (buyer) na redu.
--             Jovana je counter-ovala, sad Mile bira: accept ili counter ili odustani.
-- ============================================================================
INSERT INTO otc_offers (stock_ticker, buyer_id, seller_id, amount, price_per_stock, premium,
                        settlement_date, status, modified_by, last_modified, created_at, version)
SELECT 'MSFT', 15, 5, 3, 420.00, 120.00,
       (CURRENT_DATE + INTERVAL '30 days')::date, 'PENDING_BUYER', 'SELLER:5',
       CURRENT_TIMESTAMP - INTERVAL '6 hours', CURRENT_TIMESTAMP - INTERVAL '2 days', 0
WHERE NOT EXISTS (SELECT 1 FROM otc_offers
                  WHERE stock_ticker='MSFT' AND seller_id=5 AND buyer_id=15
                    AND price_per_stock=420.00 AND amount=3);

-- ============================================================================
-- Scenario 5: ACCEPTED + ACTIVE option contract — Mile (15) je kupio AAPL
--             opciju od Marko-a (1), premium 100 USD placen, contract aktivan.
--             U UI Contracts tab-u Mile-a treba da vidi ovaj ugovor sa dugmetom
--             "Iskoristi (Exercise)".
-- ============================================================================
INSERT INTO otc_offers (id, stock_ticker, buyer_id, seller_id, amount, price_per_stock, premium,
                        settlement_date, status, modified_by, last_modified, created_at, version)
SELECT 9001, 'AAPL', 15, 1, 5, 190.00, 100.00,
       (CURRENT_DATE + INTERVAL '60 days')::date, 'ACCEPTED', 'SELLER:1',
       CURRENT_TIMESTAMP - INTERVAL '1 day', CURRENT_TIMESTAMP - INTERVAL '3 days', 0
WHERE NOT EXISTS (SELECT 1 FROM otc_offers WHERE id=9001);

-- Bump sequence iznad 9001 (ako naknadne ponude koriste serial, da ne kolizija).
SELECT setval(pg_get_serial_sequence('otc_offers','id'),
              GREATEST((SELECT MAX(id) FROM otc_offers), 9100));

INSERT INTO option_contracts (offer_id, stock_ticker, buyer_id, seller_id, amount,
                              price_per_stock, settlement_date, status,
                              created_at, version)
SELECT 9001, 'AAPL', 15, 1, 5, 190.00,
       (CURRENT_DATE + INTERVAL '60 days')::date, 'ACTIVE',
       CURRENT_TIMESTAMP - INTERVAL '1 day', 0
WHERE NOT EXISTS (SELECT 1 FROM option_contracts WHERE offer_id=9001);

-- ============================================================================
-- Scenario 6: REJECTED ponuda u istoriji — Mile (15) je odbio MSFT ponudu od Ane (2)
--             Pokriva "Istorija" tab/sortiranje po datumu.
-- ============================================================================
INSERT INTO otc_offers (stock_ticker, buyer_id, seller_id, amount, price_per_stock, premium,
                        settlement_date, status, modified_by, last_modified, created_at, version)
SELECT 'MSFT', 2, 15, 2, 425.00, 50.00,
       (CURRENT_DATE + INTERVAL '7 days')::date, 'REJECTED', 'SELLER:15',
       CURRENT_TIMESTAMP - INTERVAL '12 hours', CURRENT_TIMESTAMP - INTERVAL '2 days', 0
WHERE NOT EXISTS (SELECT 1 FROM otc_offers
                  WHERE stock_ticker='MSFT' AND seller_id=15 AND buyer_id=2 AND status='REJECTED');

-- ============================================================================
-- Bonus: 2 ponude izmedju drugih klijenata (NE Mile) — ekran "Aktivne ponude"
-- nije prazan kad se loguje neko drugi (npr. Marko = id 1) sa testne strane.
-- ============================================================================
INSERT INTO otc_offers (stock_ticker, buyer_id, seller_id, amount, price_per_stock, premium,
                        settlement_date, status, modified_by, last_modified, created_at, version)
SELECT 'TSLA', 3, 1, 5, 250.00, 75.00,
       (CURRENT_DATE + INTERVAL '15 days')::date, 'PENDING_BUYER', 'SELLER:1',
       CURRENT_TIMESTAMP, CURRENT_TIMESTAMP - INTERVAL '4 hours', 0
WHERE NOT EXISTS (SELECT 1 FROM otc_offers
                  WHERE stock_ticker='TSLA' AND seller_id=1 AND buyer_id=3
                    AND price_per_stock=250.00 AND amount=5);

INSERT INTO otc_offers (stock_ticker, buyer_id, seller_id, amount, price_per_stock, premium,
                        settlement_date, status, modified_by, last_modified, created_at, version)
SELECT 'GOOGL', 5, 2, 4, 148.00, 40.00,
       (CURRENT_DATE + INTERVAL '20 days')::date, 'PENDING_SELLER', 'BUYER:5',
       CURRENT_TIMESTAMP, CURRENT_TIMESTAMP - INTERVAL '8 hours', 0
WHERE NOT EXISTS (SELECT 1 FROM otc_offers
                  WHERE stock_ticker='GOOGL' AND seller_id=2 AND buyer_id=5
                    AND price_per_stock=148.00 AND amount=4);
