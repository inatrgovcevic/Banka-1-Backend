package service

import (
	"context"
	"database/sql"
	"net/http"

	"banka1/banking-core-service-go/internal/account"
	"banka1/banking-core-service-go/internal/card"
	"banka1/banking-core-service-go/internal/config"
)

type Container struct {
	Config config.Config
	DB     *sql.DB

	Accounts          *AccountService
	MarginAccounts    *MarginAccountService
	MarginTx          *MarginTransactionService
	Internal          *InternalService
	Interbank         *InterbankService
	CardService       *CardService
	Verification      *VerificationService
	Transactions      *TransactionService
	Transfers         *TransferService
	ExternalTransfers *ExternalTransferService
	Gdpr              *GdprService
	Rabbit            *RabbitPublisher
	Scheduled         *ScheduledJobs
	Cards             CardServices
}

type CardServices struct {
	LuhnValidator           card.LuhnValidator
	BrandDetector           card.BrandDetector
	MasterCardFeeCalculator card.MasterCardFeeCalculator
	FXFeeApplier            card.FXFeeApplier
}

func NewContainer(cfg config.Config, db *sql.DB) *Container {
	rabbit := NewRabbitPublisher(cfg)
	accountSvc := NewAccountService(db, cfg, rabbit)
	cardCalc := card.NewMasterCardFeeCalculator(cfg.MasterCardFXFeePercent, cfg.MasterCardNetworkFee)
	market := NewMarketClient(cfg, http.DefaultClient)
	marginAccounts := NewMarginAccountService(db, account.NumberGenerator{})
	internal := NewInternalService(db, accountSvc, market)
	cardSvc := NewCardService(db, cfg, accountSvc, rabbit)
	verificationSvc := NewVerificationService(db, cfg, rabbit)
	transactionSvc := NewTransactionService(db, cfg, accountSvc, market, verificationSvc, rabbit)
	externalTransfers := NewExternalTransferService(db, cfg, rabbit)
	gdprSvc := NewGdprService(db, cfg)
	accountSvc.SetAutomaticCardCreator(cardSvc)
	return &Container{
		Config:            cfg,
		DB:                db,
		Accounts:          accountSvc,
		MarginAccounts:    marginAccounts,
		MarginTx:          NewMarginTransactionService(db, cfg, accountSvc, marginAccounts),
		Internal:          internal,
		Interbank:         NewInterbankService(db, accountSvc),
		CardService:       cardSvc,
		Verification:      verificationSvc,
		Transactions:      transactionSvc,
		Transfers:         NewTransferService(db, cfg, accountSvc, transactionSvc, verificationSvc, rabbit),
		ExternalTransfers: externalTransfers,
		Gdpr:              gdprSvc,
		Rabbit:            rabbit,
		Scheduled:         NewScheduledJobs(db, cfg),
		Cards: CardServices{
			LuhnValidator:           card.LuhnValidator{},
			BrandDetector:           card.BrandDetector{},
			MasterCardFeeCalculator: cardCalc,
			FXFeeApplier:            card.FXFeeApplier{MasterCard: cardCalc},
		},
	}
}

func (c *Container) StartBackground(ctx context.Context) {
	if c == nil {
		return
	}
	if c.ExternalTransfers != nil {
		c.ExternalTransfers.Start(ctx)
	}
	if c.Gdpr != nil {
		c.Gdpr.Start(ctx)
	}
	if c.Scheduled != nil {
		c.Scheduled.Start(ctx)
	}
}
