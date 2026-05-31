CREATE TABLE IF NOT EXISTS loan_request_table (
                                                  id BIGSERIAL PRIMARY KEY,
                                                  version BIGINT NOT NULL DEFAULT 0,
                                                  deleted BOOLEAN NOT NULL DEFAULT FALSE,
                                                  created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),

    loan_type VARCHAR(50) NOT NULL,
    interest_type VARCHAR(50) NOT NULL,
    amount NUMERIC(19, 4) NOT NULL,
    currency VARCHAR(10) NOT NULL,
    purpose VARCHAR(255) NOT NULL,
    monthly_salary NUMERIC(19, 4) NOT NULL,
    employment_status VARCHAR(50) NOT NULL,
    current_employment_period INTEGER NOT NULL,
    repayment_period INTEGER NOT NULL,
    contact_phone VARCHAR(50) NOT NULL,
    account_number VARCHAR(50) NOT NULL,
    client_id BIGINT NOT NULL,
    status VARCHAR(50) NOT NULL,
    user_email VARCHAR(255) NOT NULL,
    username VARCHAR(255) NOT NULL
    );

CREATE TABLE IF NOT EXISTS loan_table (
                                          id BIGSERIAL PRIMARY KEY,
                                          version BIGINT NOT NULL DEFAULT 0,
                                          deleted BOOLEAN NOT NULL DEFAULT FALSE,
                                          created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),

    loan_type VARCHAR(50) NOT NULL,
    account_number VARCHAR(50) NOT NULL,
    amount NUMERIC(19, 4) NOT NULL,
    repayment_period INTEGER NOT NULL,
    nominal_interest_rate NUMERIC(19, 10) NOT NULL,
    effective_interest_rate NUMERIC(19, 10) NOT NULL,
    interest_type VARCHAR(50) NOT NULL,
    agreement_date DATE NOT NULL,
    maturity_date DATE NOT NULL,
    installment_amount NUMERIC(19, 4) NOT NULL,
    next_installment_date DATE NOT NULL,
    remaining_debt NUMERIC(19, 4) NOT NULL,
    currency VARCHAR(10) NOT NULL,
    status VARCHAR(50) NOT NULL,
    user_email VARCHAR(255) NOT NULL,
    username VARCHAR(255) NOT NULL,
    client_id BIGINT NOT NULL,
    installment_count INTEGER NOT NULL DEFAULT 0
    );

CREATE TABLE IF NOT EXISTS installment_table (
                                                 id BIGSERIAL PRIMARY KEY,
                                                 version BIGINT NOT NULL DEFAULT 0,
                                                 deleted BOOLEAN NOT NULL DEFAULT FALSE,
                                                 created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),

    loan_id BIGINT NOT NULL,
    installment_amount NUMERIC(19, 4) NOT NULL,
    interest_rate_at_payment NUMERIC(19, 10) NOT NULL,
    currency VARCHAR(10) NOT NULL,
    expected_due_date DATE NOT NULL,
    actual_due_date DATE,
    payment_status VARCHAR(50) NOT NULL,
    retry INTEGER NOT NULL DEFAULT 0,

    CONSTRAINT fk_installment_loan
    FOREIGN KEY (loan_id)
    REFERENCES loan_table(id)
    ON DELETE CASCADE
    );

CREATE INDEX IF NOT EXISTS idx_installment_loan_id
    ON installment_table(loan_id);

CREATE INDEX IF NOT EXISTS idx_loan_account_number
    ON loan_table(account_number);

CREATE INDEX IF NOT EXISTS idx_loan_request_client_id
    ON loan_request_table(client_id);