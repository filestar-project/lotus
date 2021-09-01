package genesis

import (
	"context"
	"github.com/filecoin-project/specs-actors/v3/actors/builtin"
	"github.com/filecoin-project/specs-actors/v3/actors/builtin/token"

	"github.com/filecoin-project/lotus/chain/types"
	bstore "github.com/filecoin-project/lotus/lib/blockstore"
	"github.com/filecoin-project/specs-actors/actors/util/adt"
	cbor "github.com/ipfs/go-ipld-cbor"

)

func SetupTokenActor(bs bstore.Blockstore) (*types.Actor, error) {

	store := adt.WrapStore(context.TODO(), cbor.NewCborStore(bs))

	tns, err:= token.ConstructState(store)
	if err != nil {
		return nil, err
	}

	stcid, err := store.Put(store.Context(), tns)
	if err != nil {
		return nil, err
	}

	return &types.Actor{
		Code:    builtin.TokenActorCodeID,
		Head:    stcid,
		Balance: types.NewInt(0),
	}, nil
}
