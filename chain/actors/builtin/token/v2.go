package token

//import (
//	addr "github.com/filecoin-project/go-address"
//	"github.com/filecoin-project/go-state-types/abi"
//	"github.com/filecoin-project/go-state-types/big"
//	"github.com/filecoin-project/lotus/chain/actors/adt"
//
//	token2 "github.com/filecoin-project/specs-actors/v2/actors/builtin/token"
//	adt2 "github.com/filecoin-project/specs-actors/v2/actors/util/adt"
//
//	"github.com/ipfs/go-cid"
//)
//
//func load2(store adt.Store, root cid.Cid) (State, error) {
//	out := state2{store: store}
//	err := store.Get(store.Context(), root, &out)
//	if err != nil {
//		return nil, err
//	}
//	return &out, nil
//}
//
//type state2 struct {
//	token2.State
//	store adt.Store
//}
//
//var _ State = (*state2)(nil)
//
//func (s *state2) GetInfo() (*TokenStateInfo, error) {
//	return &TokenStateInfo{
//		Nonce: s.State.Nonce,
//	}, nil
//}
//
//func (s *state2) TokenCreators() ([]addr.Address, error) {
//	var tokenCreators []addr.Address
//	var idx = big.NewInt(1)
//	var tokenCreator addr.Address
//
//	tokenCreatorsArray, err := adt2.AsArray(s.store, s.State.Creators)
//	if err != nil {
//		return nil, err
//	}
//
//	for idx.LessThanEqual(s.State.Nonce) {
//		_, err = tokenCreatorsArray.Get(idx.Uint64(), &tokenCreator)
//		if err != nil {
//			return tokenCreators, err
//		}
//		tokenCreators = append(tokenCreators, tokenCreator)
//		idx = big.Add(idx, big.NewInt(1))
//	}
//
//	return tokenCreators, nil
//}
//
//func (s *state2) TokenCreatorByTokenID(tokenID big.Int) (addr.Address, error) {
//	var tokenCreator addr.Address
//	if tokenID.Nil() || tokenID.LessThanEqual(big.Zero()) || tokenID.GreaterThan(s.State.Nonce) {
//		return tokenCreator, UnsupportedTokenIDValue
//	}
//	tokenCreatorsArray, err := adt2.AsArray(s.store, s.State.Creators)
//	if err != nil {
//		return tokenCreator, err
//	}
//
//	_, err = tokenCreatorsArray.Get(tokenID.Uint64(), &tokenCreator)
//	if err != nil {
//		return tokenCreator, err
//	}
//
//	return tokenCreator, nil
//}
//
//func (s *state2) TokenIDsByCreators(creator addr.Address) ([]big.Int, error) {
//	var tokenIDs []big.Int
//	if creator.Protocol() != addr.ID {
//		return tokenIDs, UnsupportedAddressProtocol
//	}
//	var idx = big.NewInt(1)
//	var tokenCreator addr.Address
//
//	tokenCreatorsArray, err := adt2.AsArray(s.store, s.State.Creators)
//	if err != nil {
//		return tokenIDs, err
//	}
//
//	for idx.LessThanEqual(s.State.Nonce) {
//		_, err = tokenCreatorsArray.Get(idx.Uint64(), &tokenCreator)
//		if err != nil {
//			return tokenIDs, err
//		}
//		if tokenCreator == creator {
//			tokenIDs = append(tokenIDs, idx)
//		}
//		idx = big.Add(idx, big.NewInt(1))
//	}
//	return tokenIDs, nil
//}
//
//func (s *state2) TokenURIByTokenID(tokenID big.Int) (string, error) {
//	if tokenID.Nil() || tokenID.LessThanEqual(big.Zero()) || tokenID.GreaterThan(s.State.Nonce) {
//		return "", UnsupportedTokenIDValue
//	}
//	urisArray, err := adt2.AsArray(s.store, s.State.URIs)
//	if err != nil {
//		return "", err
//	}
//	var tokenUri token2.TokenURI
//	_, err = urisArray.Get(tokenID.Uint64(), &tokenUri)
//	if err != nil {
//		return "", err
//	}
//
//	return tokenUri.TokenURI, nil
//}
//
//func (s *state2) TokenBalanceByTokenID(tokenID big.Int) ([]*TokenBalanceInfoByTokenID, error) {
//	var tokenBalanceInfoByTokenID []*TokenBalanceInfoByTokenID
//	if tokenID.Nil() || tokenID.LessThanEqual(big.Zero()) || tokenID.GreaterThan(s.State.Nonce) {
//		return tokenBalanceInfoByTokenID, UnsupportedTokenIDValue
//	}
//	balanceArray, err := adt2.AsArray(s.store, s.State.Balances)
//	if err != nil {
//		return tokenBalanceInfoByTokenID, err
//	}
//
//	addrTokenAmountMap, found, err := s.State.LoadAddrTokenAmountMap(s.store, balanceArray, tokenID)
//	if err != nil {
//		return tokenBalanceInfoByTokenID, err
//	}
//	if !found {
//		return nil, nil
//	}
//
//	tokenAmountMap, err := adt2.AsMap(s.store, addrTokenAmountMap.AddrTokenAmountMap)
//	if err != nil {
//		return tokenBalanceInfoByTokenID, err
//	}
//
//	owners, err := tokenAmountMap.CollectKeys()
//
//	for _, key := range owners {
//		address, err := addr.NewFromBytes([]byte(key))
//		if err != nil {
//			return tokenBalanceInfoByTokenID, err
//		}
//		tokenAmount, found, err := s.State.LoadAddrTokenAmount(tokenAmountMap, address)
//		if err != nil {
//			return tokenBalanceInfoByTokenID, err
//		}
//
//		if !found {
//			tokenBalanceInfoByTokenID = append(tokenBalanceInfoByTokenID, nil)
//			continue
//		}
//
//		tokenBalanceInfoByTokenID = append(tokenBalanceInfoByTokenID, &TokenBalanceInfoByTokenID{Owner: address, Balance: tokenAmount})
//
//	}
//
//	return tokenBalanceInfoByTokenID, nil
//}
//
//func (s *state2) TokenBalanceByAddr(owner addr.Address) ([]*TokenBalanceInfoByAddr, error) {
//	var tokenBalanceInfoByAddr []*TokenBalanceInfoByAddr
//	if owner.Protocol() != addr.ID {
//		return tokenBalanceInfoByAddr, UnsupportedAddressProtocol
//	}
//
//	balanceArray, err := adt2.AsArray(s.store, s.State.Balances)
//	if err != nil {
//		return nil, err
//	}
//
//	var idx = big.NewInt(1)
//
//	for idx.LessThanEqual(s.State.Nonce) {
//		addrTokenAmountMap, found, err := s.State.LoadAddrTokenAmountMap(s.store, balanceArray, idx)
//		if err != nil {
//			return tokenBalanceInfoByAddr, err
//		}
//		if !found {
//			tokenBalanceInfoByAddr = append(tokenBalanceInfoByAddr, nil)
//			idx = big.Add(idx, big.NewInt(1))
//			continue
//		}
//		tokenAmountMap, err := adt2.AsMap(s.store, addrTokenAmountMap.AddrTokenAmountMap)
//		if err != nil {
//			return tokenBalanceInfoByAddr, err
//		}
//
//		tokenAmount, found, err := s.State.LoadAddrTokenAmount(tokenAmountMap, owner)
//		if err != nil {
//			return tokenBalanceInfoByAddr, err
//		}
//		if !found {
//			tokenBalanceInfoByAddr = append(tokenBalanceInfoByAddr, nil)
//			idx = big.Add(idx, big.NewInt(1))
//			continue
//		}
//		tokenBalanceInfoByAddr = append(tokenBalanceInfoByAddr, &TokenBalanceInfoByAddr{TokenID: idx, Balance: tokenAmount})
//
//		idx = big.Add(idx, big.NewInt(1))
//	}
//
//	return tokenBalanceInfoByAddr, nil
//}
//
//func (s *state2) TokenBalanceByTokenIDAndAddr(tokenID big.Int, owner addr.Address) (abi.TokenAmount, error) {
//	if tokenID.Nil() || tokenID.LessThanEqual(big.Zero()) || tokenID.GreaterThan(s.State.Nonce) {
//		return big.Zero(), UnsupportedTokenIDValue
//	}
//	if owner.Protocol() != addr.ID {
//		return big.Zero(), UnsupportedAddressProtocol
//	}
//
//	balanceArray, err := adt2.AsArray(s.store, s.State.Balances)
//	if err != nil {
//		return big.Zero(), err
//	}
//
//	addrTokenAmountMap, found, err := s.State.LoadAddrTokenAmountMap(s.store, balanceArray, tokenID)
//	if err != nil || !found {
//		return big.Zero(), err
//	}
//
//	tokenAmountMap, err := adt2.AsMap(s.store, addrTokenAmountMap.AddrTokenAmountMap)
//	if err != nil {
//		return big.Zero(), err
//	}
//
//	tokenAmount, found, err := s.State.LoadAddrTokenAmount(tokenAmountMap, owner)
//	if err != nil || !found {
//		return big.Zero(), err
//	}
//
//	return tokenAmount, nil
//}
//
//func (s *state2) TokenBalanceByTokenIDsAndAddrs(tokenIDs []big.Int, owners []addr.Address) ([]abi.TokenAmount, error) {
//
//	var tokenAmounts []abi.TokenAmount
//	balanceArray, err := adt2.AsArray(s.store, s.State.Balances
//	if err != nil {
//		return tokenAmounts, err
//	}
//
//	for idx, _ := range tokenIDs {
//		if tokenIDs[idx].Nil() || tokenIDs[idx].LessThanEqual(big.Zero()) || tokenIDs[idx].GreaterThan(s.State.Nonce) {
//			return tokenAmounts, UnsupportedTokenIDValue
//		}
//		if owners[idx].Protocol() != addr.ID {
//			return tokenAmounts, UnsupportedAddressProtocol
//		}
//		addrTokenAmountMap, found, err := s.State.LoadAddrTokenAmountMap(s.store, balanceArray, tokenIDs[idx])
//		if err != nil {
//			return tokenAmounts, err
//		}
//		if !found {
//			tokenAmounts = append(tokenAmounts, big.Zero())
//			continue
//		}
//
//		tokenAmountMap, err := adt2.AsMap(s.store, addrTokenAmountMap.AddrTokenAmountMap)
//		if err != nil {
//			return tokenAmounts, err
//		}
//
//		tokenAmount, found, err := s.State.LoadAddrTokenAmount(tokenAmountMap, owners[idx])
//		if err != nil {
//			return tokenAmounts, err
//		}
//		if !found {
//			tokenAmounts = append(tokenAmounts, big.Zero())
//			continue
//		}
//		tokenAmounts = append(tokenAmounts, tokenAmount)
//	}
//	return tokenAmounts, nil
//}
//
//func (s *state2) TokenIsAllApproved(addrFrom addr.Address, addrTo addr.Address) (bool, error) {
//	if addrFrom.Protocol() != addr.ID || addrTo.Protocol() != addr.ID {
//		return false, UnsupportedAddressProtocol
//	}
//
//	isAllApproveMap, err := adt2.AsMap(s.store, s.State.Approves)
//	if err != nil {
//		return false, err
//	}
//
//	addrApproveMap, found, err := s.State.LoadAddrApproveMap(s.store, isAllApproveMap, addrFrom)
//	if err != nil || !found {
//		return false, err
//	}
//
//	addrApprovemap, err := adt2.AsMap(s.store, addrApproveMap.AddrApproveMap)
//	if err != nil {
//		return false, err
//	}
//
//	addrApprove, found, err := s.State.LoadAddrApprove(addrApprovemap, addrTo)
//	if err != nil || !found {
//		return false, err
//	}
//
//	return addrApprove, nil
//}
//
