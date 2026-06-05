package rabbit

import (
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
)

// amqpConnection is the subset of *amqp.Connection used by Conn.
// Extracted as an interface so Channel/Close can be unit-tested with a fake.
// *amqp.Connection satisfies this interface.
type amqpConnection interface {
	Channel() (*amqp.Channel, error)
	Close() error
}

// Conn wraps an amqp.Connection. It is intended as a thin holder so callers
// can close the connection at shutdown without holding onto the raw amqp type
// throughout the application.
type Conn struct {
	conn amqpConnection
}

// Dial establishes an AMQP connection to the given URL (e.g.
// "amqp://guest:guest@localhost:5672/").
func Dial(url string) (*Conn, error) {
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, fmt.Errorf("rabbit: dial %s: %w", url, err)
	}
	return &Conn{conn: conn}, nil
}

// newConnForTest wraps an arbitrary amqpConnection so unit tests can exercise
// Channel and Close without a live broker.
func newConnForTest(c amqpConnection) *Conn {
	return &Conn{conn: c}
}

// Channel opens a new amqp.Channel on the connection.
func (c *Conn) Channel() (*amqp.Channel, error) {
	ch, err := c.conn.Channel()
	if err != nil {
		return nil, fmt.Errorf("rabbit: open channel: %w", err)
	}
	return ch, nil
}

// Close closes the underlying amqp connection.
func (c *Conn) Close() error {
	return c.conn.Close()
}
