package filters

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/core/types"
	web3 "github.com/filecoin-project/lotus/api/web3_api"
	subs "github.com/filecoin-project/lotus/node/impl/web3/subscription"
	"github.com/filecoin-project/specs-actors/v2/actors/builtin/contract"
	evmTypes "github.com/filestar-project/evm-adapter/evm/types"
)

// Type determines the kind of filter and is used to put the filter in to
// the correct bucket when added.

type ContentType int

const (
	Logs ContentType = iota
	Transactions
	Blocks
)

func (content ContentType) String() string {
	values := []string{"Logs", "Transactions", "Blocks"}
	return values[content]
}

func ConvertCriteria(query web3.FilterQuery) *FilterCriteria {
	var fromBlockBig, toBlockBig *big.Int = nil, nil
	fromBlock, err := query.FromBlock.ToInt()
	if err == nil {
		fromBlockBig = big.NewInt(fromBlock)
	}
	toBlock, err := query.ToBlock.ToInt()
	if err == nil && query.ToBlock != "" && query.ToBlock != "0x" {
		toBlockBig = big.NewInt(toBlock)
	}
	addresses := make([]evmTypes.Address, len(query.Address))
	topics := make([][]evmTypes.Hash, len(query.Topics))
	for i, addr := range query.Address {
		addr, err := hex.DecodeString(addr)
		if err == nil {
			addresses[i] = evmTypes.BytesToAddress(addr)
		}
	}
	for i, topicSet := range query.Topics {
		for _, topic := range topicSet {
			topicBytes, err := hex.DecodeString(topic)
			if err == nil {
				topics[i] = append(topics[i], evmTypes.BytesToHash(topicBytes))
			}
		}
	}
	return &FilterCriteria{
		FromBlock: fromBlockBig,
		ToBlock:   toBlockBig,
		Addresses: addresses,
		Topics:    topics,
	}
}

type FilterCriteria struct {
	FromBlock *big.Int
	ToBlock   *big.Int
	Addresses []evmTypes.Address
	Topics    [][]evmTypes.Hash
	TipsetKey string
}

type Filter struct {
	Type         ContentType
	LastHash     int64
	Hashes       []string
	Criteria     *FilterCriteria
	LastLogs     int64
	Logs         []web3.LogsView
	Timer        *time.Timer
	Subscription *subs.Subscriber
}

func (filter *Filter) UpdateTimer(timeout time.Duration) {
	if !filter.Timer.Stop() {
		<-filter.Timer.C
	}
	filter.Timer.Reset(timeout)
}

func (filter *Filter) GetAllLogs() []web3.LogsView {
	if filter.Logs == nil {
		return []web3.LogsView{}
	}
	filter.LastLogs = int64(len(filter.Logs))
	return filter.Logs
}

func (filter *Filter) GetAllHashes() []string {
	if filter.Hashes == nil {
		return []string{}
	}
	filter.LastHash = int64(len(filter.Hashes))
	return filter.Hashes
}

func (filter *Filter) GetLastLogs() []web3.LogsView {
	if filter.LastLogs == int64(len(filter.Logs)) {
		return []web3.LogsView{}
	}
	result := filter.Logs[filter.LastLogs:]
	filter.LastLogs = int64(len(filter.Logs))
	return result
}

func (filter *Filter) GetLastHashes() []string {
	if filter.LastHash == int64(len(filter.Hashes)) {
		return []string{}
	}
	result := filter.Hashes[filter.LastHash:]
	filter.LastHash = int64(len(filter.Hashes))
	return result
}

// tipsetLogs returns the logs matching the Filter criteria within a single block.
func TipsetLogs(entry contract.LogsEntry, criteria *FilterCriteria) []contract.EvmLogs {
	if !bloomFilter(entry.LogsBloom, criteria.Addresses, criteria.Topics) {
		return []contract.EvmLogs{}
	}
	return filterLogs(entry.Logs, uint64(entry.Height), criteria)
}

func includes(addresses []evmTypes.Address, a evmTypes.Address) bool {
	for _, addr := range addresses {
		if addr == a {
			return true
		}
	}

	return false
}

// FilterLogs creates a slice of logs matching the given criteria.
func filterLogs(logs []contract.EvmLogs, height uint64, criteria *FilterCriteria) []contract.EvmLogs {
	var ret []contract.EvmLogs
Logs:
	for _, log := range logs {
		if criteria.FromBlock != nil && criteria.FromBlock.Int64() >= 0 && criteria.FromBlock.Uint64() > height {
			continue
		}
		if criteria.ToBlock != nil && criteria.ToBlock.Int64() >= 0 && criteria.ToBlock.Uint64() < height {
			continue
		}

		if len(criteria.Addresses) > 0 && !includes(criteria.Addresses, log.Address) {
			continue
		}
		// If the to Filtered topics is greater than the amount of topics in logs, skip.
		if len(criteria.Topics) > len(log.Topics) {
			continue
		}
		for i, sub := range criteria.Topics {
			match := len(sub) == 0 // empty rule set == wildcard
			for _, topic := range sub {
				if log.Topics[i] == topic {
					match = true
					break
				}
			}
			if !match {
				continue Logs
			}
		}
		ret = append(ret, log)
	}
	return ret
}

func bloomFilter(bloom types.Bloom, addresses []evmTypes.Address, topics [][]evmTypes.Hash) bool {
	if len(addresses) > 0 {
		var included bool
		for _, addr := range addresses {
			if types.BloomLookup(bloom, addr) {
				included = true
				break
			}
		}
		if !included {
			return false
		}
	}

	for _, sub := range topics {
		included := len(sub) == 0 // empty rule set == wildcard
		for _, topic := range sub {
			if types.BloomLookup(bloom, topic) {
				included = true
				break
			}
		}
		if !included {
			return false
		}
	}
	return true
}

type filterManager struct {
	subscribersMu    sync.Mutex
	subscribeManager *subs.Publisher
	filtersMu        sync.Mutex
	filters          map[subs.ID]*Filter
	timeout          time.Duration
}

func newFilterManager() *filterManager {
	return &filterManager{
		subscribersMu:    sync.Mutex{},
		subscribeManager: subs.NewPublisher(),
		filters:          make(map[subs.ID]*Filter),
		filtersMu:        sync.Mutex{},
		timeout:          5 * time.Minute,
	}
}

var manager *filterManager = newFilterManager()

// Observer, check filters and remove if it's dead
func checkAlive() {
	ticker := time.NewTicker(manager.timeout)
	defer ticker.Stop()
	for {
		<-ticker.C
		manager.filtersMu.Lock()
		for id, f := range manager.filters {
			select {
			case <-f.Timer.C:
				RemoveFilterSubscriber(f.Type, id)
				delete(manager.filters, id)
			default:
				continue
			}
		}
		manager.filtersMu.Unlock()
	}
}

func StartFiltersObserver() {
	go checkAlive()
}

func Register(id subs.ID, newFilter *Filter) bool {
	// Add filter to map
	newFilter.Timer = time.NewTimer(manager.timeout)
	manager.filtersMu.Lock()
	manager.filters[id] = newFilter
	manager.filtersMu.Unlock()
	// Subscribe filter to updates
	if newFilter.Subscription != nil {
		return AddFilterSubscriber(newFilter.Type, newFilter.Subscription)
	}
	return true
}

func GetRegisterFilter(id subs.ID) (*Filter, error) {
	manager.filtersMu.Lock()
	defer manager.filtersMu.Unlock()

	if f, found := manager.filters[id]; found {
		f.UpdateTimer(manager.timeout)
		return f, nil
	}
	return &Filter{}, fmt.Errorf("filter with id %s doesn't exist", id)
}

func Unregister(id subs.ID) bool {
	// Add filter to map
	var filter *Filter
	manager.filtersMu.Lock()
	filter = manager.filters[id]
	delete(manager.filters, id)
	manager.filtersMu.Unlock()
	// Subscribe filter to updates
	if filter.Subscription != nil {
		return RemoveFilterSubscriber(filter.Type, id)
	}
	return true
}

func ExistFilterChannel(content ContentType) bool {
	manager.subscribersMu.Lock()
	defer manager.subscribersMu.Unlock()
	return manager.subscribeManager.ExistChannel(content.String())
}

func AddFilterChannel(content ContentType, channelInfo *chan interface{}) bool {
	manager.subscribersMu.Lock()
	defer manager.subscribersMu.Unlock()
	err := manager.subscribeManager.AddChannel(content.String(), channelInfo)
	return err == nil
}

func RemoveFilterChannel(content ContentType) bool {
	manager.subscribersMu.Lock()
	defer manager.subscribersMu.Unlock()
	err := manager.subscribeManager.RemoveChannel(content.String())
	return err == nil
}

func AddFilterSubscriber(content ContentType, sub *subs.Subscriber) bool {
	manager.subscribersMu.Lock()
	defer manager.subscribersMu.Unlock()
	err := manager.subscribeManager.AddSubscriber(content.String(), sub)
	return err == nil
}

func RemoveFilterSubscriber(content ContentType, id subs.ID) bool {
	manager.subscribersMu.Lock()
	defer manager.subscribersMu.Unlock()
	err := manager.subscribeManager.RemoveSubscriber(content.String(), id)
	return err == nil
}
