package store

import (
	"context"
	"os"
	"testing"
	"time"

	"Banka1Back/credit-service-go/internal/model"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testDB *pgxpool.Pool

func TestMain(m *testing.M) {
	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		os.Exit(m.Run())
	}

	pool, err := pgxpool.New(context.Background(), dbURL)
	if err != nil {
		panic("failed to connect to test database: " + err.Error())
	}
	defer pool.Close()

	if err = pool.Ping(context.Background()); err != nil {
		panic("failed to ping test database: " + err.Error())
	}

	testDB = pool

	setupSchema(pool)

	code := m.Run()

	cleanupTables(pool)

	os.Exit(code)
}

func setupSchema(pool *pgxpool.Pool) {
	ctx := context.Background()

	_, _ = pool.Exec(ctx, `
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
		)
	`)

	_, _ = pool.Exec(ctx, `
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
		)
	`)

	_, _ = pool.Exec(ctx, `
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
			CONSTRAINT fk_installment_loan FOREIGN KEY (loan_id) REFERENCES loan_table(id) ON DELETE CASCADE
		)
	`)
}

func cleanupTables(pool *pgxpool.Pool) {
	ctx := context.Background()
	_, _ = pool.Exec(ctx, `TRUNCATE TABLE installment_table, loan_table, loan_request_table RESTART IDENTITY CASCADE`)
}

func skipIfNoDB(t *testing.T) {
	t.Helper()
	if testDB == nil {
		t.Skip("TEST_DATABASE_URL not set; skipping integration test")
	}
}

func sampleLoanRequest() model.LoanRequest {
	return model.LoanRequest{
		LoanType:                model.LoanGotovinski,
		InterestType:            model.InterestFixed,
		Amount:                  decimal.NewFromInt(50000),
		Currency:                model.CurrencyRSD,
		Purpose:                 "house renovation",
		MonthlySalary:           decimal.NewFromInt(1500),
		EmploymentStatus:        model.EmploymentPermanent,
		CurrentEmploymentPeriod: 24,
		RepaymentPeriod:         12,
		ContactPhone:            "0601234567",
		AccountNumber:           "1234567890123456789",
		ClientID:                1001,
		Status:                  model.StatusPending,
		UserEmail:               "user@bank.io",
		Username:                "testuser",
	}
}

func sampleLoan() model.Loan {
	now := time.Now()
	return model.Loan{
		LoanType:              model.LoanGotovinski,
		AccountNumber:         "1234567890123456789",
		Amount:                decimal.NewFromInt(50000),
		RepaymentPeriod:       12,
		NominalInterestRate:   decimal.NewFromFloat(0.01),
		EffectiveInterestRate: decimal.NewFromFloat(0.01),
		InterestType:          model.InterestFixed,
		AgreementDate:         time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()),
		MaturityDate:          time.Date(now.Year(), now.Month()+12, now.Day(), 0, 0, 0, 0, now.Location()),
		InstallmentAmount:     decimal.NewFromFloat(4400.00),
		NextInstallmentDate:   time.Date(now.Year(), now.Month()+1, now.Day(), 0, 0, 0, 0, now.Location()),
		RemainingDebt:         decimal.NewFromInt(50000),
		Currency:              model.CurrencyRSD,
		Status:                model.StatusActive,
		UserEmail:             "user@bank.io",
		Username:              "testuser",
		ClientID:              1001,
		InstallmentCount:      0,
	}
}

// ---------------------------------------------------------------------------
// LoanRequestStore tests
// ---------------------------------------------------------------------------

func TestLoanRequestStore_Save_Success(t *testing.T) {
	skipIfNoDB(t)
	cleanupTables(testDB)

	store := NewLoanRequestStore(testDB)
	req := sampleLoanRequest()

	saved, err := store.Save(context.Background(), req)
	require.NoError(t, err)
	assert.Greater(t, saved.ID, int64(0))
	assert.Equal(t, model.StatusPending, saved.Status)
	assert.False(t, saved.CreatedAt.IsZero())
}

func TestLoanRequestStore_FindAll_ReturnsResults(t *testing.T) {
	skipIfNoDB(t)
	cleanupTables(testDB)

	store := NewLoanRequestStore(testDB)
	_, err := store.Save(context.Background(), sampleLoanRequest())
	require.NoError(t, err)

	results, total, err := store.FindAll(context.Background(), nil, nil, 0, 10)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(results), 1)
	assert.GreaterOrEqual(t, total, 1)
}

func TestLoanRequestStore_FindAll_WithLoanTypeFilter(t *testing.T) {
	skipIfNoDB(t)
	cleanupTables(testDB)

	store := NewLoanRequestStore(testDB)
	_, err := store.Save(context.Background(), sampleLoanRequest())
	require.NoError(t, err)

	lt := model.LoanGotovinski
	results, total, err := store.FindAll(context.Background(), &lt, nil, 0, 10)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(results), 1)
	assert.GreaterOrEqual(t, total, 1)
}

func TestLoanRequestStore_FindAll_WithAccountNumberFilter(t *testing.T) {
	skipIfNoDB(t)
	cleanupTables(testDB)

	store := NewLoanRequestStore(testDB)
	req := sampleLoanRequest()
	req.AccountNumber = "9999999999999999999"
	_, err := store.Save(context.Background(), req)
	require.NoError(t, err)

	acct := "9999999999999999999"
	results, total, err := store.FindAll(context.Background(), nil, &acct, 0, 10)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(results), 1)
	assert.GreaterOrEqual(t, total, 1)
}

func TestLoanRequestStore_FindAll_Pagination(t *testing.T) {
	skipIfNoDB(t)
	cleanupTables(testDB)

	store := NewLoanRequestStore(testDB)
	for i := 0; i < 3; i++ {
		_, err := store.Save(context.Background(), sampleLoanRequest())
		require.NoError(t, err)
	}

	results, _, err := store.FindAll(context.Background(), nil, nil, 0, 2)
	require.NoError(t, err)
	assert.LessOrEqual(t, len(results), 2)
}

func TestLoanRequestStore_FindByID_Success(t *testing.T) {
	skipIfNoDB(t)
	cleanupTables(testDB)

	store := NewLoanRequestStore(testDB)
	saved, err := store.Save(context.Background(), sampleLoanRequest())
	require.NoError(t, err)

	found, err := store.FindByID(context.Background(), saved.ID)
	require.NoError(t, err)
	assert.Equal(t, saved.ID, found.ID)
	assert.Equal(t, model.LoanGotovinski, found.LoanType)
}

func TestLoanRequestStore_FindByID_NotFound_ReturnsError(t *testing.T) {
	skipIfNoDB(t)
	cleanupTables(testDB)

	store := NewLoanRequestStore(testDB)

	_, err := store.FindByID(context.Background(), 999999)
	require.Error(t, err)
}

func TestLoanRequestStore_UpdateStatusIfPending_Success(t *testing.T) {
	skipIfNoDB(t)
	cleanupTables(testDB)

	store := NewLoanRequestStore(testDB)
	saved, err := store.Save(context.Background(), sampleLoanRequest())
	require.NoError(t, err)

	updated, err := store.UpdateStatusIfPending(context.Background(), saved.ID, model.StatusDeclined)
	require.NoError(t, err)
	assert.True(t, updated)
}

func TestLoanRequestStore_UpdateStatusIfPending_NotPending_ReturnsFalse(t *testing.T) {
	skipIfNoDB(t)
	cleanupTables(testDB)

	store := NewLoanRequestStore(testDB)
	saved, err := store.Save(context.Background(), sampleLoanRequest())
	require.NoError(t, err)

	// First decline
	_, err = store.UpdateStatusIfPending(context.Background(), saved.ID, model.StatusDeclined)
	require.NoError(t, err)

	// Second attempt should return false (already declined)
	updated, err := store.UpdateStatusIfPending(context.Background(), saved.ID, model.StatusDeclined)
	require.NoError(t, err)
	assert.False(t, updated)
}

func TestLoanRequestStore_ApproveWithLoanAndInstallment_Success(t *testing.T) {
	skipIfNoDB(t)
	cleanupTables(testDB)

	reqStore := NewLoanRequestStore(testDB)
	savedReq, err := reqStore.Save(context.Background(), sampleLoanRequest())
	require.NoError(t, err)

	loan := sampleLoan()
	installment := model.Installment{
		InstallmentAmount:     decimal.NewFromFloat(4400.00),
		InterestRateAtPayment: decimal.NewFromFloat(0.01),
		Currency:              model.CurrencyRSD,
		ExpectedDueDate:       time.Now().AddDate(0, 1, 0),
		PaymentStatus:         model.PaymentUnpaid,
		Retry:                 0,
	}

	savedLoan, err := reqStore.ApproveWithLoanAndInstallment(context.Background(), savedReq.ID, loan, installment)
	require.NoError(t, err)
	assert.Greater(t, savedLoan.ID, int64(0))
}

func TestLoanRequestStore_ApproveWithLoanAndInstallment_NotPending_ReturnsError(t *testing.T) {
	skipIfNoDB(t)
	cleanupTables(testDB)

	reqStore := NewLoanRequestStore(testDB)
	savedReq, err := reqStore.Save(context.Background(), sampleLoanRequest())
	require.NoError(t, err)

	// Decline the request first
	_, err = reqStore.UpdateStatusIfPending(context.Background(), savedReq.ID, model.StatusDeclined)
	require.NoError(t, err)

	// Try to approve an already-declined request
	_, err = reqStore.ApproveWithLoanAndInstallment(context.Background(), savedReq.ID, sampleLoan(), model.Installment{
		InstallmentAmount:     decimal.NewFromFloat(100),
		InterestRateAtPayment: decimal.NewFromFloat(0.01),
		Currency:              model.CurrencyRSD,
		ExpectedDueDate:       time.Now().AddDate(0, 1, 0),
		PaymentStatus:         model.PaymentUnpaid,
	})
	require.Error(t, err)
}

// ---------------------------------------------------------------------------
// LoanStore tests
// ---------------------------------------------------------------------------

func createTestLoan(t *testing.T, clientID int64) model.Loan {
	t.Helper()
	loanStore := NewLoanStore(testDB)
	loan := sampleLoan()
	loan.ClientID = clientID

	saved, err := loanStore.Create(context.Background(), loan)
	require.NoError(t, err)
	return saved
}

func TestLoanStore_Create_Success(t *testing.T) {
	skipIfNoDB(t)
	cleanupTables(testDB)

	store := NewLoanStore(testDB)
	loan := sampleLoan()

	saved, err := store.Create(context.Background(), loan)
	require.NoError(t, err)
	assert.Greater(t, saved.ID, int64(0))
	assert.Equal(t, model.LoanGotovinski, saved.LoanType)
}

func TestLoanStore_FindByClientID_Success(t *testing.T) {
	skipIfNoDB(t)
	cleanupTables(testDB)

	clientID := int64(2001)
	loan := createTestLoan(t, clientID)

	store := NewLoanStore(testDB)
	loans, total, err := store.FindByClientID(context.Background(), clientID, 0, 10)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(loans), 1)
	assert.GreaterOrEqual(t, total, 1)
	assert.Equal(t, loan.ID, loans[0].ID)
}

func TestLoanStore_FindByClientID_NoLoans_ReturnsEmpty(t *testing.T) {
	skipIfNoDB(t)
	cleanupTables(testDB)

	store := NewLoanStore(testDB)
	loans, total, err := store.FindByClientID(context.Background(), 99999, 0, 10)
	require.NoError(t, err)
	assert.Empty(t, loans)
	assert.Equal(t, 0, total)
}

func TestLoanStore_FindByID_Success(t *testing.T) {
	skipIfNoDB(t)
	cleanupTables(testDB)

	loan := createTestLoan(t, 3001)

	store := NewLoanStore(testDB)
	found, err := store.FindByID(context.Background(), loan.ID)
	require.NoError(t, err)
	assert.Equal(t, loan.ID, found.ID)
	assert.Equal(t, model.LoanGotovinski, found.LoanType)
}

func TestLoanStore_FindByID_NotFound_ReturnsError(t *testing.T) {
	skipIfNoDB(t)
	cleanupTables(testDB)

	store := NewLoanStore(testDB)
	_, err := store.FindByID(context.Background(), 999999)
	require.Error(t, err)
}

func TestLoanStore_FindAllWithFilters_NoFilters(t *testing.T) {
	skipIfNoDB(t)
	cleanupTables(testDB)

	createTestLoan(t, 4001)
	createTestLoan(t, 4002)

	store := NewLoanStore(testDB)
	loans, total, err := store.FindAllWithFilters(context.Background(), nil, nil, nil, 0, 10)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(loans), 2)
	assert.GreaterOrEqual(t, total, 2)
}

func TestLoanStore_FindAllWithFilters_ByLoanType(t *testing.T) {
	skipIfNoDB(t)
	cleanupTables(testDB)

	createTestLoan(t, 5001)

	store := NewLoanStore(testDB)
	lt := model.LoanGotovinski
	loans, total, err := store.FindAllWithFilters(context.Background(), &lt, nil, nil, 0, 10)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(loans), 1)
	assert.GreaterOrEqual(t, total, 1)
}

func TestLoanStore_FindAllWithFilters_ByStatus(t *testing.T) {
	skipIfNoDB(t)
	cleanupTables(testDB)

	createTestLoan(t, 6001)

	store := NewLoanStore(testDB)
	status := model.StatusActive
	loans, total, err := store.FindAllWithFilters(context.Background(), nil, nil, &status, 0, 10)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(loans), 1)
	assert.GreaterOrEqual(t, total, 1)
}

func TestLoanStore_FindAllWithFilters_ByAccountNumber(t *testing.T) {
	skipIfNoDB(t)
	cleanupTables(testDB)

	createTestLoan(t, 7001)

	store := NewLoanStore(testDB)
	acct := "1234567890123456789"
	loans, total, err := store.FindAllWithFilters(context.Background(), nil, &acct, nil, 0, 10)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(loans), 1)
	assert.GreaterOrEqual(t, total, 1)
}

func TestLoanStore_MarkOverdue_Success(t *testing.T) {
	skipIfNoDB(t)
	cleanupTables(testDB)

	loan := createTestLoan(t, 8001)

	store := NewLoanStore(testDB)
	err := store.MarkOverdue(context.Background(), loan.ID)
	require.NoError(t, err)

	found, err := store.FindByID(context.Background(), loan.ID)
	require.NoError(t, err)
	assert.Equal(t, model.StatusOverdue, found.Status)
}

func TestLoanStore_UpdateAfterInstallmentPayment_Success(t *testing.T) {
	skipIfNoDB(t)
	cleanupTables(testDB)

	loan := createTestLoan(t, 9001)

	store := NewLoanStore(testDB)
	newDebt := decimal.NewFromInt(45000)
	nextDate := time.Now().AddDate(0, 1, 0)

	err := store.UpdateAfterInstallmentPayment(
		context.Background(),
		loan.ID,
		newDebt,
		1,
		nextDate,
		model.StatusActive,
	)
	require.NoError(t, err)

	found, err := store.FindByID(context.Background(), loan.ID)
	require.NoError(t, err)
	assert.True(t, found.RemainingDebt.Equal(newDebt))
	assert.Equal(t, 1, found.InstallmentCount)
}

func TestLoanStore_UpdateAfterInstallmentPayment_PaidOff(t *testing.T) {
	skipIfNoDB(t)
	cleanupTables(testDB)

	loan := createTestLoan(t, 10001)

	store := NewLoanStore(testDB)
	err := store.UpdateAfterInstallmentPayment(
		context.Background(),
		loan.ID,
		decimal.Zero,
		12,
		loan.NextInstallmentDate,
		model.StatusPaidOff,
	)
	require.NoError(t, err)

	found, err := store.FindByID(context.Background(), loan.ID)
	require.NoError(t, err)
	assert.Equal(t, model.StatusPaidOff, found.Status)
}

// ---------------------------------------------------------------------------
// InstallmentStore tests
// ---------------------------------------------------------------------------

func createTestInstallment(t *testing.T, loanID int64, dueDate time.Time, status model.PaymentStatus) {
	t.Helper()
	store := NewInstallmentStore(testDB)
	installment := model.Installment{
		LoanID:                loanID,
		InstallmentAmount:     decimal.NewFromFloat(4400.00),
		InterestRateAtPayment: decimal.NewFromFloat(0.01),
		Currency:              model.CurrencyRSD,
		ExpectedDueDate:       dueDate,
		ActualDueDate:         nil,
		PaymentStatus:         status,
		Retry:                 0,
	}
	err := store.Create(context.Background(), installment)
	require.NoError(t, err)
}

func TestInstallmentStore_Create_Success(t *testing.T) {
	skipIfNoDB(t)
	cleanupTables(testDB)

	loan := createTestLoan(t, 11001)

	store := NewInstallmentStore(testDB)
	installment := model.Installment{
		LoanID:                loan.ID,
		InstallmentAmount:     decimal.NewFromFloat(4400.00),
		InterestRateAtPayment: decimal.NewFromFloat(0.01),
		Currency:              model.CurrencyRSD,
		ExpectedDueDate:       time.Now().AddDate(0, 1, 0),
		PaymentStatus:         model.PaymentUnpaid,
		Retry:                 0,
	}

	err := store.Create(context.Background(), installment)
	require.NoError(t, err)
}

func TestInstallmentStore_FindByLoanID_Success(t *testing.T) {
	skipIfNoDB(t)
	cleanupTables(testDB)

	loan := createTestLoan(t, 12001)
	createTestInstallment(t, loan.ID, time.Now().AddDate(0, 1, 0), model.PaymentUnpaid)

	store := NewInstallmentStore(testDB)
	installments, err := store.FindByLoanID(context.Background(), loan.ID)
	require.NoError(t, err)
	assert.Len(t, installments, 1)
}

func TestInstallmentStore_FindByLoanID_Empty_ReturnsEmpty(t *testing.T) {
	skipIfNoDB(t)
	cleanupTables(testDB)

	store := NewInstallmentStore(testDB)
	installments, err := store.FindByLoanID(context.Background(), 99999)
	require.NoError(t, err)
	assert.Empty(t, installments)
}

func TestInstallmentStore_FindDueUnpaid_ReturnsDue(t *testing.T) {
	skipIfNoDB(t)
	cleanupTables(testDB)

	loan := createTestLoan(t, 13001)
	// Due date in the past = due
	createTestInstallment(t, loan.ID, time.Now().AddDate(0, 0, -1), model.PaymentUnpaid)

	store := NewInstallmentStore(testDB)
	due, err := store.FindDueUnpaid(context.Background())
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(due), 1)
}

func TestInstallmentStore_FindDueUnpaid_FutureDate_NotReturned(t *testing.T) {
	skipIfNoDB(t)
	cleanupTables(testDB)

	loan := createTestLoan(t, 14001)
	// Due date in the future = not due
	createTestInstallment(t, loan.ID, time.Now().AddDate(0, 1, 0), model.PaymentUnpaid)

	store := NewInstallmentStore(testDB)
	due, err := store.FindDueUnpaid(context.Background())
	require.NoError(t, err)
	for _, d := range due {
		assert.NotEqual(t, loan.ID, d.LoanID)
	}
}

func TestInstallmentStore_FindDueUnpaid_PaidNotReturned(t *testing.T) {
	skipIfNoDB(t)
	cleanupTables(testDB)

	loan := createTestLoan(t, 15001)
	// Past due date but PAID = should not appear
	createTestInstallment(t, loan.ID, time.Now().AddDate(0, 0, -1), model.PaymentPaid)

	store := NewInstallmentStore(testDB)
	due, err := store.FindDueUnpaid(context.Background())
	require.NoError(t, err)
	for _, d := range due {
		assert.NotEqual(t, loan.ID, d.LoanID)
	}
}

func TestInstallmentStore_MarkRetryOrOverdue_FirstRetry(t *testing.T) {
	skipIfNoDB(t)
	cleanupTables(testDB)

	loan := createTestLoan(t, 16001)
	createTestInstallment(t, loan.ID, time.Now().AddDate(0, 0, -1), model.PaymentUnpaid)

	instStore := NewInstallmentStore(testDB)
	installments, err := instStore.FindByLoanID(context.Background(), loan.ID)
	require.NoError(t, err)
	require.Len(t, installments, 1)

	err = instStore.MarkRetryOrOverdue(context.Background(), installments[0])
	require.NoError(t, err)
}

func TestInstallmentStore_MarkRetryOrOverdue_AlreadyRetried(t *testing.T) {
	skipIfNoDB(t)
	cleanupTables(testDB)

	loan := createTestLoan(t, 17001)
	createTestInstallment(t, loan.ID, time.Now().AddDate(0, 0, -1), model.PaymentUnpaid)

	instStore := NewInstallmentStore(testDB)
	installments, err := instStore.FindByLoanID(context.Background(), loan.ID)
	require.NoError(t, err)
	require.Len(t, installments, 1)

	// Mark with retry=1 to simulate already-retried state
	overdue := installments[0]
	overdue.Retry = 1
	overdue.PaymentStatus = model.PaymentOverdue

	err = instStore.MarkRetryOrOverdue(context.Background(), overdue)
	require.NoError(t, err)
}

func TestInstallmentStore_MarkPaid_Success(t *testing.T) {
	skipIfNoDB(t)
	cleanupTables(testDB)

	loan := createTestLoan(t, 18001)
	createTestInstallment(t, loan.ID, time.Now().AddDate(0, 1, 0), model.PaymentUnpaid)

	instStore := NewInstallmentStore(testDB)
	installments, err := instStore.FindByLoanID(context.Background(), loan.ID)
	require.NoError(t, err)
	require.Len(t, installments, 1)

	err = instStore.MarkPaid(context.Background(), installments[0].ID)
	require.NoError(t, err)

	found, err := instStore.FindByLoanID(context.Background(), loan.ID)
	require.NoError(t, err)
	assert.Equal(t, model.PaymentPaid, found[0].PaymentStatus)
}
