package clients

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestJoinCSV_MultipleItems(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "a,b,c", joinCSV([]string{"a", "b", "c"}))
}

func TestJoinCSV_SingleItem(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "x", joinCSV([]string{"x"}))
}

func TestJoinCSV_EmptySlice_ReturnsEmpty(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "", joinCSV([]string{}))
}
