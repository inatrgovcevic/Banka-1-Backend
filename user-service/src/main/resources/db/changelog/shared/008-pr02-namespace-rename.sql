--liquibase formatted sql

-- changeset jovan:8 context:always
-- PR_02 C2.6 — naknadna migracija za konsolidovani user-service.
--
-- Nista nije potrebno menjati u DDL-u jer su employee i client tabele uvek
-- imale razlicita imena (employees vs clients, zaposlen_permissions vs
-- klijent_permissions, itd.). Spajanje paketa u Java sloju ne diraju DB šemu.
--
-- Ovaj changeset postoji samo radi marker-a u DATABASECHANGELOG tabeli — ako
-- migracija reaguje na PR_02 deploy, lako je proveriti da je servis startovan
-- u konsolidovanom modu:
--
--   SELECT id, author, dateexecuted, comments
--   FROM databasechangelog
--   WHERE id = '8' AND author = 'jovan';

INSERT INTO databasechangeloglock (id, locked) VALUES (1, FALSE)
    ON CONFLICT (id) DO NOTHING;

-- rollback: -- nothing; ovaj changeset je no-op pa nema rollback.
