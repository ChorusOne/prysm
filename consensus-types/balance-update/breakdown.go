package balanceupdate

type Breakdown struct {
	breakdown [TotalReasonsCount]BalanceUpdate
}

func (b *Breakdown) Add(delta uint64, reason Reason) {
	b.breakdown[reason].Add(delta)
}

func (b *Breakdown) Sub(delta uint64, reason Reason) {
	b.breakdown[reason].Sub(delta)
}

func (b Breakdown) BalanceUpdate(r Reason) BalanceUpdate {
	return b.breakdown[r]
}

func (b Breakdown) Total() BalanceUpdate {
	var totalInc, totalDec uint64
	for _, bu := range b.breakdown {
		if bu.Increase != 0 {
			totalInc += bu.Increase
		}
		if bu.Decrease != 0 {
			totalDec += bu.Decrease
		}
	}
	if totalInc != 0 && totalDec != 0 {
		if totalInc > totalDec {
			totalInc -= totalDec
			totalDec = 0
		} else {
			totalDec -= totalInc
			totalInc = 0
		}
	}
	return BalanceUpdate{
		Increase: totalInc,
		Decrease: totalDec,
	}
}

func (b Breakdown) IsZero() bool {
	for r := 0; r < TotalReasonsCount; r++ {
		if !b.breakdown[r].IsZero() {
			return false
		}
	}
	return true
}
