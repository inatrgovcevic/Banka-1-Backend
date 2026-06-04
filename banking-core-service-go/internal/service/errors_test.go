package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBadRequest_Status400(t *testing.T) {
	err := BadRequest("wrong value %s", "x")
	assert.Equal(t, 400, err.Status)
	assert.Contains(t, err.Error(), "wrong value x")
	assert.Equal(t, "ERR_VALIDATION", err.Code)
}

func TestNotFound_Status404(t *testing.T) {
	err := NotFound("item %d not found", 42)
	assert.Equal(t, 404, err.Status)
	assert.Contains(t, err.Error(), "42")
}

func TestConflict_Status409(t *testing.T) {
	err := Conflict("ERR_DUPE", "Conflict", "already exists")
	assert.Equal(t, 409, err.Status)
	assert.Equal(t, "ERR_DUPE", err.Code)
}

func TestForbidden_Status403(t *testing.T) {
	err := Forbidden("access denied")
	assert.Equal(t, 403, err.Status)
	assert.Equal(t, "ERR_FORBIDDEN", err.Code)
}

func TestInternal_Status500(t *testing.T) {
	err := Internal("unexpected failure")
	assert.Equal(t, 500, err.Status)
}

func TestError_NilPointer_ReturnsEmptyString(t *testing.T) {
	var err *Error
	assert.Equal(t, "", err.Error())
}
