-- Uklanjanje starog ograničenja koje ne prepoznaje PROCESSING stanje
ALTER TABLE notification_deliveries
DROP CONSTRAINT IF EXISTS notification_deliveries_status_check;

-- Dodavanje novog ograničenja koje obuhvata sva stanja iz novog Go State Machine-a
ALTER TABLE notification_deliveries
    ADD CONSTRAINT notification_deliveries_status_check
        CHECK (status IN ('PENDING', 'PROCESSING', 'RETRY_SCHEDULED', 'SUCCEEDED', 'FAILED'));
