--liquibase formatted sql

-- changeset jovan:5
-- PR_06 C6.5: partial unique index na verification_sessions tabeli kako bi se
-- osiguralo da po jednom resource_id-u (transferId/cardRequestId/itd.) postoji
-- najvise JEDNA aktivna sesija u stanju PENDING.
--
-- Pre PR_06: race-condition u OtpService.requestOtp() omogucava 2 paralelna POST-a
-- da kreiraju 2 redundant PENDING sesije za isti transferId. Klijent dobija 2
-- SMS-a, oba validna, sto znaci da napadac koji ukrade SMS od jedne sesije moze
-- da konfirmise drugi, vec mrtvu transakciju.
--
-- Posle PR_06: DB-level uniqueness garantuje da samo jedna PENDING sesija po
-- (resource_id, resource_type) paru postoji. Service prvo SELECT-uje postojecu
-- pre kreiranja novog (race-free zbog read-uncommitted koji DB-prevenira).

-- PR_15: prave kolone su operation_type / related_entity_id (vidi 001 createTable),
-- ne resource_id/resource_type. Drugi index sa istim semantickim ciljem (samo jedna PENDING
-- sesija po client+op+resource) vec postoji u 002 changeset-u
-- (uk_verification_sessions_pending_unique). Ovde dodajemo IF NOT EXISTS noop guard
-- da se changeset zabelezi u DATABASECHANGELOG bez konflikta.
CREATE UNIQUE INDEX IF NOT EXISTS uk_verification_sessions_pending
    ON verification_sessions (operation_type, related_entity_id)
    WHERE status = 'PENDING';

-- rollback DROP INDEX IF EXISTS uk_verification_sessions_pending;


-- changeset jovan:6
-- PR_06 C6.5: idempotency log za saga listeners.
-- Cuva (event_id, processed_at) tako da listener mogu da preskoce vec procesirane
-- event-e (sprecava double-execute u slucaju RabbitMQ redelivery-a).

CREATE TABLE IF NOT EXISTS saga_idempotency_log (
    event_id     VARCHAR(64)  NOT NULL,
    listener     VARCHAR(64)  NOT NULL,
    processed_at TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (event_id, listener)
);

CREATE INDEX idx_saga_idem_processed_at ON saga_idempotency_log(processed_at);
-- TTL cleanup: 14 dana retencija (cron task u saga-orchestrator).

-- rollback DROP TABLE IF EXISTS saga_idempotency_log;
