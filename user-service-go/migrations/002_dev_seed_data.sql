-- Dev seed data matching the old Java/Liquibase local dataset closely enough for frontend smoke tests.

INSERT INTO employees (ime, prezime, datum_rodjenja, pol, email, username, password, pozicija, departman, aktivan, role, version, deleted)
VALUES
    ('Admin', 'Adminovic', '1990-01-01', 'M', 'admin@banka.com', 'admin', '$argon2id$v=19$m=65536,t=3,p=1$cml4YnF1MGJOaG5md1cxOQ$kTOwNnDZmFymtQgsCUgpYFUJC9eV8wmpBCEldnS3XeE', 'Direktor', 'Uprava', true, 'ADMIN', 0, false),
    ('Petar', 'Petrovic', '1985-05-12', 'M', 'petar.petrovic@banka.com', 'petar', '$argon2id$v=19$m=65536,t=3,p=1$cml4YnF1MGJOaG5md1cxOQ$kTOwNnDZmFymtQgsCUgpYFUJC9eV8wmpBCEldnS3XeE', 'Sef brokerskog tima', 'Hartije od vrednosti', true, 'SUPERVISOR', 0, false),
    ('Milica', 'Milic', '1988-02-14', 'Z', 'milica.milic@banka.com', 'milica', '$argon2id$v=19$m=65536,t=3,p=1$cml4YnF1MGJOaG5md1cxOQ$kTOwNnDZmFymtQgsCUgpYFUJC9eV8wmpBCEldnS3XeE', 'Sef ekspoziture', 'Prodaja', true, 'SUPERVISOR', 0, false),
    ('Ana', 'Anic', '1995-11-05', 'Z', 'ana.anic@banka.com', 'ana', '$argon2id$v=19$m=65536,t=3,p=1$cml4YnF1MGJOaG5md1cxOQ$kTOwNnDZmFymtQgsCUgpYFUJC9eV8wmpBCEldnS3XeE', 'Broker', 'Hartije od vrednosti', true, 'AGENT', 0, false),
    ('Jovan', 'Jovanovic', '1992-08-23', 'M', 'jovan.jovanovic@banka.com', 'jovan', '$argon2id$v=19$m=65536,t=3,p=1$cml4YnF1MGJOaG5md1cxOQ$kTOwNnDZmFymtQgsCUgpYFUJC9eV8wmpBCEldnS3XeE', 'Diler hartija od vrednosti', 'Hartije od vrednosti', true, 'AGENT', 0, false),
    ('Marko', 'Markovic', '1991-07-30', 'M', 'marko.markovic@banka.com', 'marko', '$argon2id$v=19$m=65536,t=3,p=1$cml4YnF1MGJOaG5md1cxOQ$kTOwNnDZmFymtQgsCUgpYFUJC9eV8wmpBCEldnS3XeE', 'IT Administrator', 'IT', true, 'BASIC', 0, false),
    ('Nikola', 'Nikolic', '1993-09-15', 'M', 'nikola.nikolic@banka.com', 'nikola', '$argon2id$v=19$m=65536,t=3,p=1$cml4YnF1MGJOaG5md1cxOQ$kTOwNnDZmFymtQgsCUgpYFUJC9eV8wmpBCEldnS3XeE', 'Salterski radnik', 'Prodaja', true, 'BASIC', 0, false),
    ('Jelena', 'Jelic', '1987-12-01', 'Z', 'jelena.jelic@banka.com', 'jelena', '$argon2id$v=19$m=65536,t=3,p=1$cml4YnF1MGJOaG5md1cxOQ$kTOwNnDZmFymtQgsCUgpYFUJC9eV8wmpBCEldnS3XeE', 'Kreditni sluzbenik', 'Finansije', true, 'BASIC', 0, false)
ON CONFLICT (email) DO UPDATE SET
    password = EXCLUDED.password,
    aktivan = EXCLUDED.aktivan,
    deleted = EXCLUDED.deleted,
    updated_at = now();

INSERT INTO zaposlen_permissions (zaposlen_id, permission)
SELECT e.id, p.permission
FROM employees e
JOIN (
    VALUES
        ('ADMIN', 'BANKING_BASIC'), ('ADMIN', 'CLIENT_MANAGE'), ('ADMIN', 'SECURITIES_TRADE_LIMITED'),
        ('ADMIN', 'SECURITIES_TRADE_UNLIMITED'), ('ADMIN', 'TRADE_UNLIMITED'), ('ADMIN', 'OTC_TRADE'),
        ('ADMIN', 'FUND_AGENT_MANAGE'), ('ADMIN', 'EMPLOYEE_MANAGE_ALL'),
        ('SUPERVISOR', 'BANKING_BASIC'), ('SUPERVISOR', 'CLIENT_MANAGE'), ('SUPERVISOR', 'SECURITIES_TRADE_LIMITED'),
        ('SUPERVISOR', 'SECURITIES_TRADE_UNLIMITED'), ('SUPERVISOR', 'TRADE_UNLIMITED'), ('SUPERVISOR', 'OTC_TRADE'),
        ('SUPERVISOR', 'FUND_AGENT_MANAGE'),
        ('AGENT', 'BANKING_BASIC'), ('AGENT', 'CLIENT_MANAGE'), ('AGENT', 'SECURITIES_TRADE_LIMITED'),
        ('BASIC', 'BANKING_BASIC'), ('BASIC', 'CLIENT_MANAGE')
) AS p(role, permission) ON p.role = e.role
ON CONFLICT DO NOTHING;

INSERT INTO clients (ime, prezime, datum_rodjenja, pol, email, broj_telefona, adresa, password, role, aktivan, version, deleted)
VALUES
    ('Marko', 'Markovic', 694310400000, 'M', 'marko.markovic@banka.com', '+381641234567', 'Ulica 1, Beograd', '$argon2id$v=19$m=65536,t=3,p=1$cml4YnF1MGJOaG5md1cxOQ$kTOwNnDZmFymtQgsCUgpYFUJC9eV8wmpBCEldnS3XeE', 'CLIENT_BASIC', true, 0, false),
    ('Ana', 'Anic', 757382400000, 'Z', 'ana.anic@banka.com', '+381652345678', 'Ulica 2, Novi Sad', '$argon2id$v=19$m=65536,t=3,p=1$cml4YnF1MGJOaG5md1cxOQ$kTOwNnDZmFymtQgsCUgpYFUJC9eV8wmpBCEldnS3XeE', 'CLIENT_BASIC', true, 0, false),
    ('Jovana', 'Jovanovic', 883612800000, 'Z', 'jovana.jovanovic@banka.com', '+381663456789', 'Ulica 3, Nis', '$argon2id$v=19$m=65536,t=3,p=1$cml4YnF1MGJOaG5md1cxOQ$kTOwNnDZmFymtQgsCUgpYFUJC9eV8wmpBCEldnS3XeE', 'CLIENT_BASIC', true, 0, false),
    ('Stefan', 'Stefanovic', 946684800000, 'M', 'stefan.stefanovic@banka.com', '+381674567890', 'Ulica 4, Kragujevac', '$argon2id$v=19$m=65536,t=3,p=1$cml4YnF1MGJOaG5md1cxOQ$kTOwNnDZmFymtQgsCUgpYFUJC9eV8wmpBCEldnS3XeE', 'CLIENT_BASIC', true, 0, false),
    ('Milica', 'Milic', 631152000000, 'Z', 'milica.milic@banka.com', '+381685678901', 'Ulica 5, Subotica', '$argon2id$v=19$m=65536,t=3,p=1$cml4YnF1MGJOaG5md1cxOQ$kTOwNnDZmFymtQgsCUgpYFUJC9eV8wmpBCEldnS3XeE', 'CLIENT_BASIC', true, 0, false),
    ('Nikola', 'Nikolic', 724204800000, 'M', 'nikola.nikolic@banka.com', '+381696789012', 'Ulica 6, Novi Sad', '$argon2id$v=19$m=65536,t=3,p=1$cml4YnF1MGJOaG5md1cxOQ$kTOwNnDZmFymtQgsCUgpYFUJC9eV8wmpBCEldnS3XeE', 'CLIENT_BASIC', true, 0, false),
    ('Jelena', 'Jelic', 565056000000, 'Z', 'jelena.jelic@banka.com', '+381607890123', 'Ulica 7, Beograd', '$argon2id$v=19$m=65536,t=3,p=1$cml4YnF1MGJOaG5md1cxOQ$kTOwNnDZmFymtQgsCUgpYFUJC9eV8wmpBCEldnS3XeE', 'CLIENT_BASIC', true, 0, false),
    ('Aleksandar', 'Aleksic', 662688000000, 'M', 'aleksandar.aleksic@banka.com', '+381618901234', 'Ulica 8, Nis', '$argon2id$v=19$m=65536,t=3,p=1$cml4YnF1MGJOaG5md1cxOQ$kTOwNnDZmFymtQgsCUgpYFUJC9eV8wmpBCEldnS3XeE', 'CLIENT_BASIC', true, 0, false)
ON CONFLICT (email) DO UPDATE SET
    password = EXCLUDED.password,
    aktivan = EXCLUDED.aktivan,
    deleted = EXCLUDED.deleted,
    updated_at = now();

INSERT INTO client_permissions (client_id, permission)
SELECT c.id, 'CLIENT_BASIC'
FROM clients c
WHERE c.role = 'CLIENT_BASIC'
ON CONFLICT DO NOTHING;
