--liquibase formatted sql

-- changeset jovan:11 splitStatements:false
-- PR_11 C11.16 + PR_15: dodaje daily_limit_remaining kolonu u account_table.
-- Stari audit referencirao 'accounts' tabelu sto je netacno (prava tabela je account_table).

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'account_table') THEN
        ALTER TABLE account_table ADD COLUMN IF NOT EXISTS daily_limit_remaining NUMERIC(19,2);
        IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'account_table' AND column_name = 'dnevni_limit') THEN
            UPDATE account_table
               SET daily_limit_remaining = COALESCE(dnevni_limit, 0)
             WHERE daily_limit_remaining IS NULL;
        END IF;
    END IF;
END $$;

-- rollback ALTER TABLE account_table DROP COLUMN IF EXISTS daily_limit_remaining;
