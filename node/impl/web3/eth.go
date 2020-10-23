package web3impl

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"strconv"

	"go.uber.org/fx"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/eth/protocols/eth"
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	web3 "github.com/filecoin-project/lotus/api/web3_api"
	"github.com/filecoin-project/lotus/chain/actors"
	types "github.com/filecoin-project/lotus/chain/types"
	"github.com/filecoin-project/lotus/node/impl/full"
	filters "github.com/filecoin-project/lotus/node/impl/web3/eth_filters"
	"github.com/filecoin-project/lotus/node/impl/web3/subscription"
	"github.com/filecoin-project/specs-actors/v2/actors/builtin"
	"github.com/filecoin-project/specs-actors/v2/actors/builtin/contract"
	init0 "github.com/filecoin-project/specs-actors/v2/actors/builtin/init"
	cid "github.com/ipfs/go-cid"
	"golang.org/x/xerrors"
)

type EthInfoAPI struct {
	fx.In

	full.WalletAPI
	full.MpoolAPI
	full.StateAPI
	full.GasAPI
	full.ChainAPI
	full.SyncAPI
}

func (ethInfo *EthInfoAPI) ProtocolVersion(ctx context.Context) (web3.HexString, error) {
	return web3.GetHexString(eth.ETH65), nil
}

func (ethInfo *EthInfoAPI) Syncing(ctx context.Context) (interface{}, error) {
	syncStatus, err := ethInfo.SyncState(ctx)
	if err != nil {
		return web3.SyncingType{}, err
	}
	if len(syncStatus.ActiveSyncs) == 0 || syncStatus.ActiveSyncs[0].Base == nil || syncStatus.ActiveSyncs[0].Target == nil {
		return false, nil
	}
	return web3.SyncingType{
		StartingBlock: web3.GetHexString(int64(syncStatus.ActiveSyncs[0].Base.Height())),
		CurrentBlock:  web3.GetHexString(int64(syncStatus.ActiveSyncs[0].Target.Height())),
		HighestBlock:  web3.GetHexString(int64(syncStatus.ActiveSyncs[0].Base.Height() + syncStatus.ActiveSyncs[0].Height)),
	}, nil
}
func (ethInfo *EthInfoAPI) Coinbase(ctx context.Context) (string, error) {
	defaultAddress, err := ethInfo.WalletDefaultAddress(ctx)
	if err != nil {
		return "", err
	}
	return defaultAddress.String(), nil
}
func (ethInfo *EthInfoAPI) Mining(ctx context.Context) (bool, error) {
	miners, err := ethInfo.StateListMiners(ctx, types.EmptyTSK)
	if err != nil {
		return false, err
	}
	return len(miners) > 0, nil
}
func (ethInfo *EthInfoAPI) Hashrate(ctx context.Context) (web3.HexString, error) {
	return web3.GetHexString(0), nil
}
func (ethInfo *EthInfoAPI) GasPrice(ctx context.Context) (web3.HexString, error) {
	estimGasPremium, err := ethInfo.GasEstimateGasPremium(ctx, 0, address.Undef, 0, types.EmptyTSK)
	if err != nil {
		return "0x", err
	}
	msgWithGasPremium := &types.Message{GasPremium: estimGasPremium}
	estimGasFee, err := ethInfo.GasEstimateFeeCap(ctx, msgWithGasPremium, 0, types.EmptyTSK)
	if err != nil {
		return "0x", err
	}
	return web3.HexString("0x" + estimGasFee.Text(16)), nil
}
func (ethInfo *EthInfoAPI) Accounts(ctx context.Context) ([]string, error) {
	accountAddresses, err := ethInfo.WalletList(ctx)
	if err != nil {
		return []string{""}, err
	}
	accounts := make([]string, len(accountAddresses))
	for i, address := range accountAddresses {
		accounts[i] = address.String()
	}
	return accounts, nil
}
func (ethInfo *EthInfoAPI) BlockNumber(ctx context.Context) (web3.HexString, error) {
	head, err := ethInfo.ChainHead(ctx)
	if err != nil {
		return "0x", err
	}
	return web3.GetHexString(int64(head.Height())), nil
}
func (ethInfo *EthInfoAPI) GetBalance(ctx context.Context, address string, blocknumOrTag web3.Quantity) (web3.HexString, error) {
	addr, err := convertAddress(address)
	if err != nil {
		return "0x", err
	}
	tipset, err := getTipsetByBlockNumberOrTag(ethInfo, ctx, blocknumOrTag)
	if err != nil {
		return "0x", err
	}
	actor, err := ethInfo.StateGetActor(ctx, addr, tipset.Key())
	if err != nil {
		return "0x", err
	}
	return web3.HexString("0x" + actor.Balance.Text(16)), nil
}

func (ethInfo *EthInfoAPI) GetStorageAt(ctx context.Context, address string, position web3.HexString, blocknumOrTag web3.Quantity) (web3.HexString, error) {
	actorAddr, actorErr := convertAddress(address)
	if actorErr != nil {
		return "", xerrors.Errorf("failed to get from address: %w", actorErr)
	}

	tipset, err := checkContractAddress(ethInfo, ctx, actorAddr, blocknumOrTag)
	if err != nil {
		return "", xerrors.Errorf("failed to get actor: %w", err)
	}
	manager, err := contract.GetStateRootManager()
	if err != nil {
		return "", xerrors.Errorf("failed to get StateRootManager: %w", err)
	}
	root, err := manager.GetRoot(strconv.FormatInt(int64(tipset.Height()), 16))
	if err != nil {
		return "", xerrors.Errorf("failed to get root: %w", err)
	}
	params, err := actors.SerializeParams(&contract.StorageInfo{Address: actorAddr, Position: string(position), Root: root})
	if err != nil {
		return "", xerrors.Errorf("failed to serialize getStorageAt params: %w", err)
	}

	msg := &types.Message{
		From:       actorAddr,
		To:         actorAddr,
		Value:      big.NewInt(0),
		GasPremium: big.NewInt(0),
		GasFeeCap:  big.NewInt(0),
		GasLimit:   int64(0),
		Method:     builtin.MethodsContract.GetStorageAt,
		Params:     params,
		Nonce:      0,
	}

	res, err := callWithoutTransaction(ethInfo, ctx, msg, tipset)
	if err != nil {
		return "", xerrors.Errorf("failed to call message without transaction: %s", err)
	}
	var result contract.StorageResult
	if err := result.UnmarshalCBOR(bytes.NewReader(res.MsgRct.Return)); err != nil {
		return "", xerrors.Errorf("failed to unmarshal return value: %s", err)
	}

	return web3.HexString("0x" + hex.EncodeToString(result.Value)), nil
}
func (ethInfo *EthInfoAPI) GetTipsetByHeight(ctx context.Context, blocknumOrTag web3.Quantity) ([]web3.BlockInfo, error) {
	tipset, err := getTipsetByBlockNumberOrTag(ethInfo, ctx, blocknumOrTag)
	if err != nil {
		return []web3.BlockInfo{}, err
	}
	blocksInfo := make([]web3.BlockInfo, len(tipset.Blocks()))
	for index, block := range tipset.Blocks() {
		blockInfo, err := convertBlockHeader(ethInfo, ctx, block, true, web3.GetHexString(int64(index)))
		if err != nil {
			return []web3.BlockInfo{}, err
		}
		blocksInfo[index] = blockInfo
	}
	return blocksInfo, nil
}

func (ethInfo *EthInfoAPI) GetTransactionCount(ctx context.Context, address string, blocknumOrTag web3.Quantity) (web3.HexString, error) {
	tipset, err := getTipsetByBlockNumberOrTag(ethInfo, ctx, blocknumOrTag)
	if err != nil {
		return "0x", err
	}
	actorAddr, actorErr := convertAddress(address)
	if actorErr != nil {
		return "0x", xerrors.Errorf("failed to get actor address: %w", actorErr)
	}

	actor, err := ethInfo.StateGetActor(ctx, actorAddr, tipset.Key())
	if err != nil {
		return "0x", xerrors.Errorf("failed to get actor for address: %w", err)
	}

	return web3.GetHexString(int64(actor.Nonce)), nil
}

func (ethInfo *EthInfoAPI) GetBlockByNumber(ctx context.Context, blocknumOrTag web3.Quantity, fullTransactions bool, index web3.Quantity) (web3.BlockInfo, error) {
	tipset, err := getTipsetByBlockNumberOrTag(ethInfo, ctx, blocknumOrTag)
	if err != nil {
		return web3.BlockInfo{}, err
	}
	blockIndex, err := web3.ConvertQuantity(index).ToInt()
	if err != nil {
		return web3.BlockInfo{}, err
	}
	if blockIndex >= int64(len(tipset.Blocks())) {
		return web3.BlockInfo{}, fmt.Errorf("index of block in tipset out of range")
	}
	return convertBlockHeader(ethInfo, ctx, tipset.Blocks()[blockIndex], fullTransactions, web3.ConvertQuantity(index))
}
func (ethInfo *EthInfoAPI) GetBlockByHash(ctx context.Context, blockhash string, fullTransactions bool) (web3.BlockInfo, error) {
	blockCid, err := cid.Decode(blockhash)
	if err != nil {
		return web3.BlockInfo{}, err
	}
	blockHeader, err := ethInfo.ChainGetBlock(ctx, blockCid)
	if err != nil {
		return web3.BlockInfo{}, err
	}
	tipset, err := ethInfo.ChainGetTipSetByHeight(ctx, blockHeader.Height, types.EmptyTSK)
	if err != nil {
		return web3.BlockInfo{}, err
	}
	for blockIndex, block := range tipset.Blocks() {
		if block.Cid() == blockCid {
			return convertBlockHeader(ethInfo, ctx, blockHeader,
				fullTransactions, web3.GetHexString(int64(blockIndex)))
		}
	}
	return web3.BlockInfo{}, fmt.Errorf("can't find block in blockHeader height tipset")
}
func (ethInfo *EthInfoAPI) GetUncleByBlockHashAndIndex(ctx context.Context, data string, index web3.Quantity) (web3.BlockInfo, error) {
	return web3.BlockInfo{}, nil
}
func (ethInfo *EthInfoAPI) EstimateGas(ctx context.Context, info web3.MessageInfo, blocknumOrTag web3.Quantity) (web3.HexString, error) {
	tipset, err := getTipsetByBlockNumberOrTag(ethInfo, ctx, blocknumOrTag)
	if err != nil {
		return "0x", err
	}
	msg, err := convertMessageInfo(ethInfo, ctx, tipset.Key(), info, false)
	if err != nil {
		return "0x", err
	}
	gasLimit, err := ethInfo.GasEstimateGasLimit(ctx, msg, tipset.Key())
	return web3.GetHexString(gasLimit), err
}
func (ethInfo *EthInfoAPI) GetTransactionByHash(ctx context.Context, txhash string) (web3.TransactionInfo, error) {
	messageCid, err := cid.Decode(txhash)
	if err != nil {
		return web3.TransactionInfo{}, nil
	}
	return getTransactionByCid(ethInfo, ctx, messageCid)
}
func (ethInfo *EthInfoAPI) GetCode(ctx context.Context, address string, blocknumOrTag web3.Quantity) (web3.HexString, error) {
	actorAddr, actorErr := convertAddress(address)
	if actorErr != nil {
		return "", xerrors.Errorf("failed to get from address: %w", actorErr)
	}

	tipset, err := checkContractAddress(ethInfo, ctx, actorAddr, blocknumOrTag)
	if err != nil {
		return "", xerrors.Errorf("failed to get actor: %w", err)
	}

	msg := &types.Message{
		From:       actorAddr,
		To:         actorAddr,
		Value:      big.NewInt(0),
		GasPremium: big.NewInt(0),
		GasFeeCap:  big.NewInt(0),
		GasLimit:   0,
		Method:     builtin.MethodsContract.GetCode,
		Nonce:      0,
	}

	res, err := callWithoutTransaction(ethInfo, ctx, msg, tipset)
	if err != nil {
		return "", xerrors.Errorf("failed to call message without transaction: %s", err)
	}
	var result contract.GetCodeResult
	if err := result.UnmarshalCBOR(bytes.NewReader(res.MsgRct.Return)); err != nil {
		return "", xerrors.Errorf("failed to unmarshal return value: %s", err)
	}

	return web3.HexString("0x" + result.Code), nil
}
func (ethInfo *EthInfoAPI) GetBlockTransactionCountByHash(ctx context.Context, blockhash string) (web3.HexString, error) {
	blockCid, err := cid.Decode(blockhash)
	if err != nil {
		return "0x", err
	}
	messages, err := ethInfo.ChainGetBlockMessages(ctx, blockCid)
	if err != nil {
		return "0x", err
	}
	return web3.GetHexString(int64(len(messages.Cids))), nil
}
func (ethInfo *EthInfoAPI) GetBlockTransactionCountByNumber(ctx context.Context, blocknumOrTag web3.Quantity, index web3.Quantity) (web3.HexString, error) {
	tipset, err := getTipsetByBlockNumberOrTag(ethInfo, ctx, blocknumOrTag)
	if err != nil {
		return "0x", err
	}
	blockIndex, err := web3.ConvertQuantity(index).ToInt()
	if err != nil {
		return "0x", err
	}
	if blockIndex >= int64(len(tipset.Blocks())) {
		return "0x", fmt.Errorf("index out of range")
	}
	messages, err := ethInfo.ChainGetBlockMessages(ctx, tipset.Blocks()[blockIndex].Cid())
	if err != nil {
		return "0x", err
	}
	return web3.GetHexString(int64(len(messages.Cids))), nil

}
func (ethInfo *EthInfoAPI) GetUncleCountByBlockHash(ctx context.Context, blockhash string) (web3.HexString, error) {
	return "0x", nil
}
func (ethInfo *EthInfoAPI) GetUncleCountByBlockNumber(ctx context.Context, blocknumOrTag web3.Quantity) (web3.HexString, error) {
	return "0x", nil
}
func (ethInfo *EthInfoAPI) GetTransactionByBlockHashAndIndex(ctx context.Context, blockhash string, index web3.Quantity) (web3.TransactionInfo, error) {
	blockCid, err := cid.Decode(blockhash)
	if err != nil {
		return web3.TransactionInfo{}, err
	}
	messages, err := ethInfo.ChainGetBlockMessages(ctx, blockCid)
	if err != nil {
		return web3.TransactionInfo{}, err
	}
	blockIndex, err := web3.ConvertQuantity(index).ToInt()
	if err != nil {
		return web3.TransactionInfo{}, err
	}
	if blockIndex >= int64(len(messages.Cids)) {
		return web3.TransactionInfo{}, fmt.Errorf("index out of range")
	}
	return getTransactionByCid(ethInfo, ctx, messages.Cids[blockIndex])
}
func (ethInfo *EthInfoAPI) GetTransactionByBlockNumberAndIndex(ctx context.Context, blocknumOrTag, index, blockIndex web3.Quantity) (web3.TransactionInfo, error) {
	tipset, err := getTipsetByBlockNumberOrTag(ethInfo, ctx, blocknumOrTag)
	if err != nil {
		return web3.TransactionInfo{}, err
	}
	intBlockIndex, err := web3.ConvertQuantity(blockIndex).ToInt()
	if err != nil {
		return web3.TransactionInfo{}, err
	}
	if intBlockIndex >= int64(len(tipset.Blocks())) {
		return web3.TransactionInfo{}, fmt.Errorf("blockIndex out of range")
	}
	messages, err := ethInfo.ChainGetBlockMessages(ctx, tipset.Blocks()[intBlockIndex].Cid())
	if err != nil {
		return web3.TransactionInfo{}, err
	}
	intIndex, err := web3.ConvertQuantity(index).ToInt()
	if err != nil {
		return web3.TransactionInfo{}, err
	}
	if intIndex >= int64(len(messages.Cids)) {
		return web3.TransactionInfo{}, fmt.Errorf("index out of range")
	}
	return getTransactionByCid(ethInfo, ctx, messages.Cids[intIndex])
}
func (ethInfo *EthInfoAPI) GetTransactionReceipt(ctx context.Context, txhash string) (web3.TransactionReceipt, error) {
	messageCid, err := cid.Decode(txhash)
	if err != nil {
		return web3.TransactionReceipt{}, err
	}
	messageInfo, err := getTransactionByCid(ethInfo, ctx, messageCid)
	if err != nil {
		return web3.TransactionReceipt{}, err
	}
	currentHeight, err := messageInfo.BlockNumber.ToInt()
	if err != nil {
		return web3.TransactionReceipt{}, err
	}
	tipset, err := ethInfo.ChainGetTipSetByHeight(ctx, abi.ChainEpoch(currentHeight+1), types.EmptyTSK)
	if err != nil {
		return web3.TransactionReceipt{}, err
	}
	messageReceipt, err := ethInfo.StateGetReceipt(ctx, messageCid, tipset.Key())
	if err != nil {
		return web3.TransactionReceipt{}, err
	}
	// if ExitCode not equal to 0, then message didn't execute
	status := "0x1"
	if messageReceipt.ExitCode != 0 {
		status = "0x0"
	}
	transactionInfo := web3.TransactionReceipt{
		TransactionHash:   txhash,
		TransactionIndex:  messageInfo.TransactionIndex,
		BlockHash:         messageInfo.BlockHash,
		BlockNumber:       messageInfo.BlockNumber,
		BlockIndex:        web3.GetHexString(0),
		From:              messageInfo.From,
		To:                messageInfo.To,
		CumulativeGasUsed: web3.GetHexString(messageReceipt.GasUsed),
		GasUsed:           web3.GetHexString(messageReceipt.GasUsed),
		Status:            status,
	}

	// Add contract information to transaction:
	// 1. If it's a contract create method
	//    a) message.Method == Exec, then result should be ExecReturn
	//    b) message.method == ExecWithResult, then result shoud be ContractResult
	// 2. Else if it's a contract call, then result shoud be ContractResult
	// 3. Else it's a send transaction, no extra information to transactionInfo
	//
	// ExecWithResult and contract call processed in the same manner
	if messageInfo.To == builtin.InitActorAddr.String() {
		var result init0.ExecReturn
		err = result.UnmarshalCBOR(bytes.NewReader(messageReceipt.Return))
		if err == nil {
			transactionInfo.ContractAddress = result.RobustAddress.String()
			return transactionInfo, nil
		}
	}
	var result contract.ContractResult
	err = result.UnmarshalCBOR(bytes.NewReader(messageReceipt.Return))
	if err == nil {
		transactionInfo.Logs = convertLogsArrayToView(result.Logs)
		transactionInfo.LogsBloom = hex.EncodeToString(contract.LogsBloom(result.Logs))
		transactionInfo.GasUsed = web3.GetHexString(result.GasUsed)
		transactionInfo.ContractAddress = result.Address.GetCommonAddress().String()
	}
	return transactionInfo, nil
}
func (ethInfo *EthInfoAPI) GetUncleByBlockNumberAndIndex(ctx context.Context, blockhash string, index web3.Quantity) (web3.TransactionInfo, error) {
	return web3.TransactionInfo{}, nil
}
func (ethInfo *EthInfoAPI) GetCompilers(ctx context.Context) ([]string, error) {
	return []string{"none"}, nil
}
func (ethInfo *EthInfoAPI) CompileSerpent(ctx context.Context, data string) (string, error) {
	return "compiler not allow", nil
}
func (ethInfo *EthInfoAPI) CompileLLL(ctx context.Context, data string) (string, error) {
	return "compiler not allow", nil
}
func (ethInfo *EthInfoAPI) CompileSolidity(ctx context.Context, data string) (string, error) {
	return "compiler not allow", nil
}
func (ethInfo *EthInfoAPI) NewFilter(ctx context.Context, info web3.NewFilterInfo) (web3.HexString, error) {
	query := web3.FilterQuery{
		NewFilterInfo: info,
		BlockHash:     "",
	}
	filterCrit := filters.ConvertCriteria(query)
	filter := filters.Filter{
		Type:     filters.Logs,
		Hashes:   []string{},
		Criteria: filterCrit,
		Logs:     []web3.LogsView{},
		Subscription: &subscription.Subscriber{
			Identify: subscription.NewID(),
			Mailbox:  make(chan interface{}),
		},
	}
	filter.Subscription.Handler = func(m interface{}) {
		logEntry := m.(contract.LogsEntry)
		filter.Logs = append(filter.Logs, convertLogsArrayToView(filters.TipsetLogs(logEntry, filter.Criteria))...)
	}
	err := AddLogsChannel()
	if err != nil {
		return "0x", fmt.Errorf("can't add Logs channel to listen new logs")
	}
	if filters.Register(filter.Subscription.Identify, &filter) {
		filter.Subscription.Subscribe()
		return web3.HexString(filter.Subscription.Identify), nil
	}
	return "0x", fmt.Errorf("can't register filter")
}
func (ethInfo *EthInfoAPI) NewBlockFilter(ctx context.Context) (web3.HexString, error) {
	filter := &filters.Filter{
		Type:     filters.Blocks,
		LastHash: 0,
		Hashes:   make([]string, 0),
		Subscription: &subscription.Subscriber{
			Identify: subscription.NewID(),
			Mailbox:  make(chan interface{}),
		},
	}
	filter.Subscription.Handler = func(m interface{}) {
		hash := m.(string)
		filter.Hashes = append(filter.Hashes, hash)
	}
	err := AddBlocksChannel(ethInfo)
	if err != nil {
		return "0x", fmt.Errorf("can't add Blocks channel to listen new blocks")
	}
	if filters.Register(filter.Subscription.Identify, filter) {
		filter.Subscription.Subscribe()
		return web3.HexString(filter.Subscription.Identify), nil
	}
	return "0x", fmt.Errorf("can't register filter")
}
func (ethInfo *EthInfoAPI) NewPendingTransactionFilter(ctx context.Context) (web3.HexString, error) {
	filter := &filters.Filter{
		Type:     filters.Transactions,
		LastHash: 0,
		Hashes:   make([]string, 0),
		Subscription: &subscription.Subscriber{
			Identify: subscription.NewID(),
			Mailbox:  make(chan interface{}),
		},
	}
	filter.Subscription.Handler = func(m interface{}) {
		hash := m.(string)
		filter.Hashes = append(filter.Hashes, hash)
	}
	err := AddTransactionsChannel(ethInfo)
	if err != nil {
		return "0x", fmt.Errorf("can't add Transactions channel to listen new pending transactions")
	}
	if filters.Register(filter.Subscription.Identify, filter) {
		filter.Subscription.Subscribe()
		return web3.HexString(filter.Subscription.Identify), nil
	}
	return "0x", fmt.Errorf("can't register filter")
}
func (ethInfo *EthInfoAPI) UninstallFilter(ctx context.Context, id web3.Quantity) (bool, error) {
	return filters.Unregister(subscription.ID(web3.ConvertQuantity(id))), nil
}
func (ethInfo *EthInfoAPI) GetFilterChanges(ctx context.Context, id web3.Quantity) (web3.GetLogsResult, error) {
	filter, err := filters.GetRegisterFilter(subscription.ID(web3.ConvertQuantity(id)))
	if err == nil {
		switch filter.Type {
		case filters.Blocks, filters.Transactions:
			return filter.GetLastHashes(), nil
		case filters.Logs:
			return filter.GetLastLogs(), nil
		}
	}
	return []contract.EvmLogs{}, err
}
func (ethInfo *EthInfoAPI) GetFilterLogs(ctx context.Context, id web3.Quantity) (web3.GetLogsResult, error) {
	filter, err := filters.GetRegisterFilter(subscription.ID(web3.ConvertQuantity(id)))
	if err != nil || filter.Type != filters.Logs {
		return nil, fmt.Errorf("can't find logs filter with id = %s", id)
	}
	logs := filter.GetAllLogs()
	if logs == nil {
		return []contract.EvmLogs{}, nil
	}
	return logs, nil
}
func (ethInfo *EthInfoAPI) GetLogs(ctx context.Context, query web3.FilterQuery) (web3.GetLogsResult, error) {
	if query.BlockHash != "" {
		logs, err := getLogsForTipset(ethInfo, ctx, query.BlockHash)
		return logs, err
	}
	filterCrit := filters.ConvertCriteria(query)
	logs, err := getLogsForRange(ethInfo, ctx, filterCrit)
	return logs, err
}
func (ethInfo *EthInfoAPI) GetWork(ctx context.Context) (web3.WorkInfo, error) {
	return web3.WorkInfo{}, nil
}

type EthFunctionalityAPI struct {
	EthInfoAPI
}

func (ethFunc *EthFunctionalityAPI) Call(ctx context.Context, info web3.MessageInfo, blocknumOrTag web3.Quantity) (web3.HexString, error) {
	message, err := convertMessageInfo(&ethFunc.EthInfoAPI, ctx, types.EmptyTSK, info, false)
	if err != nil {
		return "0x", err
	}
	tipset, err := getTipsetByBlockNumberOrTag(&ethFunc.EthInfoAPI, ctx, blocknumOrTag)
	if err != nil {
		return "0x", xerrors.Errorf("failed to get tipset: %w", err)
	}
	callResult, err := callWithoutTransaction(&ethFunc.EthInfoAPI, ctx, message, tipset)
	if err != nil {
		return "0x", err
	}
	var result contract.ContractResult
	if err := result.UnmarshalCBOR(bytes.NewReader(callResult.MsgRct.Return)); err != nil {
		return "0x", xerrors.Errorf("failed to unmarshal return value: %s", err)
	}
	return web3.HexString("0x" + hex.EncodeToString(result.Value)), nil
}
func (ethFunc *EthFunctionalityAPI) SendRawTransaction(ctx context.Context, signedTx web3.HexString) (string, error) {
	encodeTx, err := signedTx.Bytes()
	if err != nil {
		return "", err
	}
	sm, err := types.DecodeSignedMessage(encodeTx)
	if err != nil {
		return "", err
	}

	cid, err := ethFunc.MpoolPush(ctx, sm)
	if err != nil {
		return "", err
	}
	return cid.String(), nil
}
func (ethFunc *EthFunctionalityAPI) Sign(ctx context.Context, address string, message web3.HexString) (web3.HexString, error) {
	addr, err := convertAddress(address)
	if err != nil {
		return "", err
	}
	messageData, err := message.Bytes()
	if err != nil {
		return "", err
	}
	msg := fmt.Sprintf("\x19Ethereum Signed Message:\n%d%s", len(messageData), string(messageData))
	msgHash := crypto.Keccak256([]byte(msg))
	signature, err := ethFunc.WalletSign(ctx, addr, msgHash)
	if err != nil {
		return "", err
	}
	return web3.HexString("0x" + hex.EncodeToString(signature.Data)), nil
}
func (ethFunc *EthFunctionalityAPI) SignTransaction(ctx context.Context, info web3.MessageInfo) (web3.HexString, error) {
	message, err := convertMessageInfo(&ethFunc.EthInfoAPI, ctx, types.EmptyTSK, info, true)
	if err != nil {
		return "", err
	}
	signedMessage, err := ethFunc.WalletSignMessage(ctx, message.From, message)
	if err != nil {
		return "", err
	}
	serialMessage, err := signedMessage.Serialize()
	if err != nil {
		return "", err
	}
	return web3.HexString("0x" + hex.EncodeToString(serialMessage)), nil
}
func (ethFunc *EthFunctionalityAPI) SendTransaction(ctx context.Context, message web3.MessageInfo) (string, error) {
	messageCid, err := sendSignedMessage(&ethFunc.EthInfoAPI, ctx, message, true)
	if err != nil {
		return "", err
	}
	return messageCid.String(), nil
}
func (ethFunc *EthFunctionalityAPI) SubmitHashrate(ctx context.Context, hashrate, id string) (bool, error) {
	return false, nil
}

var _ web3.EthFunctionality = &EthFunctionalityAPI{}
