-- =========================
-- SIFRA DELATNOSTI
-- =========================
CREATE TABLE sifra_delatnosti_table (
                                        id BIGSERIAL PRIMARY KEY,
                                        version BIGINT,
                                        sifra VARCHAR(50) NOT NULL UNIQUE,
                                        grana VARCHAR(255) NOT NULL
);

CREATE TABLE sifra_delatnosti_sektori (
                                          sifra_delatnosti_id BIGINT NOT NULL,
                                          sektor VARCHAR(255) NOT NULL,
                                          CONSTRAINT fk_sifra_delatnosti_sektori
                                              FOREIGN KEY (sifra_delatnosti_id)
                                                  REFERENCES sifra_delatnosti_table(id)
);

-- =========================
-- CURRENCY
-- =========================
CREATE TABLE currency_table (
                                id BIGSERIAL PRIMARY KEY,
                                version BIGINT,
                                naziv VARCHAR(255) NOT NULL,
                                oznaka VARCHAR(20) NOT NULL UNIQUE,
                                simbol VARCHAR(5) NOT NULL UNIQUE,
                                opis VARCHAR(500) NOT NULL,
                                status VARCHAR(20) NOT NULL
);

CREATE TABLE currency_countries (
                                    currency_id BIGINT NOT NULL,
                                    country VARCHAR(255) NOT NULL,
                                    CONSTRAINT fk_currency_countries
                                        FOREIGN KEY (currency_id)
                                            REFERENCES currency_table(id)
);

-- =========================
-- COMPANY
-- =========================
CREATE TABLE company_table (
                               id BIGSERIAL PRIMARY KEY,
                               version BIGINT,
                               naziv VARCHAR(255) NOT NULL,
                               maticni_broj VARCHAR(50) NOT NULL UNIQUE,
                               poreski_broj VARCHAR(50) NOT NULL UNIQUE,
                               sifra_delatnosti_id BIGINT NOT NULL,
                               adresa VARCHAR(255),
                               vlasnik BIGINT NOT NULL,
                               CONSTRAINT fk_company_sifra_delatnosti
                                   FOREIGN KEY (sifra_delatnosti_id)
                                       REFERENCES sifra_delatnosti_table(id)
);

-- =========================
-- ACCOUNT
-- =========================
CREATE TABLE account_table (
                               id BIGSERIAL PRIMARY KEY,
                               version BIGINT,

                               account_type VARCHAR(20) NOT NULL,

                               broj_racuna VARCHAR(50) NOT NULL UNIQUE,
                               ime_vlasnika_racuna VARCHAR(255) NOT NULL,
                               prezime_vlasnika_racuna VARCHAR(255) NOT NULL,
                               email VARCHAR(50),
                               username VARCHAR(50),
                               naziv_racuna VARCHAR(255) NOT NULL,
                               vlasnik BIGINT NOT NULL,
                               zaposlen BIGINT NOT NULL,


                               stanje DECIMAL(19,2) NOT NULL,
                               raspolozivo_stanje DECIMAL(19,2) NOT NULL,

                               datum_i_vreme_kreiranja TIMESTAMP NOT NULL,
                               datum_isteka DATE,

                               currency_id BIGINT NOT NULL,
                               status VARCHAR(20) NOT NULL,

                               dnevni_limit DECIMAL(19,2),
                               mesecni_limit DECIMAL(19,2),
                               dnevna_potrosnja DECIMAL(19,2) NOT NULL,
                               mesecna_potrosnja DECIMAL(19,2) NOT NULL,

                               company_id BIGINT,

                               account_concrete VARCHAR(50),
                               odrzavanje_racuna DECIMAL(19,2),

                               account_ownership_type VARCHAR(20),

                               CONSTRAINT fk_account_currency
                                   FOREIGN KEY (currency_id)
                                       REFERENCES currency_table(id),

                               CONSTRAINT fk_account_company
                                   FOREIGN KEY (company_id)
                                       REFERENCES company_table(id)
);


-- =========================
-- TRANSACTION RECORD
-- =========================
CREATE TABLE transaction_record_table (
                                          id BIGSERIAL PRIMARY KEY,
                                          version BIGINT,

                                          account_number VARCHAR(50) NOT NULL,
                                          bank_account_number VARCHAR(50) NOT NULL,
                                          amount DECIMAL(19,2) NOT NULL,

                                          created_at TIMESTAMP NOT NULL
);

CREATE INDEX idx_transaction_record_account_number
    ON transaction_record_table(account_number);

CREATE INDEX idx_transaction_record_bank_account_number
    ON transaction_record_table(bank_account_number);

CREATE INDEX idx_transaction_record_created_at
    ON transaction_record_table(created_at);




CREATE INDEX idx_account_vlasnik ON account_table(vlasnik);
CREATE INDEX idx_account_broj ON account_table(broj_racuna);
CREATE INDEX idx_account_company ON account_table(company_id);
CREATE INDEX idx_account_ime_vlasnika ON account_table(ime_vlasnika_racuna);
CREATE INDEX idx_account_prezime_vlasnika ON account_table(prezime_vlasnika_racuna);
CREATE INDEX idx_account_ime_prezime_vlasnika
    ON account_table(ime_vlasnika_racuna, prezime_vlasnika_racuna);

-- =========================
-- TRIGGER FUNCTION
-- =========================
CREATE OR REPLACE FUNCTION trg_account_currency_check()
RETURNS TRIGGER AS $$
DECLARE
v_oznaka VARCHAR(20);
BEGIN
SELECT oznaka INTO v_oznaka
FROM currency_table
WHERE id = NEW.currency_id;

IF NEW.account_type = 'CHECKING' AND v_oznaka <> 'RSD' THEN
        RAISE EXCEPTION 'CHECKING account must use RSD currency';
END IF;

    IF NEW.account_type = 'FX' AND v_oznaka = 'RSD' THEN
        RAISE EXCEPTION 'FX account cannot use RSD currency';
END IF;

RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- =========================
-- TRIGGERS
-- =========================
CREATE TRIGGER trg_account_currency_check_insert
    BEFORE INSERT ON account_table
    FOR EACH ROW
    EXECUTE FUNCTION trg_account_currency_check();

CREATE TRIGGER trg_account_currency_check_update
    BEFORE UPDATE ON account_table
    FOR EACH ROW
    EXECUTE FUNCTION trg_account_currency_check();