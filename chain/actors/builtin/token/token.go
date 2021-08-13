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
	token3 "github.com/filecoin-project/specs-actors/v3/actors/builtin/token"
	adt3 "github.com/filecoin-project/specs-actors/v3/actors/util/adt"
	"github.com/ipfs/go-cid"
	"golang.org/x/xerrors"
)

func init() {
	builtin.RegisterActorState(builtin3.TokenActorCodeID, func(store adt.Store, root cid.Cid) (cbor.Marshaler, error) {
		return load(store, root)
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
	case builtin3.TokenActorCodeID:
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
	token3.State
	store adt.Store
}

var _ State = (*state)(nil)

func (s *state) GetInfo() (*TokenStateInfo, error) {
	return &TokenStateInfo{
		Nonce: s.State.Nonce,
	}, nil
}

func (s *state) TokenCreators() ([]addr.Address, error) {
	var tokenCreators []addr.Address
	var idx = big.NewInt(1)
	var tokenCreator addr.Address

	tokenCreatorsArray, err := adt3.AsArray(s.store, s.State.Creators, token3.LaneStatesAmtBitwidth)
	if err != nil {
		return nil, err
	}

	for idx.LessThanEqual(s.State.Nonce) {
		_, err = tokenCreatorsArray.Get(idx.Uint64(), &tokenCreator)
		if err != nil {
			return tokenCreators, err
		}
		tokenCreators = append(tokenCreators, tokenCreator)
		idx = big.Add(idx, big.NewInt(1))
	}

	return tokenCreators, nil
}

func (s *state) TokenCreatorByTokenID(tokenID big.Int) (addr.Address, error) {
	var tokenCreator addr.Address
	if tokenID.Nil() || tokenID.LessThanEqual(big.Zero()) || tokenID.GreaterThan(s.State.Nonce) {
		return tokenCreator, UnsupportedTokenIDValue
	}
	tokenCreatorsArray, err := adt3.AsArray(s.store, s.State.Creators, token3.LaneStatesAmtBitwidth)
	if err != nil {
		return tokenCreator, err
	}

	_, err = tokenCreatorsArray.Get(tokenID.Uint64(), &tokenCreator)
	if err != nil {
		return tokenCreator, err
	}

	return tokenCreator, nil
}

func (s *state) TokenIDsByCreators(creator addr.Address) ([]big.Int, error) {
	var tokenIDs []big.Int
	if creator.Protocol() != addr.ID {
		return tokenIDs, UnsupportedAddressProtocol
	}
	var idx = big.NewInt(1)
	var tokenCreator addr.Address

	tokenCreatorsArray, err := adt3.AsArray(s.store, s.State.Creators, token3.LaneStatesAmtBitwidth)
	if err != nil {
		return tokenIDs, err
	}

	for idx.LessThanEqual(s.State.Nonce) {
		_, err = tokenCreatorsArray.Get(idx.Uint64(), &tokenCreator)
		if err != nil {
			return tokenIDs, err
		}
		if tokenCreator == creator {
			tokenIDs = append(tokenIDs, idx)
		}
		idx = big.Add(idx, big.NewInt(1))
	}
	return tokenIDs, nil
}

func (s *state) TokenURIByTokenID(tokenID big.Int) (string, error) {
	if tokenID.Nil() || tokenID.LessThanEqual(big.Zero()) || tokenID.GreaterThan(s.State.Nonce) {
		return "", UnsupportedTokenIDValue
	}
	urisArray, err := adt3.AsArray(s.store, s.State.URIs, token3.LaneStatesAmtBitwidth)
	if err != nil {
		return "", err
	}
	var tokenUri token3.TokenURI
	_, err = urisArray.Get(tokenID.Uint64(), &tokenUri)
	if err != nil {
		return "", err
	}

	return tokenUri.TokenURI, nil
}

func (s *state) TokenBalanceByTokenID(tokenID big.Int) ([]*TokenBalanceInfoByTokenID, error) {
	var tokenBalanceInfoByTokenID []*TokenBalanceInfoByTokenID
	if tokenID.Nil() || tokenID.LessThanEqual(big.Zero()) || tokenID.GreaterThan(s.State.Nonce) {
		return tokenBalanceInfoByTokenID, UnsupportedTokenIDValue
	}
	balanceArray, err := adt3.AsArray(s.store, s.State.Balances, token3.LaneStatesAmtBitwidth)
	if err != nil {
		return tokenBalanceInfoByTokenID, err
	}

	addrTokenAmountMap, found, err := s.State.LoadAddrTokenAmountMap(s.store, balanceArray, tokenID)
	if err != nil {
		return tokenBalanceInfoByTokenID, err
	}
	if !found {
		return nil, nil
	}

	tokenAmountMap, err := adt3.AsMap(s.store, addrTokenAmountMap.AddrTokenAmountMap, builtin3.DefaultHamtBitwidth)
	if err != nil {
		return tokenBalanceInfoByTokenID, err
	}

	owners, err := tokenAmountMap.CollectKeys()

	for _, key := range owners {
		address, err := addr.NewFromBytes([]byte(key))
		if err != nil {
			return tokenBalanceInfoByTokenID, err
		}
		tokenAmount, found, err := s.State.LoadAddrTokenAmount(tokenAmountMap, address)
		if err != nil {
			return tokenBalanceInfoByTokenID, err
		}

		if !found {
			tokenBalanceInfoByTokenID = append(tokenBalanceInfoByTokenID, nil)
			continue
		}

		tokenBalanceInfoByTokenID = append(tokenBalanceInfoByTokenID, &TokenBalanceInfoByTokenID{Owner: address, Balance: tokenAmount})

	}

	return tokenBalanceInfoByTokenID, nil
}

func (s *state) TokenBalanceByAddr(owner addr.Address) ([]*TokenBalanceInfoByAddr, error) {
	var tokenBalanceInfoByAddr []*TokenBalanceInfoByAddr
	if owner.Protocol() != addr.ID {
		return tokenBalanceInfoByAddr, UnsupportedAddressProtocol
	}

	balanceArray, err := adt3.AsArray(s.store, s.State.Balances, token3.LaneStatesAmtBitwidth)
	if err != nil {
		return nil, err
	}

	var idx = big.NewInt(1)

	for idx.LessThanEqual(s.State.Nonce) {
		addrTokenAmountMap, found, err := s.State.LoadAddrTokenAmountMap(s.store, balanceArray, idx)
		if err != nil {
			return tokenBalanceInfoByAddr, err
		}
		if !found {
			tokenBalanceInfoByAddr = append(tokenBalanceInfoByAddr, nil)
			idx = big.Add(idx, big.NewInt(1))
			continue
		}
		tokenAmountMap, err := adt3.AsMap(s.store, addrTokenAmountMap.AddrTokenAmountMap, builtin3.DefaultHamtBitwidth)
		if err != nil {
			return tokenBalanceInfoByAddr, err
		}

		tokenAmount, found, err := s.State.LoadAddrTokenAmount(tokenAmountMap, owner)
		if err != nil {
			return tokenBalanceInfoByAddr, err
		}
		if !found {
			tokenBalanceInfoByAddr = append(tokenBalanceInfoByAddr, nil)
			idx = big.Add(idx, big.NewInt(1))
			continue
		}
		tokenBalanceInfoByAddr = append(tokenBalanceInfoByAddr, &TokenBalanceInfoByAddr{TokenID: idx, Balance: tokenAmount})

		idx = big.Add(idx, big.NewInt(1))
	}

	return tokenBalanceInfoByAddr, nil
}

func (s *state) TokenBalanceByTokenIDAndAddr(tokenID big.Int, owner addr.Address) (abi.TokenAmount, error) {
	if tokenID.Nil() || tokenID.LessThanEqual(big.Zero()) || tokenID.GreaterThan(s.State.Nonce) {
		return big.Zero(), UnsupportedTokenIDValue
	}
	if owner.Protocol() != addr.ID {
		return big.Zero(), UnsupportedAddressProtocol
	}

	balanceArray, err := adt3.AsArray(s.store, s.State.Balances, token3.LaneStatesAmtBitwidth)
	if err != nil {
		return big.Zero(), err
	}

	addrTokenAmountMap, found, err := s.State.LoadAddrTokenAmountMap(s.store, balanceArray, tokenID)
	if err != nil || !found {
		return big.Zero(), err
	}

	tokenAmountMap, err := adt3.AsMap(s.store, addrTokenAmountMap.AddrTokenAmountMap, builtin3.DefaultHamtBitwidth)
	if err != nil {
		return big.Zero(), err
	}

	tokenAmount, found, err := s.State.LoadAddrTokenAmount(tokenAmountMap, owner)
	if err != nil || !found {
		return big.Zero(), err
	}

	return tokenAmount, nil
}

func (s *state) TokenBalanceByTokenIDsAndAddrs(tokenIDs []big.Int, owners []addr.Address) ([]abi.TokenAmount, error) {

	var tokenAmounts []abi.TokenAmount
	balanceArray, err := adt3.AsArray(s.store, s.State.Balances, token3.LaneStatesAmtBitwidth)
	if err != nil {
		return tokenAmounts, err
	}

	for idx, _ := range tokenIDs {
		if tokenIDs[idx].Nil() || tokenIDs[idx].LessThanEqual(big.Zero()) || tokenIDs[idx].GreaterThan(s.State.Nonce) {
			return tokenAmounts, UnsupportedTokenIDValue
		}
		if owners[idx].Protocol() != addr.ID {
			return tokenAmounts, UnsupportedAddressProtocol
		}
		addrTokenAmountMap, found, err := s.State.LoadAddrTokenAmountMap(s.store, balanceArray, tokenIDs[idx])
		if err != nil {
			return tokenAmounts, err
		}
		if !found {
			tokenAmounts = append(tokenAmounts, big.Zero())
			continue
		}

		tokenAmountMap, err := adt3.AsMap(s.store, addrTokenAmountMap.AddrTokenAmountMap, builtin3.DefaultHamtBitwidth)
		if err != nil {
			return tokenAmounts, err
		}

		tokenAmount, found, err := s.State.LoadAddrTokenAmount(tokenAmountMap, owners[idx])
		if err != nil {
			return tokenAmounts, err
		}
		if !found {
			tokenAmounts = append(tokenAmounts, big.Zero())
			continue
		}
		tokenAmounts = append(tokenAmounts, tokenAmount)
	}
	return tokenAmounts, nil
}

func (s *state) TokenIsAllApproved(addrFrom addr.Address, addrTo addr.Address) (bool, error) {
	if addrFrom.Protocol() != addr.ID || addrTo.Protocol() != addr.ID {
		return false, UnsupportedAddressProtocol
	}

	isAllApproveMap, err := adt3.AsMap(s.store, s.State.Approves, builtin3.DefaultHamtBitwidth)
	if err != nil {
		return false, err
	}

	addrApproveMap, found, err := s.State.LoadAddrApproveMap(s.store, isAllApproveMap, addrFrom)
	if err != nil || !found {
		return false, err
	}

	addrApprovemap, err := adt3.AsMap(s.store, addrApproveMap.AddrApproveMap, builtin3.DefaultHamtBitwidth)
	if err != nil {
		return false, err
	}

	addrApprove, found, err := s.State.LoadAddrApprove(addrApprovemap, addrTo)
	if err != nil || !found {
		return false, err
	}

	return addrApprove, nil
}

