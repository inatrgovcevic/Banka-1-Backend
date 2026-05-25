package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Direction values for interbank_messages.direction column.
const (
	DirectionInbound  = "INBOUND"
	DirectionOutbound = "OUTBOUND"
)

// MessageStatus values for interbank_messages.status column.
const (
	MessageStatusProcessed   = "PROCESSED"    // inbound: handled successfully
	MessageStatusError       = "ERROR"        // inbound: handled with error response (cached for replay)
	MessageStatusPendingSend = "PENDING_SEND" // outbound: not yet sent or in retry
	MessageStatusSent        = "SENT"         // outbound: partner accepted
	MessageStatusStuck       = "STUCK"        // outbound: max retries exhausted
)

// Message is one row in interbank_messages. Mirrors the Java InterbankMessageEntity.
// Note: request_body and response_body are TEXT columns (not BYTEA), so they are
// stored as strings — callers must serialize/deserialize JSON themselves.
type Message struct {
	ID                   int64
	Direction            string
	SenderRoutingNumber  int
	LocallyGeneratedKey  string
	MessageType          string
	Status               string
	RequestBody          string  // TEXT column — raw JSON string
	ResponseBody         *string // TEXT column, nullable
	HttpStatus           *int    // nullable — not set until a response is received
	RetryCount           int
	TransactionIdRouting *int    // nullable — only set when the message references a transaction
	TransactionIdLocal   *string // nullable
	CreatedAt            time.Time
	LastAttemptAt        *time.Time
	Version              int64
}

// MessageStore persists interbank_messages rows.
type MessageStore struct {
	pool *pgxpool.Pool
}

func NewMessageStore(pool *pgxpool.Pool) *MessageStore { return &MessageStore{pool: pool} }

// Insert writes a new row, populating m.ID and m.CreatedAt. Caller must set
// every other field before calling. Returns a pgconn.PgError with Code "23505"
// (check with IsUniqueViolation) if the (direction, sender_routing_number,
// locally_generated_key) tuple already exists.
func (s *MessageStore) Insert(ctx context.Context, m *Message) error {
	err := s.pool.QueryRow(ctx, `
		INSERT INTO interbank_messages
			(direction, sender_routing_number, locally_generated_key, message_type, status,
			 request_body, response_body, http_status, retry_count,
			 transaction_id_routing, transaction_id_local, last_attempt_at, version)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,0)
		RETURNING id, created_at`,
		m.Direction, m.SenderRoutingNumber, m.LocallyGeneratedKey,
		m.MessageType, m.Status, m.RequestBody, m.ResponseBody, m.HttpStatus, m.RetryCount,
		m.TransactionIdRouting, m.TransactionIdLocal, m.LastAttemptAt,
	).Scan(&m.ID, &m.CreatedAt)
	if err != nil {
		return err
	}
	m.Version = 0
	return nil
}

// Lookup finds the cached row by composite key. Returns (nil, nil) when not found.
func (s *MessageStore) Lookup(ctx context.Context, direction string, senderRouting int, key string) (*Message, error) {
	var m Message
	err := s.pool.QueryRow(ctx, `
		SELECT id, direction, sender_routing_number, locally_generated_key, message_type, status,
		       request_body, response_body, http_status, retry_count,
		       transaction_id_routing, transaction_id_local, created_at, last_attempt_at, version
		FROM interbank_messages
		WHERE direction = $1 AND sender_routing_number = $2 AND locally_generated_key = $3`,
		direction, senderRouting, key,
	).Scan(&m.ID, &m.Direction, &m.SenderRoutingNumber, &m.LocallyGeneratedKey, &m.MessageType, &m.Status,
		&m.RequestBody, &m.ResponseBody, &m.HttpStatus, &m.RetryCount,
		&m.TransactionIdRouting, &m.TransactionIdLocal, &m.CreatedAt, &m.LastAttemptAt, &m.Version)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &m, nil
}

// IsUniqueViolation reports whether err is a Postgres 23505 unique_violation —
// useful for Insert callers that want to handle idempotency conflicts.
func IsUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

// FindPending returns OUTBOUND messages eligible for retry: status=PENDING_SEND,
// retry_count below maxRetries, and either never attempted or last attempted
// before cutoff. Used by the retry scheduler.
func (s *MessageStore) FindPending(ctx context.Context, maxRetries int, cutoff time.Time, limit int) ([]Message, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, direction, sender_routing_number, locally_generated_key, message_type, status,
		       request_body, response_body, http_status, retry_count,
		       transaction_id_routing, transaction_id_local, created_at, last_attempt_at, version
		FROM interbank_messages
		WHERE direction = 'OUTBOUND'
		  AND status = 'PENDING_SEND'
		  AND retry_count < $1
		  AND (last_attempt_at IS NULL OR last_attempt_at < $2)
		ORDER BY last_attempt_at ASC NULLS FIRST
		LIMIT $3`,
		maxRetries, cutoff, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Message
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.ID, &m.Direction, &m.SenderRoutingNumber, &m.LocallyGeneratedKey,
			&m.MessageType, &m.Status, &m.RequestBody, &m.ResponseBody, &m.HttpStatus, &m.RetryCount,
			&m.TransactionIdRouting, &m.TransactionIdLocal, &m.CreatedAt, &m.LastAttemptAt, &m.Version); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// UpdateOptimistic applies @Version-style optimistic concurrency. The UPDATE
// predicate includes the original version; if zero rows affected, returns
// ErrOptimisticLockConflict. On success, m.Version is incremented.
func (s *MessageStore) UpdateOptimistic(ctx context.Context, m *Message) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE interbank_messages
		SET status = $1, response_body = $2, http_status = $3, retry_count = $4,
		    last_attempt_at = $5, version = version + 1
		WHERE id = $6 AND version = $7`,
		m.Status, m.ResponseBody, m.HttpStatus, m.RetryCount, m.LastAttemptAt, m.ID, m.Version)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrOptimisticLockConflict
	}
	m.Version++
	return nil
}
