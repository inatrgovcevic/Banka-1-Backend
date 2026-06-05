package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/raf-si-2025/banka-1-go/interbank-service/internal/protocol"
	"github.com/raf-si-2025/banka-1-go/interbank-service/internal/store"
	"github.com/raf-si-2025/banka-1-go/shared/auth"
)

// ErrPartnerNotFound is returned when no partner matches the requested routing number.
var ErrPartnerNotFound = errors.New("outbound: partner not found")

// MessageStoreSeam is the subset of *store.MessageStore used by InterbankClient.
// Defined as an interface so tests can inject a fake without Postgres.
type MessageStoreSeam interface {
	Insert(ctx context.Context, m *store.Message) error
	UpdateOptimistic(ctx context.Context, m *store.Message) error
	FindPending(ctx context.Context, maxRetries int, cutoff time.Time, limit int) ([]store.Message, error)
}

// ErrOutboundAuth is returned when the partner responds with 401 (token misconfigured).
var ErrOutboundAuth = errors.New("outbound: partner rejected our auth token")

// ErrOutboundNotFound is returned when the partner responds with 404.
var ErrOutboundNotFound = errors.New("outbound: partner resource not found")

// ErrOutboundConflict is returned when the partner responds with 409.
var ErrOutboundConflict = errors.New("outbound: partner conflict (409)")

// ErrOutboundUpstream is the fallback for 4xx/5xx responses not covered above.
var ErrOutboundUpstream = errors.New("outbound: partner upstream error")

// PartnerLookup resolves an outbound routing number to a partner config.
// Implementations are typically thin wrappers around an auth.PartnerStore.
type PartnerLookup interface {
	FindByRouting(routing int) (*auth.Partner, error)
}

// partnerStoreAdapter wraps auth.PartnerStore to satisfy PartnerLookup.
type partnerStoreAdapter struct {
	ps auth.PartnerStore
}

// NewPartnerLookup returns a PartnerLookup backed by ps.
func NewPartnerLookup(ps auth.PartnerStore) PartnerLookup {
	return &partnerStoreAdapter{ps: ps}
}

func (a *partnerStoreAdapter) FindByRouting(routing int) (*auth.Partner, error) {
	for _, p := range a.ps.Partners() {
		if p.Routing == routing {
			cp := p
			return &cp, nil
		}
	}
	return nil, fmt.Errorf("%w: routing=%d", ErrPartnerNotFound, routing)
}

// ---------------------------------------------------------------------------
// InterbankClient — concrete outbound HTTP client
// ---------------------------------------------------------------------------

// InterbankClient sends inter-bank protocol messages to partner banks.
// It satisfies the OutboundClient interface consumed by Coordinator.
//
// Persistence: every 2PC call (SendNewTx/SendCommitTx/SendRollbackTx) persists
// the message in interbank_messages with status PENDING_SEND before attempting
// the HTTP call. On success the row is transitioned to SENT. On failure the row
// stays in PENDING_SEND and the retry scheduler will re-send it using the same
// idempotency key (Tim 2 §2.2).
//
// OTC §3 outbound methods (OutboundCreateNegotiation, etc.) do NOT persist to
// interbank_messages — those are for 2PC only (as per Java InterbankClient comment).
type InterbankClient struct {
	myRouting int
	partners  PartnerLookup
	msgStore  MessageStoreSeam
	hc        *http.Client
	log       *slog.Logger
}

// NewInterbankClient constructs the client with a real *store.MessageStore.
// hc may be nil (uses a 60s-timeout default).
func NewInterbankClient(
	myRouting int,
	partners PartnerLookup,
	msgStore *store.MessageStore,
	hc *http.Client,
	log *slog.Logger,
) *InterbankClient {
	return NewInterbankClientWithStore(myRouting, partners, msgStore, hc, log)
}

// NewInterbankClientWithStore constructs the client with an injected MessageStoreSeam.
// Use this constructor in tests to avoid a Postgres dependency.
func NewInterbankClientWithStore(
	myRouting int,
	partners PartnerLookup,
	msgStore MessageStoreSeam,
	hc *http.Client,
	log *slog.Logger,
) *InterbankClient {
	if hc == nil {
		hc = &http.Client{Timeout: 60 * time.Second}
	}
	if log == nil {
		log = slog.Default()
	}
	return &InterbankClient{
		myRouting: myRouting,
		partners:  partners,
		msgStore:  msgStore,
		hc:        hc,
		log:       log,
	}
}

// ---------------------------------------------------------------------------
// OutboundClient interface — 2PC methods
// ---------------------------------------------------------------------------

// SendNewTx sends a NEW_TX message to partnerRouting and returns their vote.
func (c *InterbankClient) SendNewTx(ctx context.Context, partnerRouting int, tx protocol.InterbankTransactionPayload) (protocol.TransactionVote, error) {
	partner, err := c.partners.FindByRouting(partnerRouting)
	if err != nil {
		return protocol.TransactionVote{}, err
	}

	key := newIdempotencyKey()
	payload := protocol.InterbankMessagePayload{
		IdempotenceKey: protocol.IdempotenceKey{
			RoutingNumber:       c.myRouting,
			LocallyGeneratedKey: key,
		},
		MessageType: protocol.MessageTypeNewTx,
		Message:     &tx,
	}

	msg, err := c.persistPending(ctx, partnerRouting, key, string(protocol.MessageTypeNewTx), payload)
	if err != nil {
		return protocol.TransactionVote{}, fmt.Errorf("outbound: persist pending: %w", err)
	}

	respBody, httpStatus, err := c.postInterbank(ctx, partner, payload)
	if err != nil {
		c.log.WarnContext(ctx, "outbound SendNewTx failed; message stays PENDING_SEND for retry",
			"partnerRouting", partnerRouting, "key", key, "err", err)
		return protocol.TransactionVote{}, err
	}

	var vote protocol.TransactionVote
	if unmarshalErr := json.Unmarshal(respBody, &vote); unmarshalErr != nil {
		return protocol.TransactionVote{}, fmt.Errorf("outbound: decode TransactionVote: %w", unmarshalErr)
	}

	if updateErr := c.markSent(ctx, msg, httpStatus, string(respBody)); updateErr != nil {
		c.log.WarnContext(ctx, "outbound: markSent failed (non-fatal)", "key", key, "err", updateErr)
	}
	return vote, nil
}

// SendCommitTx sends a COMMIT_TX message. Returns nil on 204.
func (c *InterbankClient) SendCommitTx(ctx context.Context, partnerRouting int, txID protocol.ForeignBankId) error {
	partner, err := c.partners.FindByRouting(partnerRouting)
	if err != nil {
		return err
	}

	key := newIdempotencyKey()
	body := protocol.CommitTransactionBody{TransactionId: txID}
	payload := protocol.InterbankMessagePayload{
		IdempotenceKey: protocol.IdempotenceKey{
			RoutingNumber:       c.myRouting,
			LocallyGeneratedKey: key,
		},
		MessageType: protocol.MessageTypeCommitTx,
		Message:     &body,
	}

	msg, err := c.persistPending(ctx, partnerRouting, key, string(protocol.MessageTypeCommitTx), payload)
	if err != nil {
		return fmt.Errorf("outbound: persist pending: %w", err)
	}

	_, httpStatus, err := c.postInterbank(ctx, partner, payload)
	if err != nil {
		c.log.WarnContext(ctx, "outbound SendCommitTx failed; message stays PENDING_SEND for retry",
			"partnerRouting", partnerRouting, "key", key, "err", err)
		return err
	}

	if updateErr := c.markSent(ctx, msg, httpStatus, ""); updateErr != nil {
		c.log.WarnContext(ctx, "outbound: markSent failed (non-fatal)", "key", key, "err", updateErr)
	}
	return nil
}

// SendRollbackTx sends a ROLLBACK_TX message. Returns nil on 204.
func (c *InterbankClient) SendRollbackTx(ctx context.Context, partnerRouting int, txID protocol.ForeignBankId) error {
	partner, err := c.partners.FindByRouting(partnerRouting)
	if err != nil {
		return err
	}

	key := newIdempotencyKey()
	body := protocol.RollbackTransactionBody{TransactionId: txID}
	payload := protocol.InterbankMessagePayload{
		IdempotenceKey: protocol.IdempotenceKey{
			RoutingNumber:       c.myRouting,
			LocallyGeneratedKey: key,
		},
		MessageType: protocol.MessageTypeRollbackTx,
		Message:     &body,
	}

	msg, err := c.persistPending(ctx, partnerRouting, key, string(protocol.MessageTypeRollbackTx), payload)
	if err != nil {
		return fmt.Errorf("outbound: persist pending: %w", err)
	}

	_, httpStatus, err := c.postInterbank(ctx, partner, payload)
	if err != nil {
		c.log.WarnContext(ctx, "outbound SendRollbackTx failed; message stays PENDING_SEND for retry",
			"partnerRouting", partnerRouting, "key", key, "err", err)
		return err
	}

	if updateErr := c.markSent(ctx, msg, httpStatus, ""); updateErr != nil {
		c.log.WarnContext(ctx, "outbound: markSent failed (non-fatal)", "key", key, "err", updateErr)
	}
	return nil
}

// ---------------------------------------------------------------------------
// OTC §3 outbound methods (no message persistence — §3 is not 2PC)
// ---------------------------------------------------------------------------

// OutboundCreateNegotiation sends POST /negotiations to the partner bank.
// Returns the ForeignBankId assigned by the partner.
func (c *InterbankClient) OutboundCreateNegotiation(ctx context.Context, partnerRouting int, offer OtcOfferDto) (*protocol.ForeignBankId, error) {
	partner, err := c.partners.FindByRouting(partnerRouting)
	if err != nil {
		return nil, err
	}

	bodyBytes, err := json.Marshal(offer)
	if err != nil {
		return nil, fmt.Errorf("outbound: marshal offer: %w", err)
	}

	resp, err := c.doRequest(ctx, http.MethodPost, joinURL(partner.BaseURL, "negotiations"), partner.OutboundToken, bodyBytes)
	if err != nil {
		return nil, err
	}

	var id protocol.ForeignBankId
	if unmarshalErr := json.Unmarshal(resp, &id); unmarshalErr != nil {
		return nil, fmt.Errorf("outbound: decode ForeignBankId: %w", unmarshalErr)
	}
	c.log.InfoContext(ctx, "outbound POST /negotiations",
		"partnerRouting", partnerRouting, "id", id)
	return &id, nil
}

// OutboundPutCounter sends PUT /negotiations/{rn}/{id} to the partner bank.
// Returns the HTTP status code so the FE wrapper can propagate 204/400/409.
func (c *InterbankClient) OutboundPutCounter(ctx context.Context, partnerRouting int, negID protocol.ForeignBankId, offer OtcOfferDto) (int, error) {
	partner, err := c.partners.FindByRouting(partnerRouting)
	if err != nil {
		return 0, err
	}

	bodyBytes, err := json.Marshal(offer)
	if err != nil {
		return 0, fmt.Errorf("outbound: marshal offer: %w", err)
	}

	u := joinURL(partner.BaseURL, fmt.Sprintf("negotiations/%d/%s", negID.RoutingNumber, negID.Id))
	status, _, doErr := c.doRequestFull(ctx, http.MethodPut, u, partner.OutboundToken, bodyBytes)
	if doErr != nil {
		// Unwrap typed errors to let caller decide.
		return status, doErr
	}
	return status, nil
}

// OutboundAccept sends GET /negotiations/{rn}/{id}/accept to the partner bank.
// The partner runs 2PC; this call may block up to 60 seconds per Tim 2 §6.6.
func (c *InterbankClient) OutboundAccept(ctx context.Context, partnerRouting int, negID protocol.ForeignBankId) (int, error) {
	partner, err := c.partners.FindByRouting(partnerRouting)
	if err != nil {
		return 0, err
	}

	u := joinURL(partner.BaseURL, fmt.Sprintf("negotiations/%d/%s/accept", negID.RoutingNumber, negID.Id))
	status, _, doErr := c.doRequestFull(ctx, http.MethodGet, u, partner.OutboundToken, nil)
	if doErr != nil {
		return status, doErr
	}
	return status, nil
}

// OutboundDelete sends DELETE /negotiations/{rn}/{id} to the partner bank.
// Per Tim 2 MINOR-3, a 404 response is treated as idempotent (already deleted).
func (c *InterbankClient) OutboundDelete(ctx context.Context, partnerRouting int, negID protocol.ForeignBankId) error {
	partner, err := c.partners.FindByRouting(partnerRouting)
	if err != nil {
		return err
	}

	u := joinURL(partner.BaseURL, fmt.Sprintf("negotiations/%d/%s", negID.RoutingNumber, negID.Id))
	status, _, doErr := c.doRequestFull(ctx, http.MethodDelete, u, partner.OutboundToken, nil)
	if doErr != nil {
		// 404 → idempotent no-op per Tim 2 MINOR-3.
		if errors.Is(doErr, ErrOutboundNotFound) {
			c.log.InfoContext(ctx, "outbound DELETE /negotiations partner returned 404 — treating as idempotent",
				"partnerRouting", partnerRouting, "negID", negID)
			return nil
		}
		return doErr
	}
	_ = status
	return nil
}

// OutboundFetchPublicStock sends GET /public-stock to the partner bank.
// On 401 it returns ErrOutboundAuth; on other 4xx/5xx it logs a warning and
// returns an empty slice (graceful degradation — UI table shows empty).
func (c *InterbankClient) OutboundFetchPublicStock(ctx context.Context, partnerRouting int) ([]protocol.PublicStockEntry, error) {
	partner, err := c.partners.FindByRouting(partnerRouting)
	if err != nil {
		return nil, err
	}

	u := joinURL(partner.BaseURL, "public-stock")
	status, body, doErr := c.doRequestFull(ctx, http.MethodGet, u, partner.OutboundToken, nil)
	if doErr != nil {
		if errors.Is(doErr, ErrOutboundAuth) {
			c.log.ErrorContext(ctx, "outbound GET /public-stock returned 401 — outbound token misconfigured",
				"partnerRouting", partnerRouting)
			return nil, doErr
		}
		c.log.WarnContext(ctx, "outbound GET /public-stock failed (graceful empty)",
			"partnerRouting", partnerRouting, "status", status, "err", doErr)
		return []protocol.PublicStockEntry{}, nil
	}

	var entries []protocol.PublicStockEntry
	if unmarshalErr := json.Unmarshal(body, &entries); unmarshalErr != nil {
		c.log.WarnContext(ctx, "outbound GET /public-stock decode failed (graceful empty)",
			"partnerRouting", partnerRouting, "err", unmarshalErr)
		return []protocol.PublicStockEntry{}, nil
	}
	return entries, nil
}

// ---------------------------------------------------------------------------
// Resend — called by the retry scheduler
// ---------------------------------------------------------------------------

// Resend re-sends an existing PENDING_SEND message using its cached request body
// and the SAME idempotency key (Tim 2 §2.2 — partner deduplicates by key).
//
// On success, Resend sets msg.Status = SENT and msg.HttpStatus but does NOT
// call UpdateOptimistic — the retry scheduler owns that update after Resend
// returns nil. This avoids a double-update version conflict.
func (c *InterbankClient) Resend(ctx context.Context, msg *store.Message) error {
	partner, err := c.partners.FindByRouting(msg.SenderRoutingNumber)
	if err != nil {
		return err
	}

	// Decode the cached payload to determine message type.
	var payload protocol.InterbankMessagePayload
	if unmarshalErr := json.Unmarshal([]byte(msg.RequestBody), &payload); unmarshalErr != nil {
		return fmt.Errorf("outbound: resend: parse cached request body: %w", unmarshalErr)
	}

	respBody, httpStatus, sendErr := c.postInterbank(ctx, partner, payload)
	if sendErr != nil {
		return sendErr
	}

	// Mutate the message in place — scheduler will persist via UpdateOptimistic.
	msg.Status = store.MessageStatusSent
	msg.HttpStatus = &httpStatus
	if payload.MessageType == protocol.MessageTypeNewTx && len(respBody) > 0 {
		respStr := string(respBody)
		msg.ResponseBody = &respStr
	}
	return nil
}

// ---------------------------------------------------------------------------
// Private helpers
// ---------------------------------------------------------------------------

// newIdempotencyKey generates 16 random bytes encoded as 32 hex characters.
func newIdempotencyKey() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// persistPending writes a PENDING_SEND row to interbank_messages.
// senderRoutingNumber stores the PARTNER routing (recipient) so the retry
// scheduler knows where to re-send without parsing request_body.
func (c *InterbankClient) persistPending(
	ctx context.Context,
	partnerRouting int,
	key string,
	msgType string,
	payload protocol.InterbankMessagePayload,
) (*store.Message, error) {
	rawBody, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("outbound: marshal payload: %w", err)
	}

	now := time.Now()
	msg := &store.Message{
		Direction:           store.DirectionOutbound,
		SenderRoutingNumber: partnerRouting,
		LocallyGeneratedKey: key,
		MessageType:         msgType,
		Status:              store.MessageStatusPendingSend,
		RequestBody:         string(rawBody),
		RetryCount:          0,
		LastAttemptAt:       &now,
	}
	if err := c.msgStore.Insert(ctx, msg); err != nil {
		return nil, err
	}
	return msg, nil
}

// markSent transitions a message to SENT with the HTTP response details.
func (c *InterbankClient) markSent(ctx context.Context, msg *store.Message, httpStatus int, responseBody string) error {
	msg.Status = store.MessageStatusSent
	msg.HttpStatus = &httpStatus
	if responseBody != "" {
		msg.ResponseBody = &responseBody
	}
	now := time.Now()
	msg.LastAttemptAt = &now
	return c.msgStore.UpdateOptimistic(ctx, msg)
}

// postInterbank marshals payload as JSON and POSTs to partner.BaseURL + "interbank".
// Returns (responseBody, httpStatus, error).
func (c *InterbankClient) postInterbank(ctx context.Context, partner *auth.Partner, payload protocol.InterbankMessagePayload) ([]byte, int, error) {
	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, 0, fmt.Errorf("outbound: marshal envelope: %w", err)
	}
	u := joinURL(partner.BaseURL, "interbank")
	status, body, doErr := c.doRequestFull(ctx, http.MethodPost, u, partner.OutboundToken, bodyBytes)
	return body, status, doErr
}

// doRequest sends the HTTP request and returns the raw response body.
// Convenience wrapper — returns only the body or error.
func (c *InterbankClient) doRequest(ctx context.Context, method, url, apiKey string, body []byte) ([]byte, error) {
	_, b, err := c.doRequestFull(ctx, method, url, apiKey, body)
	return b, err
}

// doRequestFull sends the HTTP request and returns (status, body, error).
// Maps HTTP errors to typed sentinels.
func (c *InterbankClient) doRequestFull(ctx context.Context, method, rawURL, apiKey string, body []byte) (int, []byte, error) {
	var bodyReader io.Reader
	if body != nil {
		bodyReader = newBytesReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, rawURL, bodyReader)
	if err != nil {
		return 0, nil, fmt.Errorf("outbound: build request: %w", err)
	}
	req.Header.Set("X-Api-Key", apiKey)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.hc.Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("outbound: transport: %w", err)
	}
	defer resp.Body.Close()

	respBody, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return resp.StatusCode, nil, fmt.Errorf("outbound: read response body: %w", readErr)
	}

	if resp.StatusCode == http.StatusUnauthorized {
		return resp.StatusCode, respBody, ErrOutboundAuth
	}
	if resp.StatusCode == http.StatusNotFound {
		return resp.StatusCode, respBody, ErrOutboundNotFound
	}
	if resp.StatusCode == http.StatusConflict {
		return resp.StatusCode, respBody, ErrOutboundConflict
	}
	if resp.StatusCode >= 400 {
		return resp.StatusCode, respBody, fmt.Errorf("%w: status=%d body=%s", ErrOutboundUpstream, resp.StatusCode, respBody)
	}
	return resp.StatusCode, respBody, nil
}

// joinURL concatenates a partner base URL with a protocol path segment,
// guaranteeing EXACTLY ONE slash between them regardless of whether BaseURL has
// a trailing slash. This makes outbound calls correct for both
// "https://banka-2.radenkovic.rs/api" and ".../api/" partner BaseURL forms.
//
// Banka 2's Envoy gateway requires the /api prefix in BaseURL (it strips /api
// then hits backend /interbank, /negotiations, ...). The previous code did a
// bare BaseURL+segment concatenation, so a BaseURL WITHOUT a trailing slash
// produced ".../apiinterbank" (404). Normalizing here removes that footgun.
func joinURL(base, path string) string {
	return strings.TrimRight(base, "/") + "/" + strings.TrimLeft(path, "/")
}

// ---------------------------------------------------------------------------
// bytes.Reader constructor avoids importing bytes in the call sites above
// ---------------------------------------------------------------------------

func newBytesReader(b []byte) io.Reader {
	return &bytesReader{data: b, pos: 0}
}

type bytesReader struct {
	data []byte
	pos  int
}

func (r *bytesReader) Read(p []byte) (int, error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	n := copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}
