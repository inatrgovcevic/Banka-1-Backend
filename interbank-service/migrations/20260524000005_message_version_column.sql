-- +goose Up
-- +goose StatementBegin
-- Tim 2 IMPORTANT-5: dodaj version kolonu za JPA optimistic locking.
-- Bez @Version, dve scheduler instance mogu istovremeno povuci isti
-- PENDING_SEND red i poslati duplikat outbound poruke partneru.
ALTER TABLE interbank_messages
    ADD COLUMN IF NOT EXISTS version BIGINT NOT NULL DEFAULT 0;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE interbank_messages DROP COLUMN IF EXISTS version;
-- +goose StatementEnd
