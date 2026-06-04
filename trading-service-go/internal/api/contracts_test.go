package api

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// LocalDateTime
// ---------------------------------------------------------------------------

func TestNewLocalDateTime_ValidTime(t *testing.T) {
	t.Parallel()
	tm := time.Date(2024, 6, 15, 10, 30, 45, 0, time.UTC)
	ldt := NewLocalDateTime(tm)
	assert.True(t, ldt.Valid)
	assert.Equal(t, tm, ldt.Time)
}

func TestLocalDateTimeFromPtr_NonNil(t *testing.T) {
	t.Parallel()
	tm := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	ldt := LocalDateTimeFromPtr(&tm)
	assert.True(t, ldt.Valid)
}

func TestLocalDateTimeFromPtr_Nil_ReturnsInvalid(t *testing.T) {
	t.Parallel()
	ldt := LocalDateTimeFromPtr(nil)
	assert.False(t, ldt.Valid)
}

func TestLocalDateTime_MarshalJSON_Valid(t *testing.T) {
	t.Parallel()
	tm := time.Date(2024, 6, 15, 10, 30, 45, 0, time.UTC)
	ldt := NewLocalDateTime(tm)
	data, err := json.Marshal(ldt)
	require.NoError(t, err)
	assert.Contains(t, string(data), "2024-06-15T10:30:45")
}

func TestLocalDateTime_MarshalJSON_Invalid_ReturnsNull(t *testing.T) {
	t.Parallel()
	ldt := LocalDateTime{}
	data, err := json.Marshal(ldt)
	require.NoError(t, err)
	assert.Equal(t, "null", string(data))
}

// ---------------------------------------------------------------------------
// LocalDate
// ---------------------------------------------------------------------------

func TestNewLocalDate_ValidDate(t *testing.T) {
	t.Parallel()
	tm := time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC)
	ld := NewLocalDate(tm)
	assert.True(t, ld.Valid)
}

func TestLocalDateFromPtr_Nil_ReturnsInvalid(t *testing.T) {
	t.Parallel()
	ld := LocalDateFromPtr(nil)
	assert.False(t, ld.Valid)
}

func TestLocalDateFromPtr_NonNil_ReturnsValid(t *testing.T) {
	t.Parallel()
	tm := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
	ld := LocalDateFromPtr(&tm)
	assert.True(t, ld.Valid)
}

func TestLocalDate_MarshalJSON_Valid(t *testing.T) {
	t.Parallel()
	tm := time.Date(2024, 3, 5, 0, 0, 0, 0, time.UTC)
	ld := NewLocalDate(tm)
	data, err := json.Marshal(ld)
	require.NoError(t, err)
	assert.Equal(t, `"2024-03-05"`, string(data))
}

func TestLocalDate_MarshalJSON_Invalid_ReturnsNull(t *testing.T) {
	t.Parallel()
	ld := LocalDate{}
	data, err := json.Marshal(ld)
	require.NoError(t, err)
	assert.Equal(t, "null", string(data))
}

// ---------------------------------------------------------------------------
// NewPage
// ---------------------------------------------------------------------------

func TestNewPage_CalculatesTotalPages(t *testing.T) {
	t.Parallel()
	items := []int{1, 2, 3}
	p := NewPage(items, 0, 10, 25)
	assert.Equal(t, int64(25), p.TotalElements)
	assert.Equal(t, 3, p.TotalPages)
	assert.True(t, p.First)
	assert.False(t, p.Last)
}

func TestNewPage_LastPage(t *testing.T) {
	t.Parallel()
	p := NewPage([]string{"a"}, 2, 10, 21)
	assert.True(t, p.Last)
}

func TestNewPage_NilContent_UsesEmpty(t *testing.T) {
	t.Parallel()
	p := NewPage[int](nil, 0, 10, 0)
	assert.NotNil(t, p.Content)
	assert.True(t, p.Empty)
}

func TestNewPage_ZeroSize_ZeroPages(t *testing.T) {
	t.Parallel()
	p := NewPage([]int{1}, 0, 0, 1)
	assert.Equal(t, 0, p.TotalPages)
}
