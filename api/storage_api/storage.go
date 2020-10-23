package storageapi

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/filecoin-project/go-state-types/abi"
	big2 "github.com/filecoin-project/go-state-types/big"
	"github.com/libp2p/go-libp2p-core/peer"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-multistore"
	lapi "github.com/filecoin-project/lotus/api"
	"github.com/filecoin-project/lotus/build"
	"github.com/filecoin-project/lotus/chain/actors/builtin/market"
	"github.com/filecoin-project/lotus/chain/types"
	marketevents "github.com/filecoin-project/lotus/markets/loggers"
	rt "github.com/filecoin-project/specs-actors/v2/actors/runtime"
	"github.com/ipfs/go-cid"
	"golang.org/x/xerrors"
)

type StorageHandle interface {
	ImportLocalStorage(path string, car bool) (multistore.StoreID, cid.Cid, error)

	DropLocalStorage(ids []multistore.StoreID) error

	ListLocalImports() ([]lapi.Import, error)

	FindData(root cid.Cid, pieceCid *cid.Cid) ([]lapi.QueryOffer, bool, error)

	RetrieveData(
		dataCid cid.Cid,
		outputPath string,
		payer address.Address,
		minerAddr address.Address,
		pieceCid *cid.Cid,
		maxPrice big2.Int,
		car bool) (<-chan marketevents.RetrievalEvent, error)

	InitDeal(
		dataCid cid.Cid,
		miner address.Address,
		price types.FIL,
		duration,
		startEpoch int64,
		verifiedDealParam,
		fastRetrieval bool,
		provCol big2.Int,
		from address.Address,
		ref *storagemarket.DataRef) (*cid.Cid, error)

	QueryAsk(maddr address.Address, pid peer.ID) (*storagemarket.StorageAsk, error)

	ListDeals() ([]lapi.DealInfo, error)

	GetDeal(dealID cid.Cid) (rt.Deal, error)

	ListAsks() ([]*storagemarket.StorageAsk, error)
}

func ConvertDealState(state market.DealState) rt.DealState {
	return rt.DealState{
		SectorStartEpoch: state.SectorStartEpoch,
		LastUpdatedEpoch: state.LastUpdatedEpoch,
		SlashEpoch:       state.SlashEpoch,
	}
}

func ConvertDealInfo(deal lapi.DealInfo) rt.DealInfo {
	return rt.DealInfo{
		ProposalCid:   deal.ProposalCid,
		State:         deal.State,
		Message:       deal.Message,
		Provider:      deal.Provider,
		DataRef:       deal.DataRef,
		PieceCID:      deal.PieceCID,
		Size:          deal.Size,
		PricePerEpoch: deal.PricePerEpoch,
		Duration:      deal.Duration,
		DealID:        deal.DealID,
		CreationTime:  deal.CreationTime,
		Verified:      deal.Verified,
	}
}

func ConvertImport(imp lapi.Import) rt.Import {
	return rt.Import{
		Key:      imp.Key,
		Err:      imp.Err,
		Root:     imp.Root,
		Source:   imp.Source,
		FilePath: imp.FilePath,
	}
}

func ConvertQueryOffer(offer lapi.QueryOffer) rt.QueryOffer {
	return rt.QueryOffer{
		Err:                     offer.Err,
		Root:                    offer.Root,
		Piece:                   offer.Piece,
		Size:                    offer.Size,
		MinPrice:                offer.MinPrice,
		UnsealPrice:             offer.UnsealPrice,
		PaymentInterval:         offer.PaymentInterval,
		PaymentIntervalIncrease: offer.PaymentIntervalIncrease,
		Miner:                   offer.Miner,
		MinerPeer:               offer.MinerPeer,
	}
}

type StorageContractAPI struct {
	Ctx  context.Context
	Full lapi.FullNode
}

func (storage *StorageContractAPI) ImportLocalStorage(path string, car bool) (multistore.StoreID, cid.Cid, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return 0, cid.Undef, err
	}

	ref := lapi.FileRef{
		Path:  absPath,
		IsCAR: car,
	}
	c, err := storage.Full.ClientImport(storage.Ctx, ref)
	if err != nil {
		return 0, cid.Undef, err
	}
	return c.ImportID, c.Root, nil
}

func (storage *StorageContractAPI) DropLocalStorage(ids []multistore.StoreID) error {
	for _, id := range ids {
		if err := storage.Full.ClientRemoveImport(storage.Ctx, id); err != nil {
			return xerrors.Errorf("removing import %d: %w", id, err)
		}
	}
	return nil
}

func (storage *StorageContractAPI) ListLocalImports() ([]lapi.Import, error) {
	list, err := storage.Full.ClientListImports(storage.Ctx)
	if err != nil {
		return []lapi.Import{}, err
	}
	// sort by key
	sort.Slice(list, func(i, j int) bool {
		return list[i].Key < list[j].Key
	})

	return list, nil
}

func (storage *StorageContractAPI) FindData(root cid.Cid, pieceCid *cid.Cid) ([]lapi.QueryOffer, bool, error) {
	// Check if we already have this data locally

	local, err := storage.Full.ClientHasLocal(storage.Ctx, root)
	if err != nil {
		return []lapi.QueryOffer{}, local, err
	}

	offers, err := storage.Full.ClientFindData(storage.Ctx, root, pieceCid)
	if err != nil {
		return []lapi.QueryOffer{}, local, err
	}

	return offers, local, nil
}

func (storage *StorageContractAPI) RetrieveData(
	dataCid cid.Cid,
	outputPath string,
	payer address.Address,
	minerAddr address.Address,
	pieceCid *cid.Cid,
	maxPrice big2.Int,
	car bool) (<-chan marketevents.RetrievalEvent, error) {

	// Check if we already have this data locally
	has, err := storage.Full.ClientHasLocal(storage.Ctx, dataCid)
	if err != nil {
		return nil, err
	}

	if has {
		// Success: Already in local storage
		return nil, nil
	}

	var offer lapi.QueryOffer
	if minerAddr == address.Undef { // Local discovery
		offers, err := storage.Full.ClientFindData(storage.Ctx, dataCid, pieceCid)
		if err != nil {
			return nil, err
		}
		hasNoErr := false
		// filter out offers that errored
		for _, o := range offers {
			if o.Err == "" {
				if !hasNoErr || offer.MinPrice.GreaterThan(o.MinPrice) {
					hasNoErr = true
					offer = o
				}
			}
		}
	} else { // Directed retrieval
		offer, err = storage.Full.ClientMinerQueryOffer(storage.Ctx, minerAddr, dataCid, pieceCid)
		if err != nil {
			return nil, err
		}
	}
	if offer.Err != "" {
		return nil, fmt.Errorf("the received offer errored: %s", offer.Err)
	}

	if offer.MinPrice.Int == nil || offer.MinPrice.GreaterThan(maxPrice) {
		return nil, xerrors.Errorf("failed to find offer satisfying maxPrice: %s", maxPrice)
	}

	ref := &lapi.FileRef{
		Path:  outputPath,
		IsCAR: car,
	}
	return storage.Full.ClientRetrieveWithEvents(storage.Ctx, offer.Order(payer), ref)
}

func (storage *StorageContractAPI) InitDeal(
	dataCid cid.Cid,
	miner address.Address,
	price types.FIL,
	duration,
	startEpoch int64,
	verifiedDealParam,
	fastRetrieval bool,
	provCol big2.Int,
	from address.Address,
	ref *storagemarket.DataRef) (*cid.Cid, error) {

	if abi.ChainEpoch(duration) < build.MinDealDuration {
		return nil, xerrors.Errorf("minimum deal duration is %d blocks", build.MinDealDuration)
	}
	// Check if the address is a verified client
	dcap, err := storage.Full.StateVerifiedClientStatus(storage.Ctx, from, types.EmptyTSK)
	if err != nil {
		return nil, err
	}

	if verifiedDealParam && dcap == nil {
		return nil, xerrors.Errorf("address %s does not have verified client status", from)
	}

	return storage.Full.ClientStartDeal(storage.Ctx, &lapi.StartDealParams{
		Data:               ref,
		Wallet:             from,
		Miner:              miner,
		EpochPrice:         types.BigInt(price),
		MinBlocksDuration:  uint64(duration),
		DealStartEpoch:     abi.ChainEpoch(startEpoch),
		FastRetrieval:      fastRetrieval,
		VerifiedDeal:       verifiedDealParam,
		ProviderCollateral: provCol,
	})
}

func (storage *StorageContractAPI) QueryAsk(maddr address.Address, pid peer.ID) (*storagemarket.StorageAsk, error) {
	return storage.Full.ClientQueryAsk(storage.Ctx, pid, maddr)
}

func (storage *StorageContractAPI) ListDeals() ([]lapi.DealInfo, error) {
	return storage.Full.ClientListDeals(storage.Ctx)
}

func (storage *StorageContractAPI) GetDeal(dealID cid.Cid) (rt.Deal, error) {
	di, err := storage.Full.ClientGetDealInfo(storage.Ctx, dealID)
	if err != nil {
		return rt.Deal{}, err
	}
	return storage.DealFromDealInfo(types.EmptyTSK, *di), err
}

func (storage *StorageContractAPI) ListAsks() ([]*storagemarket.StorageAsk, error) {
	return storage.getAsks()
}

func (storage *StorageContractAPI) DealFromDealInfo(key types.TipSetKey, deal lapi.DealInfo) rt.Deal {
	if deal.DealID == 0 {
		return rt.Deal{
			LocalDeal:        ConvertDealInfo(deal),
			OnChainDealState: ConvertDealState(*market.EmptyDealState()),
		}
	}

	onChain, err := storage.Full.StateMarketStorageDeal(storage.Ctx, deal.DealID, key)
	if err != nil {
		return rt.Deal{LocalDeal: ConvertDealInfo(deal)}
	}

	return rt.Deal{
		LocalDeal:        ConvertDealInfo(deal),
		OnChainDealState: ConvertDealState(onChain.State),
	}
}

func processMiners(miners []address.Address, handler func(wg *sync.WaitGroup, miner address.Address)) {
	var wg sync.WaitGroup
	wg.Add(len(miners))

	for _, miner := range miners {
		go handler(&wg, miner)
	}
	wg.Wait()
}

func (storage *StorageContractAPI) getAsks() ([]*storagemarket.StorageAsk, error) {
	var lk sync.Mutex
	miners, err := storage.Full.StateListMiners(storage.Ctx, types.EmptyTSK)
	if err != nil {
		return nil, xerrors.Errorf("getting miner list: %w", err)
	}

	// Find miners with cheapiest power
	var withMinPower []address.Address
	findBestMiners := func(wg *sync.WaitGroup, miner address.Address) {
		defer wg.Done()

		power, err := storage.Full.StateMinerPower(storage.Ctx, miner, types.EmptyTSK)
		if err != nil {
			return
		}

		if power.HasMinPower { // TODO: Lower threshold
			lk.Lock()
			withMinPower = append(withMinPower, miner)
			lk.Unlock()
		}
	}
	processMiners(miners, findBestMiners)

	// Get asks from this miners
	var asks []*storagemarket.StorageAsk
	getBestAsks := func(wg *sync.WaitGroup, miner address.Address) {
		defer wg.Done()

		ctx, cancel := context.WithTimeout(storage.Ctx, 4*time.Second)
		defer cancel()

		mi, err := storage.Full.StateMinerInfo(ctx, miner, types.EmptyTSK)
		if err != nil {
			return
		}
		if mi.PeerId == nil {
			return
		}

		ask, err := storage.Full.ClientQueryAsk(ctx, *mi.PeerId, miner)
		if err != nil {
			return
		}

		lk.Lock()
		asks = append(asks, ask)
		lk.Unlock()
	}
	processMiners(withMinPower, getBestAsks)

	// sort asks by price
	sort.Slice(asks, func(i, j int) bool {
		return asks[i].Price.LessThan(asks[j].Price)
	})

	return asks, nil
}

var _ StorageHandle = &StorageContractAPI{}
