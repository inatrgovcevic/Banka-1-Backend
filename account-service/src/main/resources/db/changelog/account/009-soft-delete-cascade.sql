--liquibase formatted sql

-- changeset jovan:9 splitStatements:false
-- PR_07 C7.2 + PR_15 C15.x: kaskadni soft-delete audit kolone.
--
-- Tabele 'accounts'/'cards' iz starog audit-a ne postoje. Prave tabele:
--  - account_table (account-service legacy schema)
--  - cards         (card-service legacy schema, postoji samo u banking-core merge-u)
-- DO block sa information_schema check-om je idempotentan i radi i kada je samo
-- account-service deploy (cards tabela ne postoji), i kada je banking-core merge.

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'account_table') THEN
        ALTER TABLE account_table ADD COLUMN IF NOT EXISTS deleted_due_to_client_id BIGINT;
        CREATE INDEX IF NOT EXISTS idx_account_table_deleted_due_to_client_id ON account_table(deleted_due_to_client_id);
    END IF;
    IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'cards') THEN
        ALTER TABLE cards ADD COLUMN IF NOT EXISTS deleted_due_to_client_id BIGINT;
        CREATE INDEX IF NOT EXISTS idx_cards_deleted_due_to_client_id ON cards(deleted_due_to_client_id);
    END IF;
END $$;

-- rollback ALTER TABLE account_table DROP COLUMN IF EXISTS deleted_due_to_client_id;
-- rollback ALTER TABLE cards         DROP COLUMN IF EXISTS deleted_due_to_client_id;
