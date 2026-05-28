package service

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"banka1/banking-core-service-go/internal/config"
	"banka1/banking-core-service-go/internal/decimal"
	amqp "github.com/rabbitmq/amqp091-go"
)

type ExternalTransferService struct {
	db                    *sql.DB
	cfg                   config.Config
	rabbit                *RabbitPublisher
	http                  *http.Client
	clearingRetryBackoffs []time.Duration
	schedulerLock         *distributedLock
	cb                    *CircuitBreaker
}

type ExternalTransferRetrySummary struct {
	Escalated int `json:"escalated"`
	Retried   int `json:"retried"`
}

type externalTransferRow struct {
	ID               int64
	RetryCount       int
	Amount           decimal.Decimal
	RecipientAccount string
	Currency         string
}

type transferRetryEvent struct {
	TransferID       int64           `json:"transferId"`
	RetryAttempt     int             `json:"retryAttempt"`
	Amount           decimal.Decimal `json:"amount"`
	RecipientAccount string          `json:"recipientAccount"`
	Currency         string          `json:"currency"`
}

type transferEscalatedEvent struct {
	TransferID       int64           `json:"transferId"`
	RetryCount       int             `json:"retryCount"`
	Amount           decimal.Decimal `json:"amount"`
	RecipientAccount string          `json:"recipientAccount"`
	Currency         string          `json:"currency"`
	AlertOncall      bool            `json:"alertOncall"`
}

type clearingHouseIssueResult struct {
	Success          bool   `json:"success"`
	ClearingHouseRef string `json:"clearingHouseRef"`
	FailureReason    string `json:"failureReason"`
	retryableFailure bool
}

func (r clearingHouseIssueResult) retryable() bool {
	return !r.Success && r.retryableFailure
}

func NewExternalTransferService(db *sql.DB, cfg config.Config, rabbit *RabbitPublisher) *ExternalTransferService {
	return &ExternalTransferService{
		db:                    db,
		cfg:                   cfg,
		rabbit:                rabbit,
		http:                  &http.Client{Timeout: 30 * time.Second},
		clearingRetryBackoffs: []time.Duration{250 * time.Millisecond, 500 * time.Millisecond},
		schedulerLock:         newDistributedLock(db, "ExternalTransferRetry", 5*time.Minute, 30*time.Second),
		cb:                    NewCircuitBreaker(5, 2, 60*time.Second),
	}
}

func (s *ExternalTransferService) Start(ctx context.Context) {
	go s.runRetryScheduler(ctx)
	go s.consumeTransferRetries(ctx)
	go s.consumeTransferEscalations(ctx)
}

func (s *ExternalTransferService) RetryFailedExternalTransfers(ctx context.Context) (ExternalTransferRetrySummary, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ExternalTransferRetrySummary{}, err
	}
	defer tx.Rollback()

	summary := ExternalTransferRetrySummary{}
	escalations, err := s.externalTransfersForEscalation(ctx, tx)
	if err != nil {
		return ExternalTransferRetrySummary{}, err
	}
	for _, row := range escalations {
		if _, err := tx.ExecContext(ctx, "UPDATE external_transfers SET status = 'ESCALATED', updated_at = now() WHERE id = $1", row.ID); err != nil {
			return ExternalTransferRetrySummary{}, err
		}
		event := transferEscalatedEvent{
			TransferID:       row.ID,
			RetryCount:       row.RetryCount,
			Amount:           row.Amount,
			RecipientAccount: row.RecipientAccount,
			Currency:         row.Currency,
			AlertOncall:      true,
		}
		if err := s.rabbit.PublishJSON(ctx, s.cfg.TransferRetryExchange, "transfer.escalated", event); err != nil {
			return ExternalTransferRetrySummary{}, err
		}
		summary.Escalated++
	}

	retries, err := s.externalTransfersForRetry(ctx, tx)
	if err != nil {
		return ExternalTransferRetrySummary{}, err
	}
	for _, row := range retries {
		nextAttempt := row.RetryCount + 1
		if _, err := tx.ExecContext(ctx, `
UPDATE external_transfers
   SET retry_count = retry_count + 1,
       status = 'RETRYING',
       updated_at = now()
 WHERE id = $1
`, row.ID); err != nil {
			return ExternalTransferRetrySummary{}, err
		}
		event := transferRetryEvent{
			TransferID:       row.ID,
			RetryAttempt:     nextAttempt,
			Amount:           row.Amount,
			RecipientAccount: row.RecipientAccount,
			Currency:         row.Currency,
		}
		if err := s.rabbit.PublishJSON(ctx, s.cfg.TransferRetryExchange, "transfer.retry", event); err != nil {
			return ExternalTransferRetrySummary{}, err
		}
		summary.Retried++
	}

	if err := tx.Commit(); err != nil {
		return ExternalTransferRetrySummary{}, err
	}
	return summary, nil
}

func (s *ExternalTransferService) runRetryScheduler(ctx context.Context) {
	for {
		nextRun := nextExternalTransferRetryRun(time.Now())
		timer := time.NewTimer(time.Until(nextRun))
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
		ran, err := s.schedulerLock.try(ctx, func(ctx context.Context) error {
			summary, err := s.RetryFailedExternalTransfers(ctx)
			if err != nil {
				return err
			}
			log.Printf("external transfer retry finished: escalated=%d retried=%d", summary.Escalated, summary.Retried)
			return nil
		})
		if err != nil {
			log.Printf("external transfer retry failed: %v", err)
		} else if !ran {
			log.Printf("external transfer retry skipped: lock held by another instance")
		}
	}
}

func (s *ExternalTransferService) consumeTransferRetries(ctx context.Context) {
	s.consumeRabbitLoop(ctx, s.cfg.TransferRetryQueue, "transfer.retry", func(ctx context.Context, body []byte) error {
		var event transferRetryEvent
		if err := json.Unmarshal(body, &event); err != nil {
			return err
		}
		return s.onRetry(ctx, event)
	})
}

func (s *ExternalTransferService) consumeTransferEscalations(ctx context.Context) {
	s.consumeRabbitLoop(ctx, s.cfg.TransferEscalatedQueue, "transfer.escalated", func(ctx context.Context, body []byte) error {
		var event transferEscalatedEvent
		if err := json.Unmarshal(body, &event); err != nil {
			return err
		}
		log.Printf("ESCALATED transfer id=%d recipient=%s amount=%s currency=%s", event.TransferID, event.RecipientAccount, event.Amount.String(), event.Currency)
		return nil
	})
}

func (s *ExternalTransferService) consumeRabbitLoop(ctx context.Context, queueName, routingKey string, handler func(context.Context, []byte) error) {
	for {
		if err := s.consumeRabbit(ctx, queueName, routingKey, handler); err != nil && ctx.Err() == nil {
			log.Printf("rabbit consumer %s failed: %v", routingKey, err)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(30 * time.Second):
		}
	}
}

func (s *ExternalTransferService) consumeRabbit(ctx context.Context, queueName, routingKey string, handler func(context.Context, []byte) error) error {
	conn, err := amqp.Dial(s.cfg.RabbitURL())
	if err != nil {
		return err
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		return err
	}
	defer ch.Close()

	if err := ch.ExchangeDeclare(s.cfg.TransferRetryExchange, "topic", true, false, false, false, nil); err != nil {
		return err
	}
	queue, err := ch.QueueDeclare(queueName, true, false, false, false, nil)
	if err != nil {
		return err
	}
	if err := ch.QueueBind(queue.Name, routingKey, s.cfg.TransferRetryExchange, false, nil); err != nil {
		return err
	}
	deliveries, err := ch.Consume(queue.Name, "", false, false, false, false, nil)
	if err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case delivery, ok := <-deliveries:
			if !ok {
				return fmt.Errorf("rabbit deliveries closed for %s", routingKey)
			}
			if err := handler(ctx, delivery.Body); err != nil {
				_ = delivery.Nack(false, true)
				log.Printf("rabbit handler %s failed: %v", routingKey, err)
				continue
			}
			_ = delivery.Ack(false)
		}
	}
}

func (s *ExternalTransferService) onRetry(ctx context.Context, event transferRetryEvent) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var alreadyProcessed int
	if err := tx.QueryRowContext(ctx, `
SELECT COUNT(*)
  FROM transfer_retry_log
 WHERE transfer_id = $1 AND retry_attempt = $2
`, event.TransferID, event.RetryAttempt).Scan(&alreadyProcessed); err != nil {
		return err
	}
	if alreadyProcessed > 0 {
		return tx.Commit()
	}

	result := s.issueClearingHouseTransfer(ctx, event)
	if result.Success {
		if _, err := tx.ExecContext(ctx, `
UPDATE external_transfers
   SET status = 'COMPLETED',
       completed_at = now(),
       updated_at = now()
 WHERE id = $1
`, event.TransferID); err != nil {
			log.Printf("external transfer id=%d status update failed: %v", event.TransferID, err)
		}
	} else if strings.TrimSpace(result.FailureReason) != "" {
		log.Printf("external transfer id=%d clearing-house refused: %s", event.TransferID, result.FailureReason)
	}

	if _, err := tx.ExecContext(ctx, `
INSERT INTO transfer_retry_log (transfer_id, retry_attempt, processed_at)
VALUES ($1, $2, now())
`, event.TransferID, event.RetryAttempt); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *ExternalTransferService) issueClearingHouseTransfer(ctx context.Context, event transferRetryEvent) clearingHouseIssueResult {
	if !s.cb.Allow() {
		log.Printf("circuit open: skipping clearing-house call for transfer id=%d", event.TransferID)
		return clearingHouseIssueResult{Success: false, FailureReason: "circuit open: clearing-house unavailable", retryableFailure: true}
	}

	body, err := json.Marshal(map[string]any{
		"transferId":       event.TransferID,
		"amount":           event.Amount,
		"currency":         event.Currency,
		"recipientAccount": event.RecipientAccount,
	})
	if err != nil {
		return clearingHouseIssueResult{Success: false, FailureReason: err.Error()}
	}

	attempts := len(s.clearingRetryBackoffs) + 1
	var last clearingHouseIssueResult
	for attempt := 0; attempt < attempts; attempt++ {
		last = s.issueClearingHouseTransferOnce(ctx, event, body)
		if last.Success {
			s.cb.RecordSuccess()
			return last
		}
		if last.retryableFailure {
			s.cb.RecordFailure()
		}
		if !last.retryable() || attempt == attempts-1 {
			return last
		}
		timer := time.NewTimer(s.clearingRetryBackoffs[attempt])
		select {
		case <-ctx.Done():
			timer.Stop()
			return clearingHouseIssueResult{Success: false, FailureReason: ctx.Err().Error()}
		case <-timer.C:
		}
	}
	return last
}

func (s *ExternalTransferService) issueClearingHouseTransferOnce(ctx context.Context, event transferRetryEvent, body []byte) clearingHouseIssueResult {
	base := strings.TrimRight(s.cfg.ClearingHouseURL, "/")
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/transfers", bytes.NewReader(body))
	if err != nil {
		return clearingHouseIssueResult{Success: false, FailureReason: err.Error()}
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Idempotency-Key", fmt.Sprintf("transfer-%d", event.TransferID))
	req.Header.Set("Authorization", "Bearer "+s.cfg.ClearingHouseAPIToken)

	resp, err := s.http.Do(req)
	if err != nil {
		return clearingHouseIssueResult{Success: false, FailureReason: err.Error(), retryableFailure: true}
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return clearingHouseIssueResult{Success: false, FailureReason: fmt.Sprintf("HTTP %d", resp.StatusCode), retryableFailure: resp.StatusCode >= 500}
	}
	var out clearingHouseIssueResult
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return clearingHouseIssueResult{Success: false, FailureReason: err.Error(), retryableFailure: true}
	}
	return out
}

func (s *ExternalTransferService) externalTransfersForEscalation(ctx context.Context, tx *sql.Tx) ([]externalTransferRow, error) {
	return s.queryExternalTransfers(ctx, tx, `
SELECT id, retry_count, amount, recipient_account, currency
  FROM external_transfers
 WHERE status = 'PENDING'
   AND retry_count >= $1
   AND created_at < now() - INTERVAL '72 hours'
`, s.maxAttempts())
}

func (s *ExternalTransferService) externalTransfersForRetry(ctx context.Context, tx *sql.Tx) ([]externalTransferRow, error) {
	return s.queryExternalTransfers(ctx, tx, `
SELECT id, retry_count, amount, recipient_account, currency
  FROM external_transfers
 WHERE status = 'PENDING'
   AND retry_count < $1
   AND created_at < now() - INTERVAL '72 hours'
`, s.maxAttempts())
}

func (s *ExternalTransferService) queryExternalTransfers(ctx context.Context, tx *sql.Tx, query string, args ...any) ([]externalTransferRow, error) {
	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []externalTransferRow{}
	for rows.Next() {
		var item externalTransferRow
		if err := rows.Scan(&item.ID, &item.RetryCount, &item.Amount, &item.RecipientAccount, &item.Currency); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *ExternalTransferService) maxAttempts() int {
	if s.cfg.TransferRetryMaxAttempts <= 0 {
		return 3
	}
	return s.cfg.TransferRetryMaxAttempts
}

func nextExternalTransferRetryRun(now time.Time) time.Time {
	year, month, day := now.Date()
	location := now.Location()
	for _, hour := range []int{2, 8, 14, 20} {
		candidate := time.Date(year, month, day, hour, 0, 0, 0, location)
		if !candidate.Before(now) {
			return candidate
		}
	}
	return time.Date(year, month, day+1, 2, 0, 0, 0, location)
}
