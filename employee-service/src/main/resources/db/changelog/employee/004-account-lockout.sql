-- liquibase formatted sql

-- changeset banka1:004-account-lockout-1
-- Celina 1, Scenario 5: per-account zakljucavanje naloga posle vise neuspesnih pokusaja prijave.
ALTER TABLE employees
    ADD COLUMN failed_login_attempts INTEGER NOT NULL DEFAULT 0;

-- changeset banka1:004-account-lockout-2
ALTER TABLE employees
    ADD COLUMN locked_until TIMESTAMP;
