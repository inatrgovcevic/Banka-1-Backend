-- Port of order-service Liquibase changeset order:12 (012-order-timestamps.sql):
-- orders gain created_at (stamped once at insert) and executed_at (set when the
-- order reaches DONE). Backfilled from last_modification so pre-existing rows
-- keep a sensible creation timestamp. Guards are idempotent for the coexistence
-- path where Java Liquibase may have already applied changeset order:12.
ALTER TABLE orders ADD COLUMN IF NOT EXISTS created_at TIMESTAMP;
ALTER TABLE orders ADD COLUMN IF NOT EXISTS executed_at TIMESTAMP;
UPDATE orders SET created_at = last_modification WHERE created_at IS NULL;
UPDATE orders SET executed_at = last_modification WHERE executed_at IS NULL AND status = 'DONE';
ALTER TABLE orders ALTER COLUMN created_at SET NOT NULL;
