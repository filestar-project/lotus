package genesis

import (
	"context"
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/lotus/chain/types"
	bstore "github.com/filecoin-project/lotus/lib/blockstore"
	"github.com/filecoin-project/specs-actors/v2/actors/builtin"
	"github.com/filecoin-project/specs-actors/v2/actors/builtin/stake"
	"github.com/filecoin-project/specs-actors/v2/actors/util/adt"
	cbor "github.com/ipfs/go-ipld-cbor"
)

var RootStakeAdminID address.Address

func init() {
	idk, err := address.NewFromString("t080")
	if err != nil {
		panic(err)
	}

	RootStakeAdminID = idk
}

func SetupStakeActor(bs bstore.Blockstore, firstRoundEpoch abi.ChainEpoch) (*types.Actor, error) {
	store := adt.WrapStore(context.TODO(), cbor.NewCborStore(bs))

	emptyMap, err := adt.MakeEmptyMap(store).Root()
	if err != nil {
		return nil, err
	}

	params := &stake.ConstructorParams{
		RootKey:               RootStakeAdminID,
		MaturePeriod:          abi.ChainEpoch(12 * builtin.EpochsInHour),
		RoundPeriod:           abi.ChainEpoch(1 * builtin.EpochsInDay),
		PrincipalLockDuration: abi.ChainEpoch(90 * builtin.EpochsInDay),
		FirstRoundEpoch:       firstRoundEpoch,
		MinDepositAmount:      abi.TokenAmount(types.MustParseFIL("100 STAR")),
		MaxRewardPerRound:     abi.TokenAmount(types.MustParseFIL("10000 STAR")),
		InflationFactor:       big.NewInt(100),
	}
	// max_reward_per_round = Min(
	//		MaxRewardPerRound,
	//		TotalStakePower * InflationFactor / 10000
	// )
	sms := stake.ConstructState(params, emptyMap)

	stcid, err := store.Put(store.Context(), sms)
	if err != nil {
		return nil, err
	}

	return &types.Actor{
		Code:    builtin.StakeActorCodeID,
		Head:    stcid,
		Balance: types.NewInt(0),
	}, nil
}
