-- DEV seed for Go market-service parity with trading-service-go portfolio seed.
-- Trading dev data references listing_id 1..8, so keep these IDs stable.

INSERT INTO stock_exchange (id, exchange_name, exchange_acronym, exchange_mic_code, polity, currency, time_zone, open_time, close_time, pre_market_open_time, pre_market_close_time, post_market_open_time, post_market_close_time, is_active)
VALUES
    (1, 'New York Stock Exchange', 'NYSE', 'XNYS', 'United States', 'USD', 'America/New_York', '09:30', '16:00', '04:00', '09:30', '16:00', '20:00', true),
    (2, 'NASDAQ Stock Market', 'NASDAQ', 'XNAS', 'United States', 'USD', 'America/New_York', '09:30', '16:00', '04:00', '09:30', '16:00', '20:00', true),
    (3, 'Belgrade Stock Exchange', 'BELEX', 'XBEL', 'Serbia', 'RSD', 'Europe/Belgrade', '09:00', '14:00', NULL, NULL, NULL, NULL, true),
    (4, 'Foreign Exchange Market', 'FX', 'XFXM', 'Global', 'USD', 'UTC', '00:00', '23:59', NULL, NULL, NULL, NULL, true)
ON CONFLICT (exchange_mic_code) DO NOTHING;

INSERT INTO stock (id, ticker, name, outstanding_shares, dividend_yield)
VALUES
    (1, 'AAPL', 'Apple Inc.', 15441900000, 0.0050),
    (2, 'MSFT', 'Microsoft Corporation', 7432000000, 0.0072),
    (3, 'GOOGL', 'Alphabet Inc. Class A', 5880000000, 0.0000),
    (4, 'AMZN', 'Amazon.com Inc.', 10300000000, 0.0000),
    (5, 'TSLA', 'Tesla Inc.', 3189000000, 0.0000),
    (6, 'NVDA', 'NVIDIA Corporation', 24640000000, 0.0004),
    (7, 'META', 'Meta Platforms Inc.', 2537000000, 0.0030),
    (8, 'BAC', 'Bank of America Corporation', 7860000000, 0.0220)
ON CONFLICT (ticker) DO NOTHING;

INSERT INTO listing (id, security_id, listing_type, stock_exchange_id, ticker, name, last_refresh, price, ask, bid, volume, change)
VALUES
    (1, 1, 'STOCK', 2, 'AAPL', 'Apple Inc.', now(), 293.32, 294.00, 292.50, 52000000, 1.25),
    (2, 2, 'STOCK', 2, 'MSFT', 'Microsoft Corporation', now(), 425.20, 426.00, 424.50, 18000000, -0.80),
    (3, 3, 'STOCK', 2, 'GOOGL', 'Alphabet Inc. Class A', now(), 180.50, 181.00, 179.80, 22000000, 0.35),
    (4, 4, 'STOCK', 2, 'AMZN', 'Amazon.com Inc.', now(), 185.30, 186.00, 184.70, 30000000, 0.60),
    (5, 5, 'STOCK', 2, 'TSLA', 'Tesla Inc.', now(), 240.10, 241.00, 239.50, 80000000, -2.15),
    (6, 6, 'STOCK', 2, 'NVDA', 'NVIDIA Corporation', now(), 145.80, 146.50, 145.20, 15000000, 1.10),
    (7, 7, 'STOCK', 2, 'META', 'Meta Platforms Inc.', now(), 325.40, 326.00, 324.80, 11000000, 0.45),
    (8, 8, 'STOCK', 1, 'BAC', 'Bank of America Corporation', now(), 45.25, 45.60, 45.00, 10000000, 0.15)
ON CONFLICT (id) DO NOTHING;

INSERT INTO forex_pair (id, ticker, base_currency, quote_currency, exchange_rate, liquidity)
VALUES
    (1, 'EUR/RSD', 'EUR', 'RSD', 117.20000000, 'HIGH'),
    (2, 'USD/RSD', 'USD', 'RSD', 108.50000000, 'HIGH'),
    (3, 'EUR/USD', 'EUR', 'USD', 1.08000000, 'HIGH')
ON CONFLICT (ticker) DO NOTHING;

INSERT INTO listing (id, security_id, listing_type, stock_exchange_id, ticker, name, last_refresh, price, ask, bid, volume, change)
VALUES
    (101, 1, 'FOREX', 4, 'EUR/RSD', 'EUR/RSD', now(), 117.20000000, 117.30000000, 117.10000000, 1000, 0.01000000),
    (102, 2, 'FOREX', 4, 'USD/RSD', 'USD/RSD', now(), 108.50000000, 108.60000000, 108.40000000, 1000, -0.02000000),
    (103, 3, 'FOREX', 4, 'EUR/USD', 'EUR/USD', now(), 1.08000000, 1.08100000, 1.07900000, 1000, 0.00100000)
ON CONFLICT (id) DO NOTHING;

INSERT INTO futures_contract (id, ticker, name, contract_size, contract_unit, settlement_date)
VALUES
    (1, 'WHEAT-JUN26', 'Wheat June 2026 Futures', 5000, 'bushel', '2026-06-30'),
    (2, 'OIL-SEP26', 'Crude Oil September 2026 Futures', 1000, 'barrel', '2026-09-30')
ON CONFLICT (ticker) DO NOTHING;

INSERT INTO listing (id, security_id, listing_type, stock_exchange_id, ticker, name, last_refresh, price, ask, bid, volume, change)
VALUES
    (201, 1, 'FUTURES', 1, 'WHEAT-JUN26', 'Wheat June 2026 Futures', now(), 620.00000000, 622.00000000, 618.00000000, 5000, 3.00000000),
    (202, 2, 'FUTURES', 1, 'OIL-SEP26', 'Crude Oil September 2026 Futures', now(), 78.50000000, 78.90000000, 78.10000000, 7000, -0.40000000)
ON CONFLICT (id) DO NOTHING;

SELECT setval(pg_get_serial_sequence('stock_exchange', 'id'), GREATEST((SELECT max(id) FROM stock_exchange), 1), true);
SELECT setval(pg_get_serial_sequence('stock', 'id'), GREATEST((SELECT max(id) FROM stock), 1), true);
SELECT setval(pg_get_serial_sequence('forex_pair', 'id'), GREATEST((SELECT max(id) FROM forex_pair), 1), true);
SELECT setval(pg_get_serial_sequence('futures_contract', 'id'), GREATEST((SELECT max(id) FROM futures_contract), 1), true);
SELECT setval(pg_get_serial_sequence('listing', 'id'), GREATEST((SELECT max(id) FROM listing), 1), true);
