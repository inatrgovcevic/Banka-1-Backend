package service

import (
	"database/sql"
	"net/http"

	"banka1/banking-core-service-go/internal/account"
	"banka1/banking-core-service-go/internal/card"
	"banka1/banking-core-service-go/internal/config"
)

type Container struct {
	Config config.Config

	Accounts       *AccountService
	MarginAccounts *MarginAccountService
	MarginTx       *MarginTransactionService
	Internal       *InternalService
	Interbank      *InterbankService
	CardService    *CardService
	Verification   *VerificationService
	Transactions   *TransactionService
	Transfers      *TransferService
	Rabbit         *RabbitPublisher
	Cards          CardServices
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
	accountSvc.SetAutomaticCardCreator(cardSvc)
	return &Container{
		Config:         cfg,
		Accounts:       accountSvc,
		MarginAccounts: marginAccounts,
		MarginTx:       NewMarginTransactionService(db, cfg, accountSvc, marginAccounts),
		Internal:       internal,
		Interbank:      NewInterbankService(db, accountSvc),
		CardService:    cardSvc,
		Verification:   verificationSvc,
		Transactions:   transactionSvc,
		Transfers:      NewTransferService(db, cfg, accountSvc, transactionSvc, verificationSvc, rabbit),
		Rabbit:         rabbit,
		Cards: CardServices{
			LuhnValidator:           card.LuhnValidator{},
			BrandDetector:           card.BrandDetector{},
			MasterCardFeeCalculator: cardCalc,
			FXFeeApplier:            card.FXFeeApplier{MasterCard: cardCalc},
		},
	}
}
