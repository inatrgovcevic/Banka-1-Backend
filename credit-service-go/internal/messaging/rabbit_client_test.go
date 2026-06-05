package messaging

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetenv_WithValue_ReturnsValue(t *testing.T) {
	t.Setenv("TEST_RABBIT_KEY", "myvalue")
	result := getenv("TEST_RABBIT_KEY", "default")
	assert.Equal(t, "myvalue", result)
}

func TestGetenv_WithoutValue_ReturnsFallback(t *testing.T) {
	t.Setenv("TEST_RABBIT_KEY_MISSING", "")
	result := getenv("TEST_RABBIT_KEY_MISSING", "fallback")
	assert.Equal(t, "fallback", result)
}

func TestNewRabbitClient_NoServer_ReturnsDisabledClient(t *testing.T) {
	// Use a port where nothing is running to get immediate connection refusal
	t.Setenv("RABBITMQ_HOST", "localhost")
	t.Setenv("RABBITMQ_PORT", "19872")
	t.Setenv("RABBITMQ_USERNAME", "guest")
	t.Setenv("RABBITMQ_PASSWORD", "guest")

	c, err := NewRabbitClient()
	require.Error(t, err)
	require.NotNil(t, c)

	// Disabled client PublishJSON should be a no-op
	publishErr := c.PublishJSON(context.Background(), "test.key", map[string]string{"k": "v"})
	assert.NoError(t, publishErr)
}

func TestRabbitClient_PublishJSON_NilReceiver_ReturnsNil(t *testing.T) {
	var c *RabbitClient
	err := c.PublishJSON(context.Background(), "test.key", "payload")
	assert.NoError(t, err)
}

func TestRabbitClient_Close_NilReceiver_DoesNotPanic(t *testing.T) {
	var c *RabbitClient
	assert.NotPanics(t, func() {
		c.Close()
	})
}

func TestRabbitClient_Close_DisabledClient_DoesNotPanic(t *testing.T) {
	t.Setenv("RABBITMQ_PORT", "19873")
	c, _ := NewRabbitClient()
	assert.NotPanics(t, func() {
		c.Close()
	})
}

func TestRabbitClient_PublishJSON_DisabledClient_ReturnsNil(t *testing.T) {
	t.Setenv("RABBITMQ_PORT", "19874")
	c, _ := NewRabbitClient()

	err := c.PublishJSON(context.Background(), "any.key", struct{ Name string }{Name: "test"})
	assert.NoError(t, err)
}
