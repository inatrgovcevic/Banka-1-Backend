package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewPage_CalculatesTotalPages(t *testing.T) {
	p := NewPage([]int{1, 2, 3}, 0, 10, 25)
	assert.Equal(t, 25, p.TotalElements)
	assert.Equal(t, 3, p.TotalPages)
	assert.Equal(t, 3, p.NumberOfElements)
	assert.True(t, p.First)
	assert.False(t, p.Last)
	assert.False(t, p.Empty)
}

func TestNewPage_LastPage(t *testing.T) {
	p := NewPage([]string{"a"}, 2, 10, 21)
	assert.True(t, p.Last)
	assert.False(t, p.First)
}

func TestNewPage_Empty(t *testing.T) {
	p := NewPage([]int{}, 0, 10, 0)
	assert.True(t, p.Empty)
	assert.True(t, p.Last)
	assert.Equal(t, 0, p.TotalPages)
}

func TestNewPage_ZeroSize_DefaultsToTen(t *testing.T) {
	p := NewPage([]int{1, 2}, 0, 0, 2)
	assert.Equal(t, 10, p.Size)
}

func TestNewPage_ExactlyOneFullPage(t *testing.T) {
	p := NewPage([]int{1, 2, 3}, 0, 3, 3)
	assert.Equal(t, 1, p.TotalPages)
	assert.True(t, p.Last)
	assert.True(t, p.First)
}
