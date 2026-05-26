-- User service schema for the Go implementation.
-- This mirrors the existing Spring/Liquibase tables used by employee-service
-- and client-service so the Go service can run against the same database.

CREATE TABLE IF NOT EXISTS employees (
    id BIGSERIAL PRIMARY KEY,
    ime VARCHAR(255) NOT NULL,
    prezime VARCHAR(255) NOT NULL,
    datum_rodjenja DATE NOT NULL,
    pol VARCHAR(10) NOT NULL,
    email VARCHAR(255) NOT NULL UNIQUE,
    broj_telefona VARCHAR(255),
    adresa VARCHAR(255),
    username VARCHAR(255) NOT NULL UNIQUE,
    password VARCHAR(255),
    pozicija VARCHAR(255) NOT NULL,
    departman VARCHAR(255) NOT NULL,
    aktivan BOOLEAN NOT NULL DEFAULT TRUE,
    role VARCHAR(50) NOT NULL,
    failed_login_attempts INTEGER NOT NULL DEFAULT 0,
    locked_until TIMESTAMP,
    version BIGINT DEFAULT 0,
    deleted BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS refresh_tokens (
    id BIGSERIAL PRIMARY KEY,
    value VARCHAR(255) NOT NULL UNIQUE,
    expiration_date_time TIMESTAMP NOT NULL,
    zaposlen_id BIGINT NOT NULL REFERENCES employees(id) ON DELETE CASCADE,
    version BIGINT DEFAULT 0,
    deleted BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS confirmation_token (
    id BIGSERIAL PRIMARY KEY,
    value VARCHAR(255) NOT NULL UNIQUE,
    expiration_date_time TIMESTAMP,
    zaposlen_id BIGINT NOT NULL UNIQUE REFERENCES employees(id) ON DELETE CASCADE,
    version BIGINT DEFAULT 0,
    deleted BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS zaposlen_permissions (
    zaposlen_id BIGINT NOT NULL REFERENCES employees(id) ON DELETE CASCADE,
    permission VARCHAR(100) NOT NULL,
    PRIMARY KEY (zaposlen_id, permission)
);

CREATE INDEX IF NOT EXISTS idx_employees_ime_prezime ON employees(ime, prezime);
CREATE INDEX IF NOT EXISTS idx_employees_pozicija ON employees(pozicija);
CREATE INDEX IF NOT EXISTS idx_employees_active ON employees(deleted, id) WHERE deleted = false;

CREATE TABLE IF NOT EXISTS clients (
    id BIGSERIAL PRIMARY KEY,
    ime VARCHAR(255) NOT NULL,
    prezime VARCHAR(255) NOT NULL,
    datum_rodjenja BIGINT NOT NULL,
    pol VARCHAR(10) NOT NULL,
    email VARCHAR(255) NOT NULL UNIQUE,
    broj_telefona VARCHAR(255),
    adresa VARCHAR(255),
    password VARCHAR(255),
    jmbg_encrypted TEXT,
    aktivan BOOLEAN NOT NULL DEFAULT FALSE,
    role VARCHAR(50) NOT NULL DEFAULT 'CLIENT_BASIC',
    version BIGINT DEFAULT 0,
    deleted BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS client_permissions (
    client_id BIGINT NOT NULL REFERENCES clients(id) ON DELETE CASCADE,
    permission VARCHAR(100) NOT NULL,
    PRIMARY KEY (client_id, permission)
);

CREATE TABLE IF NOT EXISTS client_confirmation_token (
    id BIGSERIAL PRIMARY KEY,
    value VARCHAR(255) NOT NULL UNIQUE,
    expiration_date_time TIMESTAMP,
    klijent_id BIGINT NOT NULL UNIQUE REFERENCES clients(id),
    version BIGINT DEFAULT 0,
    deleted BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_clients_ime_prezime ON clients(ime, prezime);
CREATE INDEX IF NOT EXISTS idx_clients_email ON clients(email);
CREATE INDEX IF NOT EXISTS idx_clients_jmbg_encrypted ON clients(jmbg_encrypted);
CREATE INDEX IF NOT EXISTS idx_clients_active ON clients(deleted, id) WHERE deleted = false;
