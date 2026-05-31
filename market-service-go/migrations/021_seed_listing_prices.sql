-- DEV-ONLY seed: realisticne cene za listing-e koje pravi StockTickerSeedService.
-- Pri svezem reseed-u DB-a, scheduler za fetch real-time podataka (Twelve Data API)
-- ne radi bez API key-a, pa svi orderi padaju 400 jer je amount=0 (price*qty).
-- Ovaj UPDATE garantuje da uvek postoje cene za AAPL/MSFT/GOOGL/AMZN/TSLA tako da
-- BUY/SELL flow radi i u potpuno svezem dev okruzenju.

UPDATE listing SET price=293.32, ask=294.00, bid=292.50, volume=52000000 WHERE ticker='AAPL';
UPDATE listing SET price=425.20, ask=426.00, bid=424.50, volume=18000000 WHERE ticker='MSFT';
UPDATE listing SET price=180.50, ask=181.00, bid=179.80, volume=22000000 WHERE ticker='GOOGL';
UPDATE listing SET price=185.30, ask=186.00, bid=184.70, volume=30000000 WHERE ticker='AMZN';
UPDATE listing SET price=240.10, ask=241.00, bid=239.50, volume=80000000 WHERE ticker='TSLA';
UPDATE listing SET price=145.80, ask=146.50, bid=145.20, volume=15000000 WHERE ticker='NVDA';
UPDATE listing SET price=325.40, ask=326.00, bid=324.80, volume=11000000 WHERE ticker='META';
