package tldr

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFoo(t *testing.T) {
	testCases := []struct {
		houres       float64
		expectedDays int
	}{
		{1, 1},
		{24, 1},
		{0.1, 1},
		{24.00001, 2},
		{48, 2},
		{0, 0},
		{176, 8},
	}
	for _, tt := range testCases {
		result := int(math.Ceil(tt.houres / 24))
		assert.Equal(t, tt.expectedDays, result)
	}
}
