package balanceupdate

import (
	"testing"

	"github.com/prysmaticlabs/prysm/v3/testing/assert"
)

func TestBreakdownAddSub(t *testing.T) {
	b := Breakdown{}
	b.Add(42, ValidatorDeposit)
	b.Sub(84, ValidatorDeposit)
	assert.Equal(t, uint64(42), b.Total().Decrease)
	assert.Equal(t, uint64(0), b.Total().Increase)
}
