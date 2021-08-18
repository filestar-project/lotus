package stake

import (
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/lotus/chain/actors/adt"

	"github.com/ipfs/go-cid"

	stake2 "github.com/filecoin-project/specs-actors/v2/actors/builtin/stake"
	adt2 "github.com/filecoin-project/specs-actors/v2/actors/util/adt"
)

func load2(store adt.Store, root cid.Cid) (State, error) {
	out := state2{store: store}
	err := store.Get(store.Context(), root, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

type state2 struct {
	stake2.State
	store adt.Store
}

var _ State = (*state2)(nil)

func (s *state2) GetInfo() (*StakeInfo, error) {
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

func (s *state2) StakerPower(staker2 address.Address) (abi.StakePower, error) {
	power := abi.NewStakePower(0)
	if staker2.Protocol() != address.ID {
		return power, UnsupportedAddressProtocol
	}
	stakePowerMap, err := adt2.AsMap(s.store, s.State.StakePowerMap)
	if err != nil {
		return power, err
	}
	_, err = stakePowerMap.Get(abi.AddrKey(staker2), &power)
	return power, err
}

func (s *state2) StakerLockedPrincipalList(staker2 address.Address) ([]LockedPrincipal, error) {
	if staker2.Protocol() != address.ID {
		return nil, UnsupportedAddressProtocol
	}
	lockedPrincipalMap, err := adt2.AsMap(s.store, s.State.LockedPrincipalMap)
	if err != nil {
		return nil, err
	}
	lockedPrincipals, found, err := s.State.LoadLockedPrincipals(s.store, lockedPrincipalMap, staker2)
	if err != nil || !found {
		return nil, err
	}
	var lockedPrincipal []LockedPrincipal
	for _, value := range lockedPrincipals.Data {
		lpl := LockedPrincipal{
			value.Amount,
			value.Epoch,
		}
		lockedPrincipal = append(lockedPrincipal, lpl)
	}

	return lockedPrincipal, nil
}

func (s *state2) StakerLockedPrincipal(staker2 address.Address) (abi.TokenAmount, error) {
	lockedPrincipal := abi.NewTokenAmount(0)
	if staker2.Protocol() != address.ID {
		return lockedPrincipal, UnsupportedAddressProtocol
	}
	list, err := s.StakerLockedPrincipalList(staker2)
	if err != nil {
		return lockedPrincipal, err
	}
	for _, item := range list {
		lockedPrincipal = big.Add(lockedPrincipal, item.Amount)
	}
	return lockedPrincipal, nil
}

func (s *state2) StakerAvailablePrincipal(staker2 address.Address) (abi.TokenAmount, error) {
	availablePrincipal := abi.NewTokenAmount(0)
	if staker2.Protocol() != address.ID {
		return availablePrincipal, UnsupportedAddressProtocol
	}
	availablePrincipalMap, err := adt2.AsMap(s.store, s.State.AvailablePrincipalMap)
	if err != nil {
		return availablePrincipal, err
	}
	_, err = availablePrincipalMap.Get(abi.AddrKey(staker2), &availablePrincipal)
	return availablePrincipal, err
}

func (s *state2) StakerVestingRewardList(staker2 address.Address) ([]VestingFund, error) {
	if staker2.Protocol() != address.ID {
		return nil, UnsupportedAddressProtocol
	}
	vestingRewardMap, err := adt2.AsMap(s.store, s.State.VestingRewardMap)
	if err != nil {
		return nil, err
	}
	vestingFunds, found, err := s.State.LoadVestingFunds(s.store, vestingRewardMap, staker2)
	if err != nil || !found {
		return nil, err
	}
	return vestingFunds.Funds, nil
}

func (s *state2) StakerVestingReward(staker2 address.Address) (abi.TokenAmount, error) {
	vestingReward := abi.NewTokenAmount(0)
	if staker2.Protocol() != address.ID {
		return vestingReward, UnsupportedAddressProtocol
	}
	list, err := s.StakerVestingRewardList(staker2)
	if err != nil {
		return vestingReward, err
	}
	for _, item := range list {
		vestingReward = big.Add(vestingReward, item.Amount)
	}
	return vestingReward, nil
}

func (s *state2) StakerAvailableReward(staker2 address.Address) (abi.TokenAmount, error) {
	availableReward := abi.NewTokenAmount(0)
	if staker2.Protocol() != address.ID {
		return availableReward, UnsupportedAddressProtocol
	}
	availableRewardMap, err := adt2.AsMap(s.store, s.State.AvailableRewardMap)
	if err != nil {
		return availableReward, err
	}
	_, err = availableRewardMap.Get(abi.AddrKey(staker2), &availableReward)
	return availableReward, err
}

