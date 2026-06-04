package grpc

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJSONCodec_Name(t *testing.T) {
	t.Parallel()
	c := jsonCodec{}
	assert.Equal(t, "json", c.Name())
}

func TestJSONCodec_Marshal_ValidStruct(t *testing.T) {
	t.Parallel()
	c := jsonCodec{}
	type msg struct {
		Value string `json:"value"`
	}
	data, err := c.Marshal(msg{Value: "hello"})
	require.NoError(t, err)
	assert.Contains(t, string(data), "hello")
}

func TestJSONCodec_Unmarshal_ValidData(t *testing.T) {
	t.Parallel()
	c := jsonCodec{}
	type msg struct {
		Value string `json:"value"`
	}
	var out msg
	require.NoError(t, c.Unmarshal([]byte(`{"value":"world"}`), &out))
	assert.Equal(t, "world", out.Value)
}

func TestJSONCodec_Unmarshal_InvalidData_ReturnsError(t *testing.T) {
	t.Parallel()
	c := jsonCodec{}
	var out map[string]any
	err := c.Unmarshal([]byte("{bad json"), &out)
	require.Error(t, err)
}
