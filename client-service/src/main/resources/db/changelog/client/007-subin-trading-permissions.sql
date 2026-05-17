-- liquibase formatted sql

-- changeset client-service:10 context:dev
-- comment: DEV-ONLY — upgrade Mateja Subin (client_id=9) to CLIENT_TRADING role with MARGIN_TRADE permission. Skipped in production deploy.

UPDATE clients SET role = 'CLIENT_TRADING' WHERE id = 9;

INSERT INTO client_permissions (client_id, permission) VALUES (9, 'MARGIN_TRADE')
ON CONFLICT DO NOTHING;
