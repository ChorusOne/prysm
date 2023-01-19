package helpers

import (
	"errors"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"

	// pgx "github.com/jackc/pgx"
	"github.com/prysmaticlabs/prysm/v3/beacon-chain/cache"
	"github.com/prysmaticlabs/prysm/v3/beacon-chain/state"
	"github.com/prysmaticlabs/prysm/v3/config/params"
	types "github.com/prysmaticlabs/prysm/v3/consensus-types/primitives"
	mathutil "github.com/prysmaticlabs/prysm/v3/math"
	"github.com/prysmaticlabs/prysm/v3/time/slots"
	"github.com/sirupsen/logrus"
	log "github.com/sirupsen/logrus"
)

var balanceCache = cache.NewEffectiveBalanceCache()

const (
	ReasonPenaliseAttesterForMissedAttestation     = "penaliseAttesterForMissedAttestation"
	ReasonProcessDeposit                           = "processDeposit"
	ReasonProcessSlashing                          = "processSlashing"
	ReasonProcessWithdrawal                        = "processWithdrawal"
	ReasonRewardAttesterForAttestation             = "rewardAttesterForAttestation"
	ReasonRewardProposerForAttestations            = "rewardProposerForAttestations"
	ReasonRewardProposerForIncludingWhistleblowing = "rewardProposerForIncludingWhistleblowing"
	ReasonRewardProposerForProposal                = "rewardProposerForProposal"
	ReasonRewardWhistleblowerForReporting          = "rewardWhistleblowerForReporting"
)

// TotalBalance returns the total amount at stake in Gwei
// of input validators.
//
// Spec pseudocode definition:
//
//	def get_total_balance(state: BeaconState, indices: Set[ValidatorIndex]) -> Gwei:
//	 """
//	 Return the combined effective balance of the ``indices``.
//	 ``EFFECTIVE_BALANCE_INCREMENT`` Gwei minimum to avoid divisions by zero.
//	 Math safe up to ~10B ETH, afterwhich this overflows uint64.
//	 """
//	 return Gwei(max(EFFECTIVE_BALANCE_INCREMENT, sum([state.validators[index].effective_balance for index in indices])))
func TotalBalance(state state.ReadOnlyValidators, indices []types.ValidatorIndex) uint64 {
	total := uint64(0)

	for _, idx := range indices {
		val, err := state.ValidatorAtIndexReadOnly(idx)
		if err != nil {
			continue
		}
		total += val.EffectiveBalance()
	}

	// EFFECTIVE_BALANCE_INCREMENT is the lower bound for total balance.
	if total < params.BeaconConfig().EffectiveBalanceIncrement {
		return params.BeaconConfig().EffectiveBalanceIncrement
	}

	return total
}

// TotalActiveBalance returns the total amount at stake in Gwei
// of active validators.
//
// Spec pseudocode definition:
//
//	def get_total_active_balance(state: BeaconState) -> Gwei:
//	 """
//	 Return the combined effective balance of the active validators.
//	 Note: ``get_total_balance`` returns ``EFFECTIVE_BALANCE_INCREMENT`` Gwei minimum to avoid divisions by zero.
//	 """
//	 return get_total_balance(state, set(get_active_validator_indices(state, get_current_epoch(state))))
func TotalActiveBalance(s state.ReadOnlyBeaconState) (uint64, error) {
	bal, err := balanceCache.Get(s)
	switch {
	case err == nil:
		return bal, nil
	case errors.Is(err, cache.ErrNotFound):
		// Do nothing if we receive a not found error.
	default:
		// In the event, we encounter another error we return it.
		return 0, err
	}

	total := uint64(0)
	epoch := slots.ToEpoch(s.Slot())
	if err := s.ReadFromEveryValidator(func(idx int, val state.ReadOnlyValidator) error {
		if IsActiveValidatorUsingTrie(val, epoch) {
			total += val.EffectiveBalance()
		}
		return nil
	}); err != nil {
		return 0, err
	}

	// Spec defines `EffectiveBalanceIncrement` as min to avoid divisions by zero.
	total = mathutil.Max(params.BeaconConfig().EffectiveBalanceIncrement, total)
	if err := balanceCache.AddTotalEffectiveBalance(s, total); err != nil {
		return 0, err
	}

	return total, nil
}

// IncreaseBalance increases validator with the given 'index' balance by 'delta' in Gwei.
//
// Spec pseudocode definition:
//
//	def increase_balance(state: BeaconState, index: ValidatorIndex, delta: Gwei) -> None:
//	  """
//	  Increase the validator balance at index ``index`` by ``delta``.
//	  """
//	  state.balances[index] += delta
func IncreaseBalance(state state.BeaconState, idx types.ValidatorIndex, delta uint64, reason string) error {
	balAtIdx, err := state.BalanceAtIndex(idx)
	if err != nil {
		return err
	}
	newBal, err := IncreaseBalanceWithVal(balAtIdx, delta)
	if err != nil {
		return err
	}
	emitEvent(state, idx, delta, true, reason)
	return state.UpdateBalancesAtIndex(idx, newBal)
}

// IncreaseBalanceWithVal increases validator with the given 'index' balance by 'delta' in Gwei.
// This method is flattened version of the spec method, taking in the raw balance and returning
// the post balance.
//
// Spec pseudocode definition:
//
//	def increase_balance(state: BeaconState, index: ValidatorIndex, delta: Gwei) -> None:
//	  """
//	  Increase the validator balance at index ``index`` by ``delta``.
//	  """
//	  state.balances[index] += delta
func IncreaseBalanceWithVal(currBalance, delta uint64) (uint64, error) {
	return mathutil.Add64(currBalance, delta)
}

// DecreaseBalance decreases validator with the given 'index' balance by 'delta' in Gwei.
//
// Spec pseudocode definition:
//
//	def decrease_balance(state: BeaconState, index: ValidatorIndex, delta: Gwei) -> None:
//	  """
//	  Decrease the validator balance at index ``index`` by ``delta``, with underflow protection.
//	  """
//	  state.balances[index] = 0 if delta > state.balances[index] else state.balances[index] - delta
func DecreaseBalance(state state.BeaconState, idx types.ValidatorIndex, delta uint64, reason string) error {
	balAtIdx, err := state.BalanceAtIndex(idx)
	if err != nil {
		return err
	}
	emitEvent(state, idx, delta, false, reason)
	return state.UpdateBalancesAtIndex(idx, DecreaseBalanceWithVal(balAtIdx, delta))
}

var (
	shutdown chan os.Signal

	// psql *pgx.Conn

	jsonl   *os.File
	mxJsonl = sync.Mutex{}
)

func init() {
	var err error

	shutdown = make(chan os.Signal, 1)
	signal.Notify(shutdown, syscall.SIGINT, syscall.SIGTERM)

	// log.Info("Opening eth-db connection")
	// psql, err = pgx.Connect(pgx.ConnConfig{
	// 	Host:     "127.0.0.1",
	// 	Port:     5432,
	// 	Database: "eth",
	// 	User:     "root",
	// 	Password: "73e1c8c0cfbc43d01b2dcc8e900557a8dcc1226d3127772b140edd7ebfb2e865",
	// })
	// if err != nil {
	// 	log.WithError(err).Error("Failed to connect to the eth-db")
	// 	os.Exit(1)
	// }

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

			// log.Info("Closing eth-db")
			// if err := psql.Close(); err != nil {
			// 	log.WithError(err).Error("Failed to close eth-db")
			// }
		}
	}()
}

func emitEvent(state state.BeaconState, idx types.ValidatorIndex, delta uint64, increment bool, reason string) {
	if delta == 0 || len(reason) == 0 {
		return
	}

	slot := state.Slot()
	epoch := slots.ToEpoch(slot)

	var key [48]byte
	if validator, err := state.ValidatorAtIndexReadOnly(idx); err == nil {
		key = validator.PublicKey()
	}
	strKey := fmt.Sprintf("%#x", key)

	var strDelta string
	if increment {
		strDelta = fmt.Sprintf("%d", delta)
	} else {
		strDelta = fmt.Sprintf("-%d", delta)
	}

	parentRoot := fmt.Sprintf("%#x", state.LatestBlockHeader().ParentRoot)

	log.WithFields(logrus.Fields{
		"epoch":      epoch,
		"slot":       slot,
		"parentRoot": parentRoot,
		"key":        strKey,
		"delta":      strDelta,
		"reason":     reason,
	}).Info("Validator balance updated")

	mxJsonl.Lock()
	if _, err := jsonl.WriteString(fmt.Sprintf(
		"{\"epoch\":%d,\"slot\":%d,\"parentRoot\":\"%s\",\"key\",\"%s\",\"delta\":%s,\"reason\":\"%s\"}\n",
		epoch, slot, parentRoot, strKey, strDelta, reason,
	)); err != nil {
		log.WithError(err).Error("Failed to write into eth-jsonl")
	}
	mxJsonl.Unlock()
}

// DecreaseBalanceWithVal decreases validator with the given 'index' balance by 'delta' in Gwei.
// This method is flattened version of the spec method, taking in the raw balance and returning
// the post balance.
//
// Spec pseudocode definition:
//
//	def decrease_balance(state: BeaconState, index: ValidatorIndex, delta: Gwei) -> None:
//	  """
//	  Decrease the validator balance at index ``index`` by ``delta``, with underflow protection.
//	  """
//	  state.balances[index] = 0 if delta > state.balances[index] else state.balances[index] - delta
func DecreaseBalanceWithVal(currBalance, delta uint64) uint64 {
	if delta > currBalance {
		return 0
	}
	return currBalance - delta
}

// IsInInactivityLeak returns true if the state is experiencing inactivity leak.
//
// Spec code:
// def is_in_inactivity_leak(state: BeaconState) -> bool:
//
//	return get_finality_delay(state) > MIN_EPOCHS_TO_INACTIVITY_PENALTY
func IsInInactivityLeak(prevEpoch, finalizedEpoch types.Epoch) bool {
	return FinalityDelay(prevEpoch, finalizedEpoch) > params.BeaconConfig().MinEpochsToInactivityPenalty
}

// FinalityDelay returns the finality delay using the beacon state.
//
// Spec code:
// def get_finality_delay(state: BeaconState) -> uint64:
//
//	return get_previous_epoch(state) - state.finalized_checkpoint.epoch
func FinalityDelay(prevEpoch, finalizedEpoch types.Epoch) types.Epoch {
	return prevEpoch - finalizedEpoch
}
