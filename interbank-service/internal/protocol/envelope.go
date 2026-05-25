package protocol

import (
	"encoding/json"
	"fmt"
)

// MessageType discriminates the three inter-bank protocol message bodies.
// Java: enum MessageType { NEW_TX, COMMIT_TX, ROLLBACK_TX }
type MessageType string

const (
	MessageTypeNewTx      MessageType = "NEW_TX"
	MessageTypeCommitTx   MessageType = "COMMIT_TX"
	MessageTypeRollbackTx MessageType = "ROLLBACK_TX"
)

// InterbankMessagePayload is the top-level envelope sent on POST /interbank.
// The Message field is decoded based on the MessageType discriminator:
//   - NEW_TX        → *InterbankTransactionPayload
//   - COMMIT_TX     → *CommitTransactionBody
//   - ROLLBACK_TX   → *RollbackTransactionBody
//
// Java: record InterbankMessagePayload(IdempotenceKey idempotenceKey,
//
//	MessageType messageType, JsonNode message)
//
// The Java side uses JsonNode message and dispatches manually in InboundDispatcher;
// we replicate this by parsing message based on the messageType discriminator.
type InterbankMessagePayload struct {
	IdempotenceKey IdempotenceKey `json:"idempotenceKey" validate:"required"`
	MessageType    MessageType    `json:"messageType"    validate:"required,oneof=NEW_TX COMMIT_TX ROLLBACK_TX"`
	// Message is one of: *InterbankTransactionPayload, *CommitTransactionBody,
	// *RollbackTransactionBody, depending on MessageType.
	Message any `json:"message" validate:"required"`
}

// UnmarshalJSON decodes the inner message according to the messageType discriminator.
// This mirrors the Java InboundDispatcher switch on MessageType.
func (m *InterbankMessagePayload) UnmarshalJSON(data []byte) error {
	// Step 1: decode the envelope keeping message as raw JSON.
	var env struct {
		IdempotenceKey IdempotenceKey  `json:"idempotenceKey"`
		MessageType    MessageType     `json:"messageType"`
		Message        json.RawMessage `json:"message"`
	}
	if err := json.Unmarshal(data, &env); err != nil {
		return err
	}
	m.IdempotenceKey = env.IdempotenceKey
	m.MessageType = env.MessageType

	// Step 2: dispatch on messageType to decode the inner message body.
	switch env.MessageType {
	case MessageTypeNewTx:
		var tx InterbankTransactionPayload
		if err := json.Unmarshal(env.Message, &tx); err != nil {
			return fmt.Errorf("decode NEW_TX body: %w", err)
		}
		m.Message = &tx
	case MessageTypeCommitTx:
		var c CommitTransactionBody
		if err := json.Unmarshal(env.Message, &c); err != nil {
			return fmt.Errorf("decode COMMIT_TX body: %w", err)
		}
		m.Message = &c
	case MessageTypeRollbackTx:
		var r RollbackTransactionBody
		if err := json.Unmarshal(env.Message, &r); err != nil {
			return fmt.Errorf("decode ROLLBACK_TX body: %w", err)
		}
		m.Message = &r
	default:
		return fmt.Errorf("unknown messageType %q", env.MessageType)
	}
	return nil
}

// MarshalJSON serializes the envelope, emitting the concrete message body
// using json.Marshal (which will call the appropriate MarshalJSON if present).
func (m InterbankMessagePayload) MarshalJSON() ([]byte, error) {
	msgBytes, err := json.Marshal(m.Message)
	if err != nil {
		return nil, fmt.Errorf("marshal message body: %w", err)
	}
	type wireEnvelope struct {
		IdempotenceKey IdempotenceKey  `json:"idempotenceKey"`
		MessageType    MessageType     `json:"messageType"`
		Message        json.RawMessage `json:"message"`
	}
	return json.Marshal(wireEnvelope{
		IdempotenceKey: m.IdempotenceKey,
		MessageType:    m.MessageType,
		Message:        msgBytes,
	})
}
