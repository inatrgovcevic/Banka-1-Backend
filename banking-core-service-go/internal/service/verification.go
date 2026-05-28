package service

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"math/big"
	"regexp"
	"strings"
	"time"

	"banka1/banking-core-service-go/internal/config"
)

var otpCodePattern = regexp.MustCompile(`^\d{6}$`)

type VerificationService struct {
	db     *sql.DB
	cfg    config.Config
	rabbit *RabbitPublisher
}

func NewVerificationService(db *sql.DB, cfg config.Config, rabbit *RabbitPublisher) *VerificationService {
	return &VerificationService{db: db, cfg: cfg, rabbit: rabbit}
}

type VerificationGenerateRequest struct {
	ClientID        int64  `json:"clientId"`
	OperationType   string `json:"operationType"`
	RelatedEntityID string `json:"relatedEntityId"`
	ClientEmail     string `json:"clientEmail"`
}

type VerificationGenerateResponse struct {
	SessionID int64 `json:"sessionId"`
}

type VerificationValidateRequest struct {
	SessionID int64  `json:"sessionId"`
	Code      string `json:"code"`
}

type VerificationValidateResponse struct {
	Valid             bool   `json:"valid"`
	Status            string `json:"status"`
	RemainingAttempts int64  `json:"remainingAttempts"`
}

type VerificationStatusResponse struct {
	SessionID int64  `json:"sessionId"`
	Status    string `json:"status"`
}

type verificationSessionRow struct {
	ID              int64
	ClientID        int64
	Code            string
	OperationType   string
	RelatedEntityID string
	CreatedAt       time.Time
	ExpiresAt       time.Time
	AttemptCount    int64
	Status          string
}

func (s *VerificationService) Generate(ctx context.Context, principal Principal, req VerificationGenerateRequest) (VerificationGenerateResponse, error) {
	if req.ClientID == 0 {
		return VerificationGenerateResponse{}, BadRequest("clientId is required.")
	}
	if principal.ID != req.ClientID {
		return VerificationGenerateResponse{}, verificationError(403, "ERR_FORBIDDEN", "Pristup odbijen", "Cannot generate verification for other client")
	}
	req.OperationType = strings.ToUpper(strings.TrimSpace(req.OperationType))
	req.RelatedEntityID = strings.TrimSpace(req.RelatedEntityID)
	req.ClientEmail = strings.TrimSpace(req.ClientEmail)
	if !validOperationType(req.OperationType) {
		return VerificationGenerateResponse{}, BadRequest("operationType is required.")
	}
	if req.RelatedEntityID == "" {
		return VerificationGenerateResponse{}, BadRequest("relatedEntityId is required.")
	}
	if req.ClientEmail == "" || !strings.Contains(req.ClientEmail, "@") {
		return VerificationGenerateResponse{}, BadRequest("clientEmail must be a valid email address.")
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return VerificationGenerateResponse{}, err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `
UPDATE verification_sessions
   SET status = 'CANCELLED'
 WHERE client_id = $1
   AND operation_type = $2
   AND related_entity_id = $3
   AND status = 'PENDING'
`, req.ClientID, req.OperationType, req.RelatedEntityID); err != nil {
		return VerificationGenerateResponse{}, err
	}

	rawCode, err := generateOTPCode()
	if err != nil {
		return VerificationGenerateResponse{}, err
	}
	hash, err := s.hashOTP(rawCode)
	if err != nil {
		return VerificationGenerateResponse{}, err
	}
	now := time.Now()
	expiresAt := now.Add(time.Duration(s.ttlMinutes()) * time.Minute)
	var id int64
	err = tx.QueryRowContext(ctx, `
INSERT INTO verification_sessions (
    client_id, code, operation_type, related_entity_id, created_at, expires_at,
    attempt_count, status
) VALUES (
    $1, $2, $3, $4, $5, $6, 0, 'PENDING'
)
RETURNING id
`, req.ClientID, hash, req.OperationType, req.RelatedEntityID, now, expiresAt).Scan(&id)
	if err != nil {
		if looksUniqueViolation(err) {
			return VerificationGenerateResponse{}, verificationError(
				409,
				"ERR_VERIFICATION_006",
				"Aktivna verifikaciona sesija vec postoji",
				"Client ID: %d, operationType: %s, relatedEntityId: %s",
				req.ClientID,
				req.OperationType,
				req.RelatedEntityID,
			)
		}
		return VerificationGenerateResponse{}, err
	}
	if err := tx.Commit(); err != nil {
		return VerificationGenerateResponse{}, err
	}
	s.publishGeneratedEvent(ctx, req, rawCode, id)
	return VerificationGenerateResponse{SessionID: id}, nil
}

func (s *VerificationService) Validate(ctx context.Context, req VerificationValidateRequest) (VerificationValidateResponse, error) {
	if req.SessionID == 0 {
		return VerificationValidateResponse{}, BadRequest("sessionId is required.")
	}
	if !otpCodePattern.MatchString(req.Code) {
		return VerificationValidateResponse{}, BadRequest("code must be a 6-digit number.")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return VerificationValidateResponse{}, err
	}
	defer tx.Rollback()

	session, err := s.findSession(ctx, tx, req.SessionID, true)
	if err != nil {
		return VerificationValidateResponse{}, err
	}
	if session.Status == "CANCELLED" {
		return VerificationValidateResponse{}, verificationError(400, "ERR_VERIFICATION_002", "Sesija verifikacije je otkazana", "Session ID: %d", req.SessionID)
	}
	if session.Status == "VERIFIED" {
		return VerificationValidateResponse{}, verificationError(400, "ERR_VERIFICATION_003", "Sesija verifikacije je vec verifikovana", "Session ID: %d", req.SessionID)
	}
	if session.Status == "EXPIRED" {
		return VerificationValidateResponse{}, verificationError(400, "ERR_VERIFICATION_004", "Verifikacioni kod je istekao", "Session ID: %d", req.SessionID)
	}
	if time.Now().After(session.ExpiresAt) {
		if _, err := tx.ExecContext(ctx, "UPDATE verification_sessions SET status = 'EXPIRED' WHERE id = $1", session.ID); err != nil {
			return VerificationValidateResponse{}, err
		}
		if err := tx.Commit(); err != nil {
			return VerificationValidateResponse{}, err
		}
		return VerificationValidateResponse{}, verificationError(400, "ERR_VERIFICATION_004", "Verifikacioni kod je istekao", "Session ID: %d", req.SessionID)
	}

	matches, err := s.matchesOTP(req.Code, session.Code)
	if err != nil {
		return VerificationValidateResponse{}, err
	}
	if matches {
		if _, err := tx.ExecContext(ctx, "UPDATE verification_sessions SET status = 'VERIFIED' WHERE id = $1", session.ID); err != nil {
			return VerificationValidateResponse{}, err
		}
		if err := tx.Commit(); err != nil {
			return VerificationValidateResponse{}, err
		}
		return VerificationValidateResponse{Valid: true, Status: "VERIFIED", RemainingAttempts: 0}, nil
	}

	session.AttemptCount++
	status := "PENDING"
	if session.AttemptCount >= s.maxAttempts() {
		status = "CANCELLED"
	}
	if _, err := tx.ExecContext(ctx, "UPDATE verification_sessions SET attempt_count = $1, status = $2 WHERE id = $3", session.AttemptCount, status, session.ID); err != nil {
		return VerificationValidateResponse{}, err
	}
	if err := tx.Commit(); err != nil {
		return VerificationValidateResponse{}, err
	}
	remaining := s.maxAttempts() - session.AttemptCount
	if remaining < 0 {
		remaining = 0
	}
	return VerificationValidateResponse{Valid: false, Status: status, RemainingAttempts: remaining}, nil
}

func (s *VerificationService) Status(ctx context.Context, sessionID int64) (VerificationStatusResponse, error) {
	session, err := s.findSession(ctx, s.db, sessionID, false)
	if err != nil {
		return VerificationStatusResponse{}, err
	}
	if session.Status == "PENDING" && time.Now().After(session.ExpiresAt) {
		if _, err := s.db.ExecContext(ctx, "UPDATE verification_sessions SET status = 'EXPIRED' WHERE id = $1", session.ID); err != nil {
			return VerificationStatusResponse{}, err
		}
		session.Status = "EXPIRED"
	}
	return VerificationStatusResponse{SessionID: session.ID, Status: session.Status}, nil
}

func (s *VerificationService) findSession(ctx context.Context, runner sqlRunner, sessionID int64, forUpdate bool) (verificationSessionRow, error) {
	query := verificationSelectSQL + " WHERE id = $1"
	if forUpdate {
		query += " FOR UPDATE"
	}
	var out verificationSessionRow
	err := runner.QueryRowContext(ctx, query, sessionID).Scan(
		&out.ID,
		&out.ClientID,
		&out.Code,
		&out.OperationType,
		&out.RelatedEntityID,
		&out.CreatedAt,
		&out.ExpiresAt,
		&out.AttemptCount,
		&out.Status,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return verificationSessionRow{}, verificationError(404, "ERR_VERIFICATION_001", "Sesija verifikacije nije pronadjena", "Session ID: %d", sessionID)
		}
		return verificationSessionRow{}, err
	}
	return out, nil
}

func (s *VerificationService) hashOTP(rawCode string) (string, error) {
	if strings.TrimSpace(s.cfg.JWTSecret) == "" {
		return "", Internal("JWT_SECRET is not configured")
	}
	mac := hmac.New(sha256.New, []byte(s.cfg.JWTSecret))
	if _, err := mac.Write([]byte(rawCode)); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(mac.Sum(nil)), nil
}

func (s *VerificationService) matchesOTP(rawCode, expectedHash string) (bool, error) {
	hash, err := s.hashOTP(rawCode)
	if err != nil {
		return false, err
	}
	return hmac.Equal([]byte(hash), []byte(expectedHash)), nil
}

func (s *VerificationService) ttlMinutes() int64 {
	if s.cfg.VerificationTTLMinutes <= 0 {
		return 5
	}
	return s.cfg.VerificationTTLMinutes
}

func (s *VerificationService) maxAttempts() int64 {
	if s.cfg.VerificationMaxAttempts <= 0 {
		return 3
	}
	return s.cfg.VerificationMaxAttempts
}

func generateOTPCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(900000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()+100000), nil
}

func validOperationType(value string) bool {
	switch value {
	case "PAYMENT", "TRANSFER", "LIMIT_CHANGE", "CARD_REQUEST", "LOAN_REQUEST":
		return true
	default:
		return false
	}
}

func looksUniqueViolation(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unique") || strings.Contains(msg, "duplicate") || strings.Contains(msg, "23505")
}

func verificationError(status int, code, title, format string, args ...any) *Error {
	return &Error{Status: status, Code: code, Title: title, Message: fmt.Sprintf(format, args...)}
}

const verificationSelectSQL = `
SELECT id,
       client_id,
       code,
       operation_type,
       related_entity_id,
       created_at,
       expires_at,
       attempt_count,
       status
  FROM verification_sessions
`
