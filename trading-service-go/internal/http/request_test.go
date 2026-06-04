package http

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// optionalQueryParam
// ---------------------------------------------------------------------------

func TestOptionalQueryParam_Present_ReturnsPointer(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest(http.MethodGet, "/path?foo=bar", nil)
	result := optionalQueryParam(req, "foo")
	require.NotNil(t, result)
	assert.Equal(t, "bar", *result)
}

func TestOptionalQueryParam_PresentEmpty_ReturnsEmptyPointer(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest(http.MethodGet, "/path?foo=", nil)
	result := optionalQueryParam(req, "foo")
	require.NotNil(t, result)
	assert.Equal(t, "", *result)
}

func TestOptionalQueryParam_Absent_ReturnsNil(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest(http.MethodGet, "/path", nil)
	result := optionalQueryParam(req, "missing")
	assert.Nil(t, result)
}

// ---------------------------------------------------------------------------
// queryIntDefault
// ---------------------------------------------------------------------------

func TestQueryIntDefault_Present_ReturnsParsed(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest(http.MethodGet, "/path?page=3", nil)
	assert.Equal(t, 3, queryIntDefault(req, "page", 0))
}

func TestQueryIntDefault_Absent_ReturnsDefault(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest(http.MethodGet, "/path", nil)
	assert.Equal(t, 10, queryIntDefault(req, "size", 10))
}

func TestQueryIntDefault_InvalidValue_ReturnsDefault(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest(http.MethodGet, "/path?page=abc", nil)
	assert.Equal(t, 0, queryIntDefault(req, "page", 0))
}

// ---------------------------------------------------------------------------
// parseDateTimeParam
// ---------------------------------------------------------------------------

func TestParseDateTimeParam_Absent_ReturnsNil(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest(http.MethodGet, "/path", nil)
	result, err := parseDateTimeParam(req, "start")
	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestParseDateTimeParam_EmptyValue_ReturnsNil(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest(http.MethodGet, "/path?start=", nil)
	result, err := parseDateTimeParam(req, "start")
	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestParseDateTimeParam_ValidISO_ReturnsTime(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest(http.MethodGet, "/path?start=2024-06-15T10:30:00", nil)
	result, err := parseDateTimeParam(req, "start")
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 2024, result.Year())
	assert.Equal(t, 15, result.Day())
}

func TestParseDateTimeParam_ValidRFC3339_ReturnsTime(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest(http.MethodGet, "/path?start=2024-06-15T10:30:00Z", nil)
	result, err := parseDateTimeParam(req, "start")
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestParseDateTimeParam_InvalidFormat_ReturnsError(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest(http.MethodGet, "/path?start=not-a-date", nil)
	_, err := parseDateTimeParam(req, "start")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid datetime")
}

// ---------------------------------------------------------------------------
// decodeJSONLenient
// ---------------------------------------------------------------------------

func TestDecodeJSONLenient_ValidBody_Decodes(t *testing.T) {
	t.Parallel()
	type dto struct{ Value string }
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{"value":"hello"}`))
	var out dto
	require.NoError(t, decodeJSONLenient(req, &out))
	assert.Equal(t, "hello", out.Value)
}

func TestDecodeJSONLenient_EmptyBody_NoError(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(""))
	var out map[string]any
	require.NoError(t, decodeJSONLenient(req, &out))
}

func TestDecodeJSONLenient_InvalidJSON_ReturnsError(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString("{bad}"))
	var out map[string]any
	require.Error(t, decodeJSONLenient(req, &out))
}

func TestDecodeJSONLenient_UnknownFields_Ignored(t *testing.T) {
	t.Parallel()
	type dto struct{ Value string }
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{"value":"hi","unknown":true}`))
	var out dto
	require.NoError(t, decodeJSONLenient(req, &out))
	assert.Equal(t, "hi", out.Value)
}
