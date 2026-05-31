package service

import (
	"context"
	"log"
	"time"
)

func StartInstallmentScheduler(ctx context.Context, loanService *LoanService) {
	ticker := time.NewTicker(24 * time.Hour)

	go func() {
		log.Println("installment scheduler started")

		err := loanService.ProcessDueInstallments(ctx)
		if err != nil {
			log.Println("installment scheduler initial run failed:", err)
		}

		for {
			select {
			case <-ctx.Done():
				log.Println("installment scheduler stopped")
				ticker.Stop()
				return

			case <-ticker.C:
				err := loanService.ProcessDueInstallments(ctx)
				if err != nil {
					log.Println("installment scheduler run failed:", err)
				}
			}
		}
	}()
}
