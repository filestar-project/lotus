package stake

import (
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/lotus/chain/actors/adt"

	"github.com/ipfs/go-cid"

	builtin3 "github.com/filecoin-project/specs-actors/v3/actors/builtin"
	stake3 "github.com/filecoin-project/specs-actors/v3/actors/builtin/stake"
	adt3 "github.com/filecoin-project/specs-actors/v3/actors/util/adt"
)

func load3(store adt.Store, root cid.Cid) (State, error) {
	out := state3{store: store}
	err := store.Get(store.Context(), root, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

type state3 struct {
	stake3.State
	store adt.Store
}

var _ State = (*state3)(nil)

func (s *state3) GetInfo() (*StakeInfo, error) {
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

func (s *state3) StakerPower(staker3 address.Address) (abi.StakePower, error) {
	power := abi.NewStakePower(0)
	if staker3.Protocol() != address.ID {
		return power, UnsupportedAddressProtocol
	}
	stakePowerMap, err := adt3.AsMap(s.store, s.State.StakePowerMap, builtin3.DefaultHamtBitwidth)
	if err != nil {
		return power, err
	}
	_, err = stakePowerMap.Get(abi.AddrKey(staker3), &power)
	return power, err
}

func (s *state3) StakerLockedPrincipalList(staker3 address.Address) ([]LockedPrincipal, error) {
	if staker3.Protocol() != address.ID {
		return nil, UnsupportedAddressProtocol
	}
	lockedPrincipalMap, err := adt3.AsMap(s.store, s.State.LockedPrincipalMap, builtin3.DefaultHamtBitwidth)
	if err != nil {
		return nil, err
	}
	lockedPrincipals, found, err := s.State.LoadLockedPrincipals(s.store, lockedPrincipalMap, staker3)
	if err != nil || !found {
		return nil, err
	}
	return lockedPrincipals.Data, nil
}

func (s *state3) StakerLockedPrincipal(staker3 address.Address) (abi.TokenAmount, error) {
	lockedPrincipal := abi.NewTokenAmount(0)
	if staker3.Protocol() != address.ID {
		return lockedPrincipal, UnsupportedAddressProtocol
	}
	list, err := s.StakerLockedPrincipalList(staker3)
	if err != nil {
		return lockedPrincipal, err
	}
	for _, item := range list {
		lockedPrincipal = big.Add(lockedPrincipal, item.Amount)
	}
	return lockedPrincipal, nil
}

func (s *state3) StakerAvailablePrincipal(staker3 address.Address) (abi.TokenAmount, error) {
	availablePrincipal := abi.NewTokenAmount(0)
	if staker3.Protocol() != address.ID {
		return availablePrincipal, UnsupportedAddressProtocol
	}
	availablePrincipalMap, err := adt3.AsMap(s.store, s.State.AvailablePrincipalMap, builtin3.DefaultHamtBitwidth)
	if err != nil {
		return availablePrincipal, err
	}
	_, err = availablePrincipalMap.Get(abi.AddrKey(staker3), &availablePrincipal)
	return availablePrincipal, err
}

func (s *state3) StakerVestingRewardList(staker3 address.Address) ([]VestingFund, error) {
	if staker3.Protocol() != address.ID {
		return nil, UnsupportedAddressProtocol
	}
	vestingRewardMap, err := adt3.AsMap(s.store, s.State.VestingRewardMap, builtin3.DefaultHamtBitwidth)
	if err != nil {
		return nil, err
	}
	vestingFunds, found, err := s.State.LoadVestingFunds(s.store, vestingRewardMap, staker3)
	if err != nil || !found {
		return nil, err
	}
	return vestingFunds.Funds, nil
}

func (s *state3) StakerVestingReward(staker3 address.Address) (abi.TokenAmount, error) {
	vestingReward := abi.NewTokenAmount(0)
	if staker3.Protocol() != address.ID {
		return vestingReward, UnsupportedAddressProtocol
	}
	list, err := s.StakerVestingRewardList(staker3)
	if err != nil {
		return vestingReward, err
	}
	for _, item := range list {
		vestingReward = big.Add(vestingReward, item.Amount)
	}
	return vestingReward, nil
}

func (s *state3) StakerAvailableReward(staker3 address.Address) (abi.TokenAmount, error) {
	availableReward := abi.NewTokenAmount(0)
	if staker3.Protocol() != address.ID {
		return availableReward, UnsupportedAddressProtocol
	}
	availableRewardMap, err := adt3.AsMap(s.store, s.State.AvailableRewardMap, builtin3.DefaultHamtBitwidth)
	if err != nil {
		return availableReward, err
	}
	_, err = availableRewardMap.Get(abi.AddrKey(staker3), &availableReward)
	return availableReward, err
}

