package uuid

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var uuidRegex = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

func TestNew_ReturnsValidUUIDv4Format(t *testing.T) {
	t.Parallel()
	id, err := New()
	require.NoError(t, err)
	assert.Regexp(t, uuidRegex, id)
}

func TestNew_ProducesDifferentValues(t *testing.T) {
	t.Parallel()
	id1, _ := New()
	id2, _ := New()
	assert.NotEqual(t, id1, id2)
}

func TestNew_Version4Bit(t *testing.T) {
	t.Parallel()
	id, _ := New()
	assert.Equal(t, byte('4'), id[14], "UUID version bit must be 4")
}
