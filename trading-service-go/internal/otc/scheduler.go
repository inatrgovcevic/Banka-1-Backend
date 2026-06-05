package otc

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
)

// ExpireOverdueContracts mirrors ExpireOverdueContractsScheduler.expireOverdueContracts
// (cron 0 5 0 * * *, gated by OTC_SCHEDULERS_ENABLED). Flips ACTIVE contracts whose
// settlementDate has passed to EXPIRED and releases the seller's reserved stock back
// to public availability. Each contract is its own transaction (one failure does
// not roll back the rest — more robust than Java's single @Transactional sweep; the
// happy-path result is identical).
func (s *Service) ExpireOverdueContracts(ctx context.Context) error {
	today := truncateToDate(time.Now())
	stale, err := s.repo.FindContractsByStatusAndSettlementDateBefore(ctx, ContractActive, today)
	if err != nil {
		return err
	}
	if len(stale) == 0 {
		s.logger.Debug("otc expireOverdueContracts: nothing to expire", "today", today.Format("2006-01-02"))
		return nil
	}
	expired := 0
	for i := range stale {
		c := stale[i]
		err := s.runInTx(ctx, func(tx pgx.Tx) error {
			if err := s.repo.UpdateOptionContractStatus(ctx, tx, c.ID, ContractExpired); err != nil {
				return err
			}
			return s.releaseForContract(ctx, tx, c.SellerID, c.StockTicker, c.Amount)
		})
		if err != nil {
			s.logger.Error("otc expire contract failed", "contractId", c.ID, "error", err)
			continue
		}
		expired++
		s.logger.Info("expired OTC option contract", "contractId", c.ID, "ticker", c.StockTicker,
			"buyer", c.BuyerID, "seller", c.SellerID, "settled", c.SettlementDate.Format("2006-01-02"))
	}
	s.logger.Info("otc expireOverdueContracts done", "expired", expired, "today", today.Format("2006-01-02"))
	return nil
}

// SendExpiryReminders mirrors ExpireOverdueContractsScheduler.sendExpiryReminders
// (cron 0 30 8 * * *, gated). For ACTIVE contracts settling exactly reminderDays
// from today (D-N), send one reminder. The reminder is sent only when this run won
// the insert race on the (contract_id, reminder_days) idempotency marker —
// at-most-once even under concurrency (the platform §B insert-then-skip pattern).
func (s *Service) SendExpiryReminders(ctx context.Context, reminderDays int) error {
	target := truncateToDate(time.Now()).AddDate(0, 0, reminderDays)
	contracts, err := s.repo.FindContractsByStatusAndSettlementDate(ctx, ContractActive, target)
	if err != nil {
		return err
	}
	sent := 0
	for i := range contracts {
		c := contracts[i]
		inserted, err := s.repo.InsertExpiryReminderIfAbsent(ctx, s.repo.Pool(), c.ID, reminderDays)
		if err != nil {
			s.logger.Error("otc expiry reminder marker insert failed", "contractId", c.ID, "error", err)
			continue
		}
		if !inserted {
			continue
		}
		s.notifier.ExpiryReminder(ctx, &c, reminderDays)
		sent++
	}
	s.logger.Info("otc sendExpiryReminders done", "sent", sent, "reminderDays", reminderDays,
		"target", target.Format("2006-01-02"))
	return nil
}

// truncateToDate returns t at 00:00:00 UTC — the calendar date used for the
// settlement-date comparisons (cast to ::date in the repo queries).
func truncateToDate(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}
