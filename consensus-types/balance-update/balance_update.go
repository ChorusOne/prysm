package balanceupdate

type BalanceUpdate struct {
	Increase uint64
	Decrease uint64
}

func (bu *BalanceUpdate) Add(delta uint64) {
	// Since balance updates are the `uint64` deltas applied to the
	// existing `uint64` balance, we do not need to check for the
	// math overflows during add/sub operations.  Reason being, the
	// add/sub operation on balance is checked _before_ we log the
	// BalanceUpdate, and therefore the add/sub operation on cumulative
	// delta will always be capped by max(uint64).

	bu.Increase += delta
	if bu.Decrease == 0 {
		return
	}
	if bu.Increase > bu.Decrease {
		bu.Increase -= bu.Decrease
		bu.Decrease = 0
	} else {
		bu.Decrease -= bu.Increase
		bu.Increase = 0
	}
}

func (bu *BalanceUpdate) Sub(delta uint64) {
	// Since balance updates are the `uint64` deltas applied to the
	// existing `uint64` balance, we do not need to check for the
	// math overflows during add/sub operations.  Reason being, the
	// add/sub operation on balance is checked _before_ we log the
	// BalanceUpdate, and therefore the add/sub operation on cumulative
	// delta will always be capped by max(uint64).

	bu.Decrease += delta
	if bu.Increase == 0 {
		return
	}
	if bu.Increase > bu.Decrease {
		bu.Increase -= bu.Decrease
		bu.Decrease = 0
	} else {
		bu.Decrease -= bu.Increase
		bu.Increase = 0
	}
}

func (bu BalanceUpdate) IsZero() bool {
	return bu.Increase == 0 && bu.Decrease == 0
}
