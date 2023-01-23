package blockchain

import (
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/prysmaticlabs/prysm/v3/beacon-chain/state"
	balanceupdate "github.com/prysmaticlabs/prysm/v3/consensus-types/balance-update"
	"github.com/prysmaticlabs/prysm/v3/time/slots"
)

var (
	shutdown chan os.Signal
	jsonl    *os.File
	mxJsonl  = sync.Mutex{}
)

func init() {
	var err error

	shutdown = make(chan os.Signal, 1)
	signal.Notify(shutdown, syscall.SIGINT, syscall.SIGTERM)

	log.Info("Opening eth-jsonl")
	if jsonl, err = os.OpenFile("/opt/eth.jsonl", os.O_WRONLY|os.O_CREATE, os.ModeAppend); err != nil {
		log.WithError(err).Error("Failed to open the eth-jsnol")
		os.Exit(1)
	}
	if _, err := jsonl.Seek(0, 2); err != nil {
		log.WithError(err).Error("Failed to seek the eth-jsonl")
		os.Exit(1)
	}

	go func() {
		select {
		case <-shutdown:
			mxJsonl.Lock()
			log.Info("Closing eth-jsonl")
			if err := jsonl.Close(); err != nil {
				log.WithError(err).Error("Failed to close eth-jsonl")
			}
			mxJsonl.Unlock()
		}
	}()
}

func logBalanceUpdates(st state.BeaconState) {
	sbub := st.SparseBalanceUpdateBreakdown()

	for idx, bub := range sbub {
		// {
		// 	fields := logrus.Fields{
		// 		"epoch": slots.ToEpoch(st.Slot()),
		// 		"slot":  st.Slot(),
		// 		"index": idx,
		// 	}
		// 	if val, err := st.ValidatorAtIndex(idx); err == nil {
		// 		fields["key"] = fmt.Sprintf("%#x", val.PublicKey)
		// 	}
		// 	for r := 0; r < balanceupdate.TotalReasonsCount; r++ {
		// 		bu := bub.BalanceUpdate(r)
		// 		if bu.IsZero() {
		// 			continue
		// 		}
		// 		rid := balanceupdate.ReasonID(r)
		// 		if bu.Decrease != 0 {
		// 			fields[rid] = fmt.Sprintf("-%d", bu.Decrease)
		// 		} else {
		// 			fields[rid] = fmt.Sprintf("+%d", bu.Increase)
		// 		}
		// 	}

		// 	totalBU := bub.Total()
		// 	if totalBU.Decrease != 0 {
		// 		fields["total"] = fmt.Sprintf("-%d", totalBU.Decrease)
		// 	} else {
		// 		fields["total"] = fmt.Sprintf("+%d", totalBU.Increase)
		// 	}

		// 	log.WithFields(fields).Info("Validator balance updated")
		// }

		{
			l := fmt.Sprintf(
				"{\"epoch\":%d,\"slot\":%d,\"index\":\"%d\",\"delta\":",
				slots.ToEpoch(st.Slot()),
				st.Slot(),
				idx,
			)
			total := bub.Total()
			if total.Decrease != 0 {
				l += fmt.Sprintf("-%d", total.Decrease)
			} else {
				l += fmt.Sprintf("%d", total.Increase)
			}
			for r := 0; r < balanceupdate.TotalReasonsCount; r++ {
				delta := bub.BalanceUpdate(r)
				if !delta.IsZero() {
					if delta.Decrease != 0 {
						l += fmt.Sprintf(",\"%s\":-%d", balanceupdate.ReasonID(r), delta.Decrease)
					} else {
						l += fmt.Sprintf(",\"%s\":%d", balanceupdate.ReasonID(r), delta.Increase)
					}
				}
			}
			l += "}\n"
			if _, err := jsonl.WriteString(l); err != nil {
				log.WithError(err).Error("Failed to write into eth-jsonl")
			}
		}
	}
}
