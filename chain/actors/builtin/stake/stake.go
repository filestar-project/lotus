package stake

import (
	"errors"
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/go-state-types/cbor"
	builtin2 "github.com/filecoin-project/specs-actors/v2/actors/builtin"
	builtin3 "github.com/filecoin-project/specs-actors/v3/actors/builtin"
	stake3 "github.com/filecoin-project/specs-actors/v3/actors/builtin/stake"

	"github.com/ipfs/go-cid"
	"golang.org/x/xerrors"

	"github.com/filecoin-project/lotus/chain/actors/adt"
	"github.com/filecoin-project/lotus/chain/actors/builtin"
	"github.com/filecoin-project/lotus/chain/types"
)

func init() {
	builtin.RegisterActorState(builtin2.StakeActorCodeID, func(store adt.Store, root cid.Cid) (cbor.Marshaler, error) {
		return load2(store, root)
	})
	builtin.RegisterActorState(builtin3.StakeActorCodeID, func(store adt.Store, root cid.Cid) (cbor.Marshaler, error) {
		return load3(store, root)
	})
}

var (
	Address                    = builtin3.StakeActorAddr
	Methods                    = builtin3.MethodsStake
	UnsupportedAddressProtocol = errors.New("address must be ID format")
)

type LockedPrincipal = stake3.LockedPrincipal
type VestingFund = stake3.VestingFund

type State interface {
	cbor.Marshaler
	GetInfo() (*StakeInfo, error)
	StakerPower(staker address.Address) (abi.StakePower, error)
	StakerLockedPrincipalList(staker address.Address) ([]LockedPrincipal, error)
	StakerLockedPrincipal(staker address.Address) (abi.TokenAmount, error)
	StakerAvailablePrincipal(staker address.Address) (abi.TokenAmount, error)
	StakerVestingRewardList(staker address.Address) ([]VestingFund, error)
	StakerVestingReward(staker address.Address) (abi.TokenAmount, error)
	StakerAvailableReward(staker address.Address) (abi.TokenAmount, error)
}

type StakeInfo struct {
	TotalStakePower       abi.StakePower
	MaturePeriod          abi.ChainEpoch
	RoundPeriod           abi.ChainEpoch
	PrincipalLockDuration abi.ChainEpoch
	MinDepositAmount      abi.TokenAmount
	MaxRewardPerRound     abi.TokenAmount
	InflationFactor       big.Int
	LastRoundReward       abi.TokenAmount
	NextRoundEpoch        abi.ChainEpoch
}

func Load(store adt.Store, act *types.Actor) (st State, err error) {
	switch act.Code {
	case builtin2.StakeActorCodeID:
		return load2(store, act.Head)
	case builtin3.StakeActorCodeID:
		return load3(store, act.Head)
	}
	return nil, xerrors.Errorf("unknown actor code %s", act.Code)
}