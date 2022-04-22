package storage_empty

import (
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/filecoin-project/go-multistore"
	big2 "github.com/filecoin-project/go-state-types/big"
	lapi "github.com/filecoin-project/lotus/api"
	sapi "github.com/filecoin-project/lotus/api/storage_api"
	"github.com/filecoin-project/lotus/chain/types"
	marketevents "github.com/filecoin-project/lotus/markets/loggers"
	rt "github.com/filecoin-project/specs-actors/v2/actors/runtime"
	"github.com/ipfs/go-cid"
	peer "github.com/libp2p/go-libp2p-core/peer"
)

type EmptyStorageAPI struct{}

func (empty *EmptyStorageAPI) ImportLocalStorage(path string, car bool) (multistore.StoreID, cid.Cid, error) {
	return 0, cid.Undef, nil
}

func (empty *EmptyStorageAPI) DropLocalStorage(ids []multistore.StoreID) error {
	return nil
}

func (empty *EmptyStorageAPI) ListLocalImports() ([]lapi.Import, error) {
	return nil, nil
}

func (empty *EmptyStorageAPI) FindData(root cid.Cid, pieceCid *cid.Cid) ([]lapi.QueryOffer, bool, error) {
	return nil, false, nil
}

func (empty *EmptyStorageAPI) RetrieveData(
	dataCid cid.Cid,
	outputPath string,
	payer address.Address,
	minerAddr address.Address,
	pieceCid *cid.Cid,
	maxPrice big2.Int,
	car bool) (<-chan marketevents.RetrievalEvent, error) {
	return nil, nil
}

func (empty *EmptyStorageAPI) InitDeal(
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
	return nil, nil
}

func (empty *EmptyStorageAPI) QueryAsk(maddr address.Address, pid peer.ID) (*storagemarket.StorageAsk, error) {
	return nil, nil
}

func (empty *EmptyStorageAPI) ListDeals() ([]lapi.DealInfo, error) {
	return nil, nil
}

func (empty *EmptyStorageAPI) GetDeal(dealID cid.Cid) (rt.Deal, error) {
	return rt.Deal{}, nil
}

func (empty *EmptyStorageAPI) ListAsks() ([]*storagemarket.StorageAsk, error) {
	return nil, nil
}

var _ sapi.StorageHandle = &EmptyStorageAPI{}
