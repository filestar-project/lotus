package token

import (
	"errors"
	addr "github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/go-state-types/cbor"
	"github.com/filecoin-project/lotus/chain/actors/adt"
	"github.com/filecoin-project/lotus/chain/actors/builtin"
	"github.com/filecoin-project/lotus/chain/types"
	builtin3 "github.com/filecoin-project/specs-actors/v3/actors/builtin"
	"github.com/ipfs/go-cid"
	"golang.org/x/xerrors"
)

func init() {
	//builtin.RegisterActorState(builtin2.TokenActorCodeID, func(store adt.Store, root cid.Cid) (cbor.Marshaler, error) {
	//	return load2(store, root)
	//})
	builtin.RegisterActorState(builtin3.TokenActorCodeID, func(store adt.Store, root cid.Cid) (cbor.Marshaler, error) {
		return load3(store, root)
	})
}

var (
	Address                    = builtin3.TokenActorAddr
	Methods                    = builtin3.MethodsToken
	UnsupportedAddressProtocol = errors.New("address must be ID format")
	UnsupportedTokenIDValue = errors.New("tokenID must be available")
)

type TokenStateInfo struct {
	Nonce 	big.Int
}

type TokenBalanceInfoByTokenID struct {
	Owner	addr.Address
	Balance abi.TokenAmount
}

type TokenBalanceInfoByAddr struct {
	TokenID big.Int
	Balance abi.TokenAmount
}

type State interface {
	cbor.Marshaler
	GetInfo() (*TokenStateInfo, error)
	TokenCreators() ([]addr.Address, error)
	TokenCreatorByTokenID(tokenID big.Int) (addr.Address, error)
	TokenIDsByCreators(creator addr.Address) ([]big.Int, error)
	TokenURIByTokenID(tokenID big.Int) (string, error)
	TokenBalanceByTokenID(tokenID big.Int) ([]*TokenBalanceInfoByTokenID, error)
	TokenBalanceByAddr(owner addr.Address) ([]*TokenBalanceInfoByAddr, error)
	TokenBalanceByTokenIDAndAddr(tokenID big.Int, owner addr.Address) (abi.TokenAmount, error)
	TokenBalanceByTokenIDsAndAddrs(tokenID []big.Int, owner []addr.Address) ([]abi.TokenAmount, error)
	TokenIsAllApproved(addrFrom addr.Address, addrTo addr.Address) (bool, error)
}

func Load(store adt.Store, act *types.Actor) (st State, err error) {
	switch act.Code {
	//case builtin2.TokenActorCodeID:
	//	return load2(store, act.Head)
	case builtin3.TokenActorCodeID:
		return load3(store, act.Head)
	}
	return nil, xerrors.Errorf("unknown actor code %s", act.Code)
}