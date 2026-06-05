package grpc

import (
	"errors"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/raf-si-2025/banka-1-go/interbank-service/internal/protocol"
	"github.com/raf-si-2025/banka-1-go/interbank-service/internal/service"
)

func TestMapServiceError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want codes.Code
	}{
		{"not found", service.ErrNegotiationNotFound, codes.NotFound},
		{"invalid", service.ErrNegotiationInvalid, codes.InvalidArgument},
		{"closed", service.ErrNegotiationClosed, codes.FailedPrecondition},
		{"turn", service.ErrTurnViolation, codes.Aborted},
		{"sender not party", service.ErrSenderNotParty, codes.PermissionDenied},
		{"interbank protocol", service.ErrInterbankProtocol, codes.Internal},
		{"unknown", errors.New("boom"), codes.Internal},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := mapServiceError(tc.err, "msg")
			st, ok := status.FromError(got)
			if !ok {
				t.Fatalf("not a status error: %v", got)
			}
			if st.Code() != tc.want {
				t.Errorf("code=%v want %v", st.Code(), tc.want)
			}
		})
	}
}

func TestMarshalVoteToJSON(t *testing.T) {
	if got := marshalVoteToJSON(protocol.TransactionVote{Vote: protocol.VoteYes}); got != `{"vote":"YES"}` {
		t.Errorf("yes → %q", got)
	}
	if got := marshalVoteToJSON(protocol.TransactionVote{Vote: protocol.VoteNo}); got != `{"vote":"NO"}` {
		t.Errorf("no → %q", got)
	}
	// default (empty) → NO
	if got := marshalVoteToJSON(protocol.TransactionVote{}); got != `{"vote":"NO"}` {
		t.Errorf("empty → %q", got)
	}
}

func TestUnmarshalVoteFromJSON(t *testing.T) {
	var v protocol.TransactionVote
	if err := unmarshalVoteFromJSON(`{"vote":"YES"}`, &v); err != nil || v.Vote != protocol.VoteYes {
		t.Errorf("yes: vote=%q err=%v", v.Vote, err)
	}
	v = protocol.TransactionVote{}
	if err := unmarshalVoteFromJSON(`{"vote":"NO"}`, &v); err != nil || v.Vote != protocol.VoteNo {
		t.Errorf("no: vote=%q err=%v", v.Vote, err)
	}
	v = protocol.TransactionVote{}
	if err := unmarshalVoteFromJSON(`{"vote":"???"}`, &v); err == nil {
		t.Error("unrecognised body should error")
	}
}

func TestIntPtr(t *testing.T) {
	p := intPtr(204)
	if p == nil || *p != 204 {
		t.Errorf("intPtr(204) → %v", p)
	}
}
