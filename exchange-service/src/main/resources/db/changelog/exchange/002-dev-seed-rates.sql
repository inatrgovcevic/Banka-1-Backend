-- DEV/PROD seed za exchange rates (RSD baza). Realne aproksimacije po stanju 2026-05.
-- Ovaj fajl se izvrsava bez konteksta da bi sistem radio i kada Twelve Data API key nije postavljen
-- (production deploy moze override-ovati kroz scheduler nakon prvog uspesnog fetch-a).

INSERT INTO exchange_rate (currency_code, buying_rate, selling_rate, rate_date)
SELECT * FROM (VALUES
    ('EUR', 116.50, 117.50, CURRENT_DATE),
    ('USD', 108.20, 109.20, CURRENT_DATE),
    ('CHF', 124.80, 125.80, CURRENT_DATE),
    ('GBP', 137.50, 138.70, CURRENT_DATE),
    ('JPY', 0.72, 0.74, CURRENT_DATE),
    ('CAD', 79.40, 80.30, CURRENT_DATE),
    ('AUD', 70.10, 71.00, CURRENT_DATE)
) AS rates(code, buy, sell, dt)
WHERE NOT EXISTS (
    SELECT 1 FROM exchange_rate WHERE currency_code = rates.code AND rate_date = rates.dt
);
