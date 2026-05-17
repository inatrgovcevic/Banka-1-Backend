--liquibase formatted sql

-- changeset jovan:saga-2
-- PR_11 C11.1: dodaje correlation_id, total_steps, version kolone na saga_instance.
-- Postojeca tabela (kreirana u changeset 001) prosiruje se backward-compatible.

ALTER TABLE saga_instance ADD COLUMN IF NOT EXISTS correlation_id VARCHAR(64);
ALTER TABLE saga_instance ADD COLUMN IF NOT EXISTS total_steps    INTEGER NOT NULL DEFAULT 1;
ALTER TABLE saga_instance ADD COLUMN IF NOT EXISTS version        BIGINT  NOT NULL DEFAULT 0;

-- Backfill correlation_id iz payload->>'correlationId' za postojece redove (Postgres JSONB ->> operator).
UPDATE saga_instance
   SET correlation_id = COALESCE(payload->>'correlationId', payload->>'transactionId',
                                  payload->>'contractId', payload->>'transferId',
                                  id::text)
 WHERE correlation_id IS NULL;

-- Posle backfill-a, ucini kolonu NOT NULL.
ALTER TABLE saga_instance ALTER COLUMN correlation_id SET NOT NULL;

-- Unique constraint — sprecava duplikate posle redelivery-a.
ALTER TABLE saga_instance
    ADD CONSTRAINT uk_saga_type_correlation UNIQUE (saga_type, correlation_id);

CREATE INDEX IF NOT EXISTS idx_saga_instance_correlation ON saga_instance(correlation_id);
CREATE INDEX IF NOT EXISTS idx_saga_instance_state       ON saga_instance(state);

-- rollback ALTER TABLE saga_instance DROP CONSTRAINT IF EXISTS uk_saga_type_correlation;
-- rollback DROP INDEX IF EXISTS idx_saga_instance_correlation;
-- rollback DROP INDEX IF EXISTS idx_saga_instance_state;
-- rollback ALTER TABLE saga_instance DROP COLUMN IF EXISTS version;
-- rollback ALTER TABLE saga_instance DROP COLUMN IF EXISTS total_steps;
-- rollback ALTER TABLE saga_instance DROP COLUMN IF EXISTS correlation_id;
