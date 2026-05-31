package service

import (
	"context"
	"database/sql"
	"log"
	"time"

	"banka1/banking-core-service-go/internal/config"
	"banka1/banking-core-service-go/internal/decimal"
)

// ScheduledJobs portuje @Scheduled poslove iz konsolidovanih legacy servisa:
//   - account-service Scheduler.resetDailySpending  (cron "0 0 0 * * *")
//   - account-service Scheduler.resetMonthlySpending (cron "0 0 0 1 * *")
//   - account-service Scheduler.run -> MaintenanceFeeService.process (cron "0 0 0 1 * *")
//   - transaction-service TransactionServiceInternalImplementation.cleanup (fixedRate=100000)
//
// account-service Scheduler ne koristi @SchedulerLock (za razliku od
// ExternalTransferRetryScheduler), pa ovde ne koristimo distribuirani lock —
// banking-core je jedinstveni konsolidovani deployment.
type ScheduledJobs struct {
	db  *sql.DB
	cfg config.Config
}

func NewScheduledJobs(db *sql.DB, cfg config.Config) *ScheduledJobs {
	return &ScheduledJobs{db: db, cfg: cfg}
}

func (s *ScheduledJobs) Start(ctx context.Context) {
	if s == nil {
		return
	}
	go s.runCron(ctx, nextDailyMidnight, func(ctx context.Context) {
		s.resetDailySpending(ctx)
	})
	go s.runCron(ctx, nextMonthFirstMidnight, func(ctx context.Context) {
		// Dva @Scheduled metoda sa istim cron-om ("0 0 0 1 * *").
		s.resetMonthlySpending(ctx)
		s.processMaintenanceFees(ctx)
	})
	go s.runStuckPaymentCleanup(ctx)
}

func (s *ScheduledJobs) runCron(ctx context.Context, nextFn func(time.Time) time.Time, job func(context.Context)) {
	for {
		next := nextFn(time.Now())
		timer := time.NewTimer(time.Until(next))
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
		job(ctx)
	}
}

// runStuckPaymentCleanup replicira @Scheduled(fixedRate = 100000): Spring pokrece
// prvi put gotovo odmah po startu, pa zatim svakih 100s.
func (s *ScheduledJobs) runStuckPaymentCleanup(ctx context.Context) {
	ticker := time.NewTicker(100 * time.Second)
	defer ticker.Stop()
	s.cleanupStuckPayments(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.cleanupStuckPayments(ctx)
		}
	}
}

func (s *ScheduledJobs) resetDailySpending(ctx context.Context) {
	res, err := s.db.ExecContext(ctx, `
UPDATE account_table
   SET dnevna_potrosnja = 0,
       daily_limit_remaining = dnevni_limit
 WHERE deleted = false
`)
	if err != nil {
		log.Printf("daily spending reset failed: %v", err)
		return
	}
	updated, _ := res.RowsAffected()
	log.Printf("Daily spending reset executed. Updated accounts: %d", updated)
}

func (s *ScheduledJobs) resetMonthlySpending(ctx context.Context) {
	res, err := s.db.ExecContext(ctx, "UPDATE account_table SET mesecna_potrosnja = 0 WHERE deleted = false")
	if err != nil {
		log.Printf("monthly spending reset failed: %v", err)
		return
	}
	updated, _ := res.RowsAffected()
	log.Printf("Monthly spending reset executed. Updated accounts: %d", updated)
}

func (s *ScheduledJobs) cleanupStuckPayments(ctx context.Context) {
	res, err := s.db.ExecContext(ctx, `
UPDATE payment_table
   SET status = 'DENIED'
 WHERE status = 'IN_PROGRESS'
   AND created_at < now() - INTERVAL '5 minutes'
`)
	if err != nil {
		log.Printf("stuck payment cleanup failed: %v", err)
		return
	}
	if updated, _ := res.RowsAffected(); updated > 0 {
		log.Printf("Marked %d stuck payments as DENIED", updated)
	}
}

type maintenanceFeeAccount struct {
	id          int64
	number      string
	stanje      decimal.Decimal
	raspolozivo decimal.Decimal
	fee         decimal.Decimal
}

// processMaintenanceFees portuje MaintenanceFeeServiceImplementation.process():
// za svaki aktivan tekuci racun sa definisanom naknadom oduzima naknadu (ako ima
// dovoljno raspolozivog stanja) i kreditira banka RSD racun, uz audit zapis.
func (s *ScheduledJobs) processMaintenanceFees(ctx context.Context) {
	log.Printf("Starting monthly maintenance fee job")
	if err := s.runMaintenanceFees(ctx); err != nil {
		log.Printf("monthly maintenance fee job failed: %v", err)
		return
	}
	log.Printf("Finished monthly maintenance fee job")
}

func (s *ScheduledJobs) runMaintenanceFees(ctx context.Context) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var bankID int64
	var bankNumber string
	var bankStanje, bankRasp decimal.Decimal
	err = tx.QueryRowContext(ctx, `
SELECT a.id, a.broj_racuna, a.stanje, a.raspolozivo_stanje
  FROM account_table a
  JOIN currency_table c ON c.id = a.currency_id
 WHERE a.vlasnik = $1 AND c.oznaka = 'RSD'
 LIMIT 1
`, s.cfg.BankClientID).Scan(&bankID, &bankNumber, &bankStanje, &bankRasp)
	if err == sql.ErrNoRows {
		log.Printf("monthly maintenance fee job: Bank RSD account not found")
		return nil
	}
	if err != nil {
		return err
	}

	rows, err := tx.QueryContext(ctx, `
SELECT id, broj_racuna, stanje, raspolozivo_stanje, odrzavanje_racuna
  FROM account_table
 WHERE account_type = 'CHECKING'
   AND status = 'ACTIVE'
   AND deleted = false
   AND odrzavanje_racuna IS NOT NULL
   AND odrzavanje_racuna > 0
`)
	if err != nil {
		return err
	}
	accounts := []maintenanceFeeAccount{}
	for rows.Next() {
		var a maintenanceFeeAccount
		if err := rows.Scan(&a.id, &a.number, &a.stanje, &a.raspolozivo, &a.fee); err != nil {
			rows.Close()
			return err
		}
		accounts = append(accounts, a)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()

	total := decimal.Zero
	for _, acc := range accounts {
		if acc.fee.Sign() <= 0 {
			continue
		}
		if acc.raspolozivo.Cmp(acc.fee) < 0 {
			log.Printf("Insufficient funds for fee | acc=%s balance=%s fee=%s", acc.number, acc.raspolozivo.String(), acc.fee.String())
			continue
		}
		newStanje := acc.stanje.Sub(acc.fee)
		newRasp := acc.raspolozivo.Sub(acc.fee)
		if _, err := tx.ExecContext(ctx, "UPDATE account_table SET stanje = $1, raspolozivo_stanje = $2, updated_at = now() WHERE id = $3", newStanje, newRasp, acc.id); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, "INSERT INTO transaction_record_table (account_number, bank_account_number, amount) VALUES ($1, $2, $3)", acc.number, bankNumber, acc.fee); err != nil {
			return err
		}
		total = total.Add(acc.fee)
		log.Printf("Fee deducted | acc=%s fee=%s newBalance=%s", acc.number, acc.fee.String(), newStanje.String())
	}

	if _, err := tx.ExecContext(ctx, "UPDATE account_table SET stanje = $1, raspolozivo_stanje = $2, updated_at = now() WHERE id = $3", bankStanje.Add(total), bankRasp.Add(total), bankID); err != nil {
		return err
	}
	log.Printf("Monthly maintenance done | accounts=%d total=%s", len(accounts), total.String())
	return tx.Commit()
}

func nextDailyMidnight(now time.Time) time.Time {
	y, m, d := now.Date()
	next := time.Date(y, m, d, 0, 0, 0, 0, now.Location())
	if !next.After(now) {
		next = next.AddDate(0, 0, 1)
	}
	return next
}

func nextMonthFirstMidnight(now time.Time) time.Time {
	y, m, _ := now.Date()
	next := time.Date(y, m, 1, 0, 0, 0, 0, now.Location())
	if !next.After(now) {
		next = next.AddDate(0, 1, 0)
	}
	return next
}
