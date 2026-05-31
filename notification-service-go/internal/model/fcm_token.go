package model

import "time"

// FcmToken tracks the single active Firebase Cloud Messaging device token per client.
// Design invariant: one client → one active device token.
type FcmToken struct {
	ID        int64
	ClientId  int64
	Token     string
	UpdatedAt time.Time
}
