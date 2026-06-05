package rabbit

import (
	"errors"
	"strings"
	"testing"

	amqp "github.com/rabbitmq/amqp091-go"
)

func TestDial_InvalidURL(t *testing.T) {
	// A malformed scheme makes amqp.Dial fail without attempting a network call.
	_, err := Dial("not-a-valid-amqp-url")
	if err == nil {
		t.Fatal("expected error dialing invalid URL")
	}
	if !strings.Contains(err.Error(), "rabbit: dial") {
		t.Errorf("error should be wrapped with context; got %v", err)
	}
}

func TestDial_UnreachableBroker(t *testing.T) {
	// Valid URL syntax but nothing listening — Dial must return an error,
	// not panic, and must not hang indefinitely. Use a reserved discard port.
	_, err := Dial("amqp://guest:guest@127.0.0.1:1/")
	if err == nil {
		t.Fatal("expected error dialing unreachable broker")
	}
}

// fakeAMQPConn implements amqpConnection for Conn tests.
type fakeAMQPConn struct {
	channelErr error
	closeErr   error
	closed     bool
}

func (f *fakeAMQPConn) Channel() (*amqp.Channel, error) {
	if f.channelErr != nil {
		return nil, f.channelErr
	}
	// Returning a nil *amqp.Channel is fine: Conn.Channel only forwards it.
	return nil, nil
}

func (f *fakeAMQPConn) Close() error {
	f.closed = true
	return f.closeErr
}

func TestConn_Channel_Success(t *testing.T) {
	c := newConnForTest(&fakeAMQPConn{})
	if _, err := c.Channel(); err != nil {
		t.Fatalf("Channel error: %v", err)
	}
}

func TestConn_Channel_Error(t *testing.T) {
	c := newConnForTest(&fakeAMQPConn{channelErr: errors.New("no channel")})
	_, err := c.Channel()
	if err == nil {
		t.Fatal("expected error from Channel")
	}
	if !strings.Contains(err.Error(), "rabbit: open channel") {
		t.Errorf("error should be wrapped; got %v", err)
	}
}

func TestConn_Close(t *testing.T) {
	fc := &fakeAMQPConn{}
	c := newConnForTest(fc)
	if err := c.Close(); err != nil {
		t.Fatalf("Close error: %v", err)
	}
	if !fc.closed {
		t.Error("expected underlying connection to be closed")
	}
}

func TestConn_Close_Error(t *testing.T) {
	c := newConnForTest(&fakeAMQPConn{closeErr: errors.New("close boom")})
	if err := c.Close(); err == nil {
		t.Fatal("expected error from Close")
	}
}
