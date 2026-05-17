-- liquibase formatted sql
-- changeset review:1
CREATE INDEX idx_employees_email ON employees(email);
CREATE INDEX idx_employees_username ON employees(username);
