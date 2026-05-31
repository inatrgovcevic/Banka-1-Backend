package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"banka1/banking-core-service-go/internal/config"
	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	gdprClientSoftDeletedRoutingKey = "gdpr.client.soft-deleted"
	gdprBankingCoreListener         = "banking-core"
)

type GdprService struct {
	db  *sql.DB
	cfg config.Config
}

type GdprClientDeletedEvent struct {
	ClientID int64  `json:"clientId"`
	EventID  string `json:"eventId"`
}

type GdprCascadeSummary struct {
	Accounts int64 `json:"accounts"`
	Cards    int64 `json:"cards"`
	Skipped  bool  `json:"skipped"`
}

func NewGdprService(db *sql.DB, cfg config.Config) *GdprService {
	return &GdprService{db: db, cfg: cfg}
}

func (s *GdprService) Start(ctx context.Context) {
	go s.consumeClientSoftDeleted(ctx)
}

func (s *GdprService) consumeClientSoftDeleted(ctx context.Context) {
	s.consumeRabbitLoop(ctx, func(ctx context.Context, body []byte) error {
		var event GdprClientDeletedEvent
		if err := json.Unmarshal(body, &event); err != nil {
			return err
		}
		summary, err := s.OnClientSoftDeleted(ctx, event)
		if err != nil {
			return err
		}
		log.Printf("gdpr cascade finished: clientId=%d event=%s accounts=%d cards=%d skipped=%t",
			event.ClientID, event.EventID, summary.Accounts, summary.Cards, summary.Skipped)
		return nil
	})
}

func (s *GdprService) OnClientSoftDeleted(ctx context.Context, event GdprClientDeletedEvent) (GdprCascadeSummary, error) {
	event.EventID = strings.TrimSpace(event.EventID)
	if event.EventID == "" {
		return GdprCascadeSummary{}, fmt.Errorf("gdpr eventId is required")
	}
	if event.ClientID == 0 {
		return GdprCascadeSummary{}, fmt.Errorf("gdpr clientId is required")
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return GdprCascadeSummary{}, err
	}
	defer tx.Rollback()

	var alreadyProcessed int
	if err := tx.QueryRowContext(ctx, `
SELECT COUNT(*)
  FROM gdpr_event_log
 WHERE event_id = $1 AND listener = $2
`, event.EventID, gdprBankingCoreListener).Scan(&alreadyProcessed); err != nil {
		return GdprCascadeSummary{}, err
	}
	if alreadyProcessed > 0 {
		if err := tx.Commit(); err != nil {
			return GdprCascadeSummary{}, err
		}
		return GdprCascadeSummary{Skipped: true}, nil
	}

	accountResult, err := tx.ExecContext(ctx, `
UPDATE account_table
   SET deleted = true,
       deleted_due_to_client_id = $1,
       updated_at = now()
 WHERE vlasnik = $2
   AND deleted = false
`, event.ClientID, event.ClientID)
	if err != nil {
		return GdprCascadeSummary{}, err
	}
	accountsUpdated, err := accountResult.RowsAffected()
	if err != nil {
		return GdprCascadeSummary{}, err
	}

	cardResult, err := tx.ExecContext(ctx, `
UPDATE cards
   SET deleted = true,
       deleted_due_to_client_id = $1,
       updated_at = now()
 WHERE account_number IN (
       SELECT broj_racuna
         FROM account_table
        WHERE vlasnik = $2
 )
   AND deleted = false
`, event.ClientID, event.ClientID)
	if err != nil {
		return GdprCascadeSummary{}, err
	}
	cardsUpdated, err := cardResult.RowsAffected()
	if err != nil {
		return GdprCascadeSummary{}, err
	}

	summaryText := fmt.Sprintf("accounts=%d, cards=%d", accountsUpdated, cardsUpdated)
	if _, err := tx.ExecContext(ctx, `
INSERT INTO gdpr_event_log (event_id, listener, processed_at, summary)
VALUES ($1, $2, now(), $3)
`, event.EventID, gdprBankingCoreListener, summaryText); err != nil {
		return GdprCascadeSummary{}, err
	}
	if err := tx.Commit(); err != nil {
		return GdprCascadeSummary{}, err
	}
	return GdprCascadeSummary{Accounts: accountsUpdated, Cards: cardsUpdated}, nil
}

func (s *GdprService) consumeRabbitLoop(ctx context.Context, handler func(context.Context, []byte) error) {
	for {
		if err := s.consumeRabbit(ctx, handler); err != nil && ctx.Err() == nil {
			log.Printf("rabbit consumer %s failed: %v", gdprClientSoftDeletedRoutingKey, err)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(30 * time.Second):
		}
	}
}

func (s *GdprService) consumeRabbit(ctx context.Context, handler func(context.Context, []byte) error) error {
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

	if err := ch.ExchangeDeclare(s.cfg.GdprExchange, "topic", true, false, false, false, nil); err != nil {
		return err
	}
	queue, err := ch.QueueDeclare(s.cfg.GdprBankingCoreQueue, true, false, false, false, nil)
	if err != nil {
		return err
	}
	if err := ch.QueueBind(queue.Name, gdprClientSoftDeletedRoutingKey, s.cfg.GdprExchange, false, nil); err != nil {
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
				return fmt.Errorf("rabbit deliveries closed for %s", gdprClientSoftDeletedRoutingKey)
			}
			if err := handler(ctx, delivery.Body); err != nil {
				_ = delivery.Nack(false, true)
				log.Printf("rabbit handler %s failed: %v", gdprClientSoftDeletedRoutingKey, err)
				continue
			}
			_ = delivery.Ack(false)
		}
	}
}
