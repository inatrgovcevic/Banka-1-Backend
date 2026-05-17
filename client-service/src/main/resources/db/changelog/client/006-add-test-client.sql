-- liquibase formatted sql

-- changeset client-service:9 context:dev
-- DEV-ONLY seed: Mateja Subin trading-enabled test client. Password 'admin123' (Argon2 hash below).
-- Used by Cypress E2E specs. NEVER seed in production.
INSERT INTO clients (ime, prezime, datum_rodjenja, pol, email, broj_telefona, adresa, jmbg, password, role, version, deleted, aktivan)
VALUES ('Mateja', 'Subin', 643680000000, 'M', 'subin.mateja@gmail.com', NULL, NULL, '1505990710099', '$argon2id$v=19$m=65536,t=3,p=1$cml4YnF1MGJOaG5md1cxOQ$kTOwNnDZmFymtQgsCUgpYFUJC9eV8wmpBCEldnS3XeE', 'CLIENT_BASIC', 0, false, true);
