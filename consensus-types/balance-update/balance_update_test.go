package balanceupdate

import (
	"testing"

	"github.com/prysmaticlabs/prysm/v3/testing/assert"
)

func TestBalanceUpdateAddSub(t *testing.T) {
	bu := BalanceUpdate{}
	bu.Add(42)
	bu.Sub(84)

	assert.Equal(t, uint64(42), bu.Decrease)
	assert.Equal(t, uint64(0), bu.Increase)
	assert.Equal(t, false, bu.IsZero())
}
