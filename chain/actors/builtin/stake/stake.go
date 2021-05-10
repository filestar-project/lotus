package stake

import (
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/go-state-types/cbor"
	builtin2 "github.com/filecoin-project/specs-actors/v2/actors/builtin"
	stake2 "github.com/filecoin-project/specs-actors/v2/actors/builtin/stake"
	adt2 "github.com/filecoin-project/specs-actors/v2/actors/util/adt"

	"github.com/ipfs/go-cid"
	"golang.org/x/xerrors"

	"github.com/filecoin-project/lotus/chain/actors/adt"
	"github.com/filecoin-project/lotus/chain/actors/builtin"
	"github.com/filecoin-project/lotus/chain/types"
)

func init() {
	builtin.RegisterActorState(builtin2.StakeActorCodeID, func(store adt.Store, root cid.Cid) (cbor.Marshaler, error) {
		return load(store, root)
	})
}

var (
	Address = builtin2.StakeActorAddr
	Methods = builtin2.MethodsStake
)

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

type LockedPrincipal = stake2.LockedPrincipal
type VestingFund = stake2.VestingFund

func Load(store adt.Store, act *types.Actor) (st State, err error) {
	switch act.Code {
	case builtin2.StakeActorCodeID:
		return load(store, act.Head)
	}
	return nil, xerrors.Errorf("unknown actor code %s", act.Code)
}

func load(store adt.Store, root cid.Cid) (State, error) {
	out := state{store: store}
	err := store.Get(store.Context(), root, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

type state struct {
	stake2.State
	store adt.Store
}

var _ State = (*state)(nil)

func (s *state) GetInfo() (*StakeInfo, error) {
	return &StakeInfo{
		s.State.TotalStakePower,
		s.State.MaturePeriod,
		s.State.RoundPeriod,
		s.State.PrincipalLockDuration,
		s.State.MinDepositAmount,
		s.State.MaxRewardPerRound,
		s.State.InflationFactor,
		s.State.LastRoundReward,
		s.State.NextRoundEpoch,
	}, nil
}

func (s *state) StakerPower(staker address.Address) (abi.StakePower, error) {
	power := abi.NewStakePower(0)
	stakePowerMap, err := adt2.AsMap(s.store, s.State.StakePowerMap)
	if err != nil {
		return power, err
	}
	_, err = stakePowerMap.Get(abi.AddrKey(staker), &power)
	return power, err
}

func (s *state) StakerLockedPrincipalList(staker address.Address) ([]LockedPrincipal, error) {
	lockedPrincipalMap, err := adt2.AsMap(s.store, s.State.LockedPrincipalMap)
	if err != nil {
		return nil, err
	}
	lockedPrincipals, found, err := s.State.LoadLockedPrincipals(s.store, lockedPrincipalMap, staker)
	if err != nil || !found {
		return nil, err
	}
	return lockedPrincipals.Data, nil
}

func (s *state) StakerLockedPrincipal(staker address.Address) (abi.TokenAmount, error) {
	lockedPrincipal := abi.NewTokenAmount(0)
	list, err := s.StakerLockedPrincipalList(staker)
	if err != nil {
		return lockedPrincipal, err
	}
	for _, item := range list {
		lockedPrincipal = big.Add(lockedPrincipal, item.Amount)
	}
	return lockedPrincipal, nil
}

func (s *state) StakerAvailablePrincipal(staker address.Address) (abi.TokenAmount, error) {
	availablePrincipal := abi.NewTokenAmount(0)
	availablePrincipalMap, err := adt2.AsMap(s.store, s.State.AvailablePrincipalMap)
	if err != nil {
		return availablePrincipal, err
	}
	_, err = availablePrincipalMap.Get(abi.AddrKey(staker), &availablePrincipal)
	return availablePrincipal, err
}

func (s *state) StakerVestingRewardList(staker address.Address) ([]VestingFund, error) {
	vestingRewardMap, err := adt2.AsMap(s.store, s.State.VestingRewardMap)
	if err != nil {
		return nil, err
	}
	vestingFunds, found, err := s.State.LoadVestingFunds(s.store, vestingRewardMap, staker)
	if err != nil || !found {
		return nil, err
	}
	return vestingFunds.Funds, nil
}

func (s *state) StakerVestingReward(staker address.Address) (abi.TokenAmount, error) {
	vestingReward := abi.NewTokenAmount(0)
	list, err := s.StakerVestingRewardList(staker)
	if err != nil {
		return vestingReward, err
	}
	for _, item := range list {
		vestingReward = big.Add(vestingReward, item.Amount)
	}
	return vestingReward, nil
}

func (s *state) StakerAvailableReward(staker address.Address) (abi.TokenAmount, error) {
	availableReward := abi.NewTokenAmount(0)
	availableRewardMap, err := adt2.AsMap(s.store, s.State.AvailableRewardMap)
	if err != nil {
		return availableReward, err
	}
	_, err = availableRewardMap.Get(abi.AddrKey(staker), &availableReward)
	return availableReward, err
}
