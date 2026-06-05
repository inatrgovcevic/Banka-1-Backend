package client_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"Banka1Back/credit-service-go/internal/client"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClientServiceClient_AddMarginPermission_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method)
		assert.Contains(t, r.URL.Path, "/clients/customers/margin/")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	t.Setenv("SERVICES_USER_URL", server.URL)
	c := client.NewClientServiceClient()

	err := c.AddMarginPermission(42)
	require.NoError(t, err)
}

func TestClientServiceClient_AddMarginPermission_FailedStatus_ReturnsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()

	t.Setenv("SERVICES_USER_URL", server.URL)
	c := client.NewClientServiceClient()

	err := c.AddMarginPermission(42)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "margin permission failed")
}

func TestClientServiceClient_AddMarginPermission_InternalServerError_ReturnsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	t.Setenv("SERVICES_USER_URL", server.URL)
	c := client.NewClientServiceClient()

	err := c.AddMarginPermission(99)
	require.Error(t, err)
}

func TestClientServiceClient_AddMarginPermission_NetworkError_ReturnsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	server.Close()

	t.Setenv("SERVICES_USER_URL", server.URL)
	c := client.NewClientServiceClient()

	err := c.AddMarginPermission(1)
	require.Error(t, err)
}

func TestNewClientServiceClient_DefaultURL(t *testing.T) {
	t.Setenv("SERVICES_USER_URL", "")
	c := client.NewClientServiceClient()
	require.NotNil(t, c)
}

func TestNewClientServiceClient_CustomURL(t *testing.T) {
	t.Setenv("SERVICES_USER_URL", "http://user-service:8081")
	c := client.NewClientServiceClient()
	require.NotNil(t, c)
}
