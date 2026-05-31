package service

import (
	"database/sql"
	"testing"
	"time"

	"banka1/banking-core-service-go/internal/decimal"
)

// ── NewPaymentRequest.validate() ─────────────────────────────────────────────

func TestNewPaymentRequestValidate(t *testing.T) {
	good := NewPaymentRequest{
		FromAccountNumber:     "ACC001",
		ToAccountNumber:       "ACC002",
		Amount:                decimal.MustParse("100.00"),
		RecipientName:         "Petar Petrovic",
		PaymentCode:           "289",
		PaymentPurpose:        "kirija",
		VerificationSessionID: 1,
	}

	cases := []struct {
		name    string
		mutate  func(*NewPaymentRequest)
		wantErr bool
	}{
		{"valid", func(r *NewPaymentRequest) {}, false},
		{"empty fromAccount", func(r *NewPaymentRequest) { r.FromAccountNumber = "" }, true},
		{"whitespace fromAccount", func(r *NewPaymentRequest) { r.FromAccountNumber = "   " }, true},
		{"empty toAccount", func(r *NewPaymentRequest) { r.ToAccountNumber = "" }, true},
		{"zero amount", func(r *NewPaymentRequest) { r.Amount = decimal.Zero }, true},
		{"negative amount", func(r *NewPaymentRequest) { r.Amount = decimal.MustParse("-1") }, true},
		{"empty recipientName", func(r *NewPaymentRequest) { r.RecipientName = "" }, true},
		{"empty paymentPurpose", func(r *NewPaymentRequest) { r.PaymentPurpose = "" }, true},
		{"zero verificationSessionID", func(r *NewPaymentRequest) { r.VerificationSessionID = 0 }, true},
		{"paymentCode not starting with 2", func(r *NewPaymentRequest) { r.PaymentCode = "189" }, true},
		{"paymentCode too short", func(r *NewPaymentRequest) { r.PaymentCode = "29" }, true},
		{"paymentCode too long", func(r *NewPaymentRequest) { r.PaymentCode = "2891" }, true},
		{"paymentCode with letters", func(r *NewPaymentRequest) { r.PaymentCode = "2AB" }, true},
		{"paymentCode 200", func(r *NewPaymentRequest) { r.PaymentCode = "200" }, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := good
			tc.mutate(&req)
			err := req.validate()
			if (err != nil) != tc.wantErr {
				t.Fatalf("validate() error = %v, wantErr = %v", err, tc.wantErr)
			}
		})
	}
}

// ── PaymentRecipientRequest.validate() ───────────────────────────────────────

func TestPaymentRecipientRequestValidate(t *testing.T) {
	good := PaymentRecipientRequest{
		Naziv:      "Moj primalac",
		BrojRacuna: "111000110000000312",
	}

	cases := []struct {
		name    string
		mutate  func(*PaymentRecipientRequest)
		wantErr bool
	}{
		{"valid", func(r *PaymentRecipientRequest) {}, false},
		{"empty naziv", func(r *PaymentRecipientRequest) { r.Naziv = "" }, true},
		{"whitespace naziv", func(r *PaymentRecipientRequest) { r.Naziv = "  " }, true},
		{"naziv over 100 chars", func(r *PaymentRecipientRequest) { r.Naziv = string(make([]byte, 101)) }, true},
		{"empty brojRacuna", func(r *PaymentRecipientRequest) { r.BrojRacuna = "" }, true},
		{"brojRacuna over 50 chars", func(r *PaymentRecipientRequest) { r.BrojRacuna = string(make([]byte, 51)) }, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := good
			tc.mutate(&req)
			err := req.validate()
			if (err != nil) != tc.wantErr {
				t.Fatalf("validate() error = %v, wantErr = %v", err, tc.wantErr)
			}
		})
	}
}

// ── TransferRequest.validate() ───────────────────────────────────────────────

func TestTransferRequestValidate(t *testing.T) {
	good := TransferRequest{
		FromAccountNumber:     "ACC001",
		ToAccountNumber:       "ACC002",
		Amount:                decimal.MustParse("500.00"),
		VerificationSessionID: 7,
	}

	cases := []struct {
		name    string
		mutate  func(*TransferRequest)
		wantErr bool
	}{
		{"valid", func(r *TransferRequest) {}, false},
		{"empty fromAccount", func(r *TransferRequest) { r.FromAccountNumber = "" }, true},
		{"empty toAccount", func(r *TransferRequest) { r.ToAccountNumber = "" }, true},
		{"zero amount", func(r *TransferRequest) { r.Amount = decimal.Zero }, true},
		{"negative amount", func(r *TransferRequest) { r.Amount = decimal.MustParse("-0.01") }, true},
		{"zero verificationSessionID", func(r *TransferRequest) { r.VerificationSessionID = 0 }, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := good
			tc.mutate(&req)
			err := req.validate()
			if (err != nil) != tc.wantErr {
				t.Fatalf("validate() error = %v, wantErr = %v", err, tc.wantErr)
			}
		})
	}
}

// ── validateStockMarginRequest ───────────────────────────────────────────────

func TestValidateStockMarginRequest(t *testing.T) {
	uid := int64(1)
	cid := int64(2)

	cases := []struct {
		name    string
		req     StockMarginTransactionRequest
		wantErr bool
	}{
		{
			"valid with userId",
			StockMarginTransactionRequest{UserID: &uid, Amount: decimal.MustParse("100")},
			false,
		},
		{
			"valid with companyId",
			StockMarginTransactionRequest{CompanyID: &cid, Amount: decimal.MustParse("100")},
			false,
		},
		{
			"both nil — invalid",
			StockMarginTransactionRequest{Amount: decimal.MustParse("100")},
			true,
		},
		{
			"both set — invalid",
			StockMarginTransactionRequest{UserID: &uid, CompanyID: &cid, Amount: decimal.MustParse("100")},
			true,
		},
		{
			"zero amount",
			StockMarginTransactionRequest{UserID: &uid, Amount: decimal.Zero},
			true,
		},
		{
			"negative amount",
			StockMarginTransactionRequest{UserID: &uid, Amount: decimal.MustParse("-1")},
			true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateStockMarginRequest(tc.req)
			if (err != nil) != tc.wantErr {
				t.Fatalf("validateStockMarginRequest() error = %v, wantErr = %v", err, tc.wantErr)
			}
		})
	}
}

// ── validateMarginCreate ─────────────────────────────────────────────────────

func TestValidateMarginCreate(t *testing.T) {
	cases := []struct {
		name        string
		initial     decimal.Decimal
		maintenance decimal.Decimal
		bankPart    decimal.Decimal
		wantErr     bool
	}{
		{"valid", decimal.MustParse("1000"), decimal.MustParse("500"), decimal.MustParse("0.5"), false},
		{"zero initial", decimal.Zero, decimal.MustParse("500"), decimal.MustParse("0.5"), true},
		{"negative initial", decimal.MustParse("-1"), decimal.MustParse("500"), decimal.MustParse("0.5"), true},
		{"zero maintenance", decimal.MustParse("1000"), decimal.Zero, decimal.MustParse("0.5"), true},
		{"negative maintenance", decimal.MustParse("1000"), decimal.MustParse("-1"), decimal.MustParse("0.5"), true},
		{"bankParticipation negative", decimal.MustParse("1000"), decimal.MustParse("500"), decimal.MustParse("-0.1"), true},
		{"bankParticipation above 1", decimal.MustParse("1000"), decimal.MustParse("500"), decimal.MustParse("1.01"), true},
		{"bankParticipation exactly 0", decimal.MustParse("1000"), decimal.MustParse("500"), decimal.Zero, false},
		{"bankParticipation exactly 1", decimal.MustParse("1000"), decimal.MustParse("500"), decimal.One, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateMarginCreate(tc.initial, tc.maintenance, tc.bankPart)
			if (err != nil) != tc.wantErr {
				t.Fatalf("validateMarginCreate() error = %v, wantErr = %v", err, tc.wantErr)
			}
		})
	}
}

// ── validateOwnerInput ───────────────────────────────────────────────────────

func TestValidateOwnerInput(t *testing.T) {
	id := int64(1)
	cases := []struct {
		name    string
		id      *int64
		jmbg    string
		wantErr bool
	}{
		{"both empty", nil, "", true},
		{"both whitespace jmbg", nil, "   ", true},
		{"id provided", &id, "", false},
		{"jmbg provided", nil, "0101990710023", false},
		{"both provided", &id, "0101990710023", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateOwnerInput(tc.id, tc.jmbg)
			if (err != nil) != tc.wantErr {
				t.Fatalf("validateOwnerInput() error = %v, wantErr = %v", err, tc.wantErr)
			}
		})
	}
}

// ── validateCompanyPresence ──────────────────────────────────────────────────

func TestValidateCompanyPresence(t *testing.T) {
	company := &CompanyRequest{}

	cases := []struct {
		name      string
		company   *CompanyRequest
		ownership string
		wantErr   bool
	}{
		{"business with company", company, "BUSINESS", false},
		{"personal without company", nil, "PERSONAL", false},
		{"business without company", nil, "BUSINESS", true},
		{"personal with company", company, "PERSONAL", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateCompanyPresence(tc.company, tc.ownership)
			if (err != nil) != tc.wantErr {
				t.Fatalf("validateCompanyPresence() error = %v, wantErr = %v", err, tc.wantErr)
			}
		})
	}
}

// ── accountConcrete ──────────────────────────────────────────────────────────

func TestAccountConcrete(t *testing.T) {
	personalTypes := []string{"STANDARDNI", "STEDNI", "PENZIONERSKI", "ZA_MLADE", "ZA_STUDENTE", "ZA_NEZAPOSLENE"}
	for _, typ := range personalTypes {
		t.Run(typ, func(t *testing.T) {
			ownership, _, err := accountConcrete(typ)
			if err != nil {
				t.Fatalf("accountConcrete(%q) unexpected error: %v", typ, err)
			}
			if ownership != "PERSONAL" {
				t.Fatalf("accountConcrete(%q) ownership = %q, want PERSONAL", typ, ownership)
			}
		})
	}

	businessTypes := []string{"DOO", "AD", "FONDACIJA"}
	for _, typ := range businessTypes {
		t.Run(typ, func(t *testing.T) {
			ownership, _, err := accountConcrete(typ)
			if err != nil {
				t.Fatalf("accountConcrete(%q) unexpected error: %v", typ, err)
			}
			if ownership != "BUSINESS" {
				t.Fatalf("accountConcrete(%q) ownership = %q, want BUSINESS", typ, ownership)
			}
		})
	}

	t.Run("case insensitive", func(t *testing.T) {
		ownership, _, err := accountConcrete("standardni")
		if err != nil {
			t.Fatalf("accountConcrete(lowercase) unexpected error: %v", err)
		}
		if ownership != "PERSONAL" {
			t.Fatalf("accountConcrete(lowercase) ownership = %q, want PERSONAL", ownership)
		}
	})

	t.Run("unknown type", func(t *testing.T) {
		_, _, err := accountConcrete("NEPOZNATA")
		if err == nil {
			t.Fatal("accountConcrete(unknown) should return error")
		}
	})
}

// ── accountBalanceRow.validateMutable ────────────────────────────────────────

func TestValidateMutable(t *testing.T) {
	active := accountBalanceRow{Status: "ACTIVE", OwnerID: 5}

	cases := []struct {
		name     string
		row      accountBalanceRow
		clientID int64
		wantErr  bool
	}{
		{"active owner match", active, 5, false},
		{"bank account (ownerID=-1) any clientID", accountBalanceRow{Status: "ACTIVE", OwnerID: -1}, 99, false},
		{"inactive account", accountBalanceRow{Status: "INACTIVE", OwnerID: 5}, 5, true},
		{"expired account", accountBalanceRow{
			Status:    "ACTIVE",
			OwnerID:   5,
			ExpiresAt: sql.NullTime{Valid: true, Time: time.Now().Add(-time.Hour)},
		}, 5, true},
		{"wrong owner", accountBalanceRow{Status: "ACTIVE", OwnerID: 5}, 6, true},
		{"future expiry is ok", accountBalanceRow{
			Status:    "ACTIVE",
			OwnerID:   5,
			ExpiresAt: sql.NullTime{Valid: true, Time: time.Now().Add(time.Hour)},
		}, 5, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.row.validateMutable("ACC001", tc.clientID)
			if (err != nil) != tc.wantErr {
				t.Fatalf("validateMutable() error = %v, wantErr = %v", err, tc.wantErr)
			}
		})
	}
}

// ── Principal.IsPrivileged ────────────────────────────────────────────────────

func TestPrincipalIsPrivileged(t *testing.T) {
	cases := []struct {
		name  string
		roles []string
		want  bool
	}{
		{"ADMIN", []string{"ADMIN"}, true},
		{"SUPERVISOR", []string{"SUPERVISOR"}, true},
		{"AGENT", []string{"AGENT"}, true},
		{"BASIC", []string{"BASIC"}, true},
		{"SERVICE", []string{"SERVICE"}, true},
		{"CLIENT_TRADING", []string{"CLIENT_TRADING"}, false},
		{"CLIENT_BASIC", []string{"CLIENT_BASIC"}, false},
		{"empty", []string{}, false},
		{"BASIC among clients", []string{"CLIENT_BASIC", "BASIC"}, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := Principal{ID: 1, Roles: tc.roles}
			if got := p.IsPrivileged(); got != tc.want {
				t.Fatalf("IsPrivileged() = %v, want %v for roles %v", got, tc.want, tc.roles)
			}
		})
	}
}
