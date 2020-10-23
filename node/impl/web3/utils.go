package web3impl

import (
	"context"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/lotus/api"
	web3 "github.com/filecoin-project/lotus/api/web3_api"
	"github.com/filecoin-project/lotus/build"
	"github.com/filecoin-project/lotus/chain/actors"
	"github.com/filecoin-project/lotus/chain/stmgr"
	types "github.com/filecoin-project/lotus/chain/types"
	"github.com/filecoin-project/lotus/node/impl/full"
	filters "github.com/filecoin-project/lotus/node/impl/web3/eth_filters"
	"github.com/syndtr/goleveldb/leveldb"

	"github.com/filecoin-project/specs-actors/v2/actors/builtin"
	"github.com/filecoin-project/specs-actors/v2/actors/builtin/contract"
	init_ "github.com/filecoin-project/specs-actors/v2/actors/builtin/init"
	evmTypes "github.com/filestar-project/evm-adapter/evm/types"
	cid "github.com/ipfs/go-cid"
	"golang.org/x/xerrors"
)

func convertAddress(strAddr string) (address.Address, error) {
	addr, err := address.NewFromString(strAddr)
	if err != nil {
		return address.Undef, err
	}
	return addr, nil
}

func fillAddressSet(set *map[address.Address]struct{}, addresses []string) error {
	for _, addr := range addresses {
		fromAddr, err := convertAddress(addr)
		if err != nil {
			return err
		}
		(*set)[fromAddr] = struct{}{}
	}
	return nil
}

func convertTraceFilter(filter web3.TraceFilter) (*stmgr.ExecutionFilter, error) {
	result := &stmgr.ExecutionFilter{}
	result.From = make(map[address.Address]struct{})
	result.To = make(map[address.Address]struct{})
	err := fillAddressSet(&result.From, filter.FromAddress)
	if err != nil {
		return &stmgr.ExecutionFilter{}, err
	}
	err = fillAddressSet(&result.To, filter.ToAddress)
	return result, err
}

func convertTraceResult(trace []*api.InvocResult, tipset *types.TipSet) []web3.TraceBlock {
	result := make([]web3.TraceBlock, len(trace))
	for i, traceMsg := range trace {
		result[i].Action.CallType = traceMsg.Msg.Method.String()
		result[i].Action.From = traceMsg.Msg.From.String()
		result[i].Action.To = traceMsg.Msg.To.String()
		result[i].Action.Input = "0x"
		if traceMsg.Msg.Params != nil {
			result[i].Action.Input = hex.EncodeToString(traceMsg.Msg.Params)
		}
		result[i].Action.Value = web3.GetHexString(traceMsg.Msg.Value.Int64())
		result[i].Action.Gas = web3.GetHexString(traceMsg.Msg.GasLimit)
		result[i].Result.GasUsed = web3.GetHexString(traceMsg.MsgRct.GasUsed)
		result[i].Result.Output = "0x"
		if traceMsg.MsgRct.Return != nil {
			result[i].Result.Output = hex.EncodeToString(traceMsg.MsgRct.Return)
		}
		result[i].Subtraces = web3.GetHexString(int64(len(traceMsg.ExecutionTrace.Subcalls)))
		result[i].TransactionHash = traceMsg.MsgCid.String()
		result[i].Type = traceMsg.Msg.Method.String()
		result[i].BlockHash = tipset.String()
		result[i].BlockNumber = web3.GetHexString(int64(tipset.Height()))
	}
	return result
}

func convertBlockHeader(
	ethInfo *EthInfoAPI,
	ctx context.Context,
	blockHeader *types.BlockHeader,
	fullTransactions bool,
	blockIndex web3.HexString) (web3.BlockInfo, error) {
	messages, err := ethInfo.ChainGetBlockMessages(ctx, blockHeader.Cid())
	if err != nil {
		return web3.BlockInfo{}, err
	}
	transactions := make([]interface{}, len(messages.Cids))
	if fullTransactions {
		for i, message := range messages.BlsMessages {
			transactions[i] = *message
		}
		for i, signedMessage := range messages.SecpkMessages {
			transactions[len(messages.BlsMessages)+i] = *signedMessage
		}
	} else {
		for i, messageCid := range messages.Cids {
			transactions[i] = messageCid.String()
		}
	}
	return web3.BlockInfo{
		Miner:            blockHeader.Miner.String(),
		Number:           web3.GetHexString(int64(blockHeader.Height)),
		BlockIndex:       blockIndex,
		Hash:             blockHeader.Cid().String(),
		ParentHash:       blockHeader.ParentStateRoot.String(),
		TransactionsRoot: blockHeader.Messages.String(),
		StateRoot:        blockHeader.ParentStateRoot.String(),
		ReceiptsRoot:     blockHeader.ParentMessageReceipts.String(),
		Difficulty:       web3.GetHexString(int64(blockHeader.ElectionProof.WinCount)),
		TotalDifficulty:  web3.GetHexString(int64(blockHeader.ElectionProof.WinCount)),
		GasLimit:         web3.GetHexString(build.BlockGasLimit),
		Timestamp:        web3.GetHexString(int64(blockHeader.Timestamp)),
		Transactions:     transactions,
	}, nil
}

func convertMessageInfo(
	ethInfo *EthInfoAPI,
	ctx context.Context,
	tipset types.TipSetKey,
	info web3.MessageInfo,
	modifyState bool) (*types.Message, error) {
	value, parseErr := info.Value.ToInt()
	if parseErr != nil {
		return nil, xerrors.Errorf("failed to parse value: %w", parseErr)
	}

	fromAddr, actorErr := convertAddress(info.From)
	if actorErr != nil {
		return nil, xerrors.Errorf("failed to get from address: %w", actorErr)
	}

	toAddr := builtin.InitActorAddr
	if info.To != "" {
		toAddr, actorErr = convertAddress(info.To)
		if actorErr != nil {
			return nil, xerrors.Errorf("failed to get to address: %w", actorErr)
		}
	}

	gas, parseErr := info.Gas.ToInt()
	if parseErr != nil {
		return nil, xerrors.Errorf("failed to parse gas: %w", parseErr)
	}
	gasPrice, parseErr := info.GasPrice.ToInt()
	if parseErr != nil {
		return nil, xerrors.Errorf("failed to parse gasPrice: %w", parseErr)
	}
	nonce, parseErr := info.Nonce.ToInt()
	if parseErr != nil {
		return nil, xerrors.Errorf("failed to parse nonce: %w", parseErr)
	}
	messageData, err := info.Data.Bytes()
	if err != nil {
		return nil, err
	}
	isContract, err := IsContractAddress(ethInfo, ctx, toAddr, tipset)
	if err != nil {
		return nil, err
	}
	// If data == []byte{} or it's a contract actor, then build a default send funds message
	// Otherwise build message for contract
	if len(messageData) != 0 && isContract {
		params, err := actors.SerializeParams(&contract.ContractParams{Code: messageData, Value: big.NewInt(value), Commit: modifyState})
		if err != nil {
			return nil, xerrors.Errorf("failed to serialize contract call params: %w", err)
		}

		method := builtin.MethodsContract.CallContract
		if toAddr == builtin.InitActorAddr {
			method = builtin.MethodsInit.Exec
			params, err = actors.SerializeParams(&init_.ExecParams{CodeCID: builtin.ContractActorCodeID, ConstructorParams: params})
			if err != nil {
				return nil, xerrors.Errorf("failed to serialize exec contract create params: %w", err)
			}
		}
		return &types.Message{
			From:       fromAddr,
			To:         toAddr,
			Value:      big.NewInt(0),
			GasPremium: big.NewInt(gasPrice),
			GasFeeCap:  big.NewInt(gasPrice),
			GasLimit:   gas,
			Method:     method,
			Params:     params,
			Nonce:      uint64(nonce),
		}, nil
	}
	return &types.Message{
		From:       fromAddr,
		To:         toAddr,
		Value:      big.NewInt(value),
		GasPremium: big.NewInt(gasPrice),
		GasFeeCap:  big.NewInt(gasPrice),
		GasLimit:   gas,
		Method:     builtin.MethodSend,
		Params:     []byte{},
		Nonce:      uint64(nonce),
	}, nil
}

func sendSignedMessage(
	ethInfo *EthInfoAPI,
	ctx context.Context,
	info web3.MessageInfo,
	modifyState bool) (cid.Cid, error) {

	var cid cid.Cid
	msg, err := convertMessageInfo(ethInfo, ctx, types.EmptyTSK, info, modifyState)
	if err != nil {
		return cid, err
	}
	if msg.Nonce > 0 {
		sm, err := ethInfo.WalletSignMessage(ctx, msg.From, msg)
		if err != nil {
			return cid, err
		}

		_, err = ethInfo.MpoolPush(ctx, sm)
		if err != nil {
			return cid, err
		}
		cid = sm.Cid()
	} else {
		sm, err := ethInfo.MpoolPushMessage(ctx, msg, nil)
		if err != nil {
			return cid, err
		}
		cid = sm.Cid()
	}
	return cid, nil
}

func min(x, y int64) int64 {
	if x > y {
		return y
	}
	return x
}

func getTipsetNumber(
	chainAPI *full.ChainAPI,
	ctx context.Context,
	blocknumOrTag web3.Quantity) (int64, error) {
	tipset, err := chainAPI.ChainHead(ctx)
	if err != nil {
		return 0, err
	}
	topHeight := int64(tipset.Height())
	switch blocknumOrTag {
	case "":
		return topHeight, nil
	case "latest":
		return topHeight, nil
	case "pending":
		return topHeight, nil
	case "earliest":
		return 0, nil
	}
	num, err := web3.ConvertQuantity(blocknumOrTag).ToInt()
	return min(num, topHeight), err
}

func getTipsetByBlockNumberOrTag(
	ethInfo *EthInfoAPI,
	ctx context.Context,
	blocknumOrTag web3.Quantity) (*types.TipSet, error) {
	num, err := getTipsetNumber(&ethInfo.ChainAPI, ctx, blocknumOrTag)
	if err != nil {
		return nil, err
	}
	return ethInfo.ChainGetTipSetByHeight(ctx, abi.ChainEpoch(num), types.EmptyTSK)
}

func callWithoutTransaction(
	ethInfo *EthInfoAPI,
	ctx context.Context,
	message *types.Message,
	tipset *types.TipSet) (*api.InvocResult, error) {
	res, err := ethInfo.StateCall(ctx, message, tipset.Key())

	if err != nil {
		return nil, xerrors.Errorf("failed while calling StateCall: %w", err)
	}

	if res.Error != "" {
		return nil, xerrors.Errorf("failed while executing implicit message: %w", err)
	}

	return res, nil
}

func getTransactionByCid(
	ethInfo *EthInfoAPI,
	ctx context.Context,
	messageCid cid.Cid) (web3.TransactionInfo, error) {
	message, err := ethInfo.ChainGetMessage(ctx, messageCid)
	if err != nil {
		return web3.TransactionInfo{}, err
	}
	block, sign, txIndex, blockIndex, err := findBlockInfoForMessage(ethInfo, ctx, messageCid)
	if err != nil {
		return web3.TransactionInfo{}, err
	}
	V, R, S := parseSign(sign)
	return web3.TransactionInfo{
		BlockHash:        block.Cid().String(),
		BlockNumber:      web3.GetHexString(int64(block.Height)),
		BlockIndex:       web3.GetHexString(blockIndex),
		From:             message.From.String(),
		To:               message.To.String(),
		Gas:              web3.GetHexString(message.GasLimit),
		GasPrice:         web3.GetHexString(message.GasFeeCap.Int64()),
		Hash:             messageCid.String(),
		Input:            hex.EncodeToString(message.Params),
		Nonce:            web3.GetHexString(int64(message.Nonce)),
		TransactionIndex: web3.GetHexString(txIndex),
		Value:            web3.GetHexString(message.Value.Int64()),
		V:                web3.GetHexString(int64(V)),
		R:                R,
		S:                S,
	}, nil
}

func findBlockInfoForMessage(
	ethInfo *EthInfoAPI,
	ctx context.Context,
	messageCid cid.Cid) (*types.BlockHeader, []byte, int64, int64, error) {
	messageInfo, err := ethInfo.StateSearchMsg(ctx, messageCid)
	if err != nil {
		return &types.BlockHeader{}, []byte{}, 0, 0, err
	}
	if messageInfo == nil {
		return &types.BlockHeader{}, []byte{}, 0, 0, fmt.Errorf("can't find messageInfo")
	}
	executedTipset, err := ethInfo.ChainGetTipSet(ctx, messageInfo.TipSet)
	if err != nil {
		return &types.BlockHeader{}, []byte{}, 0, 0, err
	}
	if executedTipset.Height() == 0 {
		return &types.BlockHeader{}, []byte{}, 0, 0, fmt.Errorf("can't execute message at genesis tipset")
	}
	messageTipset, err := ethInfo.ChainGetTipSet(ctx, executedTipset.Parents())
	if err != nil {
		return &types.BlockHeader{}, []byte{}, 0, 0, err
	}
	for blockIndex, block := range messageTipset.Blocks() {
		messages, err := ethInfo.ChainGetBlockMessages(ctx, block.Cid())
		if err != nil {
			return &types.BlockHeader{}, []byte{}, 0, 0, err
		}
		for _, unsignedMsg := range messages.BlsMessages {
			if unsignedMsg.Cid() == messageCid {
				index, err := getMessageIndex(messages.Cids, messageCid)
				return block, []byte{}, index, int64(blockIndex), err
			}
		}
		for _, signedMsg := range messages.SecpkMessages {
			if signedMsg.Cid() == messageCid {
				index, err := getMessageIndex(messages.Cids, messageCid)
				return block, signedMsg.Signature.Data, index, int64(blockIndex), err
			}
		}
	}
	return &types.BlockHeader{}, []byte{}, 0, 0, nil
}

func parseSign(sign []byte) (uint64, string, string) {
	//Signature struct should be R || S || V
	if len(sign) != 65 {
		return 0, "", ""
	}
	return uint64(sign[64]), hex.EncodeToString(sign[:32]), hex.EncodeToString(sign[32:64])
}

func getMessageIndex(messages []cid.Cid, target cid.Cid) (int64, error) {
	for i, messageCid := range messages {
		if messageCid == target {
			return int64(i), nil
		}
	}
	return int64(0), fmt.Errorf("can't find target message Cid in message array")
}

func IsContractAddress(
	ethInfo *EthInfoAPI,
	ctx context.Context,
	addr address.Address,
	tipsetKey types.TipSetKey) (bool, error) {
	actor, err := ethInfo.StateGetActor(ctx, addr, tipsetKey)
	if err != nil {
		return false, xerrors.Errorf("failed to get actor for address: %w", err)
	}
	return actor.Code == builtin.ContractActorCodeID, nil
}

func checkContractAddress(
	ethInfo *EthInfoAPI,
	ctx context.Context,
	addr address.Address,
	blocknumOrTag web3.Quantity) (*types.TipSet, error) {
	tipset, err := getTipsetByBlockNumberOrTag(ethInfo, ctx, blocknumOrTag)
	if err != nil {
		return &types.TipSet{}, xerrors.Errorf("failed to get tipset: %w", err)
	}
	isContract, err := IsContractAddress(ethInfo, ctx, addr, tipset.Key())
	if err != nil {
		return &types.TipSet{}, err
	}
	if !isContract {
		return &types.TipSet{}, xerrors.Errorf("failed to get code: it's not a ContractActor: %w", err)
	}
	return tipset, nil
}

func convertTopics(topics []evmTypes.Hash) []string {
	result := make([]string, 0)
	for _, topic := range topics {
		result = append(result, fmt.Sprintf("0x%x", topic))
	}
	return result
}

func convertLogToView(logs contract.EvmLogs) web3.LogsView {
	return web3.LogsView{
		Address: fmt.Sprintf("0x%x", logs.Address),
		Topics:  convertTopics(logs.Topics),
		Data:    fmt.Sprintf("0x%x", logs.Data),
		Removed: logs.Removed,
	}
}

func convertLogsArrayToView(logs []contract.EvmLogs) []web3.LogsView {
	result := make([]web3.LogsView, 0)
	for _, log := range logs {
		result = append(result, convertLogToView(log))
	}
	return result
}

func getLogsForTipset(
	ethInfo *EthInfoAPI,
	ctx context.Context,
	blockHash string) ([]web3.LogsView, error) {
	blockCid, err := cid.Decode(blockHash)
	if err != nil {
		return []web3.LogsView{}, err
	}
	blockHeader, err := ethInfo.ChainGetBlock(ctx, blockCid)
	if err != nil {
		return []web3.LogsView{}, err
	}
	manager, err := contract.GetLogsManager()
	if err != nil {
		return []web3.LogsView{}, err
	}
	logsEntry, err := manager.GetHeightLogs(int64(blockHeader.Height))
	if err == leveldb.ErrNotFound {
		return []web3.LogsView{}, nil
	}
	return convertLogsArrayToView(logsEntry.Logs), err
}

func getLogsForRange(
	ethInfo *EthInfoAPI,
	ctx context.Context,
	crit *filters.FilterCriteria) ([]web3.LogsView, error) {
	head, err := ethInfo.ChainHead(ctx)
	if err != nil {
		return []web3.LogsView{}, err
	}
	height := int64(head.Height())
	manager, err := contract.GetLogsManager()
	if err != nil {
		return []web3.LogsView{}, err
	}
	from := int64(0)
	if crit.FromBlock != nil {
		from = crit.FromBlock.Int64()
	}
	to := height
	if crit.ToBlock != nil && crit.ToBlock.Int64() < height {
		to = crit.ToBlock.Int64()
	}
	var filteredLogs []web3.LogsView
	for i := from; i <= to; i++ {
		logsEntry, err := manager.GetHeightLogs(i)
		if err == nil && !logsEntry.Empty {
			filteredLogs = append(filteredLogs, convertLogsArrayToView(filters.TipsetLogs(logsEntry, crit))...)
		}
	}
	return filteredLogs, nil
}

// Add functions
// 1. AddLogsChannel
// 2. AddBlocksChannel
// 3. AddTransactionsChannel
// for register channels when it necessary

func AddLogsChannel() error {
	if !filters.ExistFilterChannel(filters.Logs) {
		logsChannel := make(chan interface{})
		// Another goroutine can create channel faster, check it
		if filters.AddFilterChannel(filters.Logs, &logsChannel) {
			manager, err := contract.GetLogsManager()
			if err != nil {
				return err
			}
			manager.SetLogsChannel(logsChannel)
		}
		return nil
	}
	return nil
}

func AddBlocksChannel(ethInfo *EthInfoAPI) error {
	if !filters.ExistFilterChannel(filters.Blocks) {
		txChannel := make(chan interface{}, 1024)
		// Another goroutine can create channel faster, check it
		if filters.AddFilterChannel(filters.Blocks, &txChannel) {
			incomingTipsets, err := ethInfo.ChainNotify(context.Background())
			if err != nil {
				return err
			}
			go func() {
				defer filters.RemoveFilterChannel(filters.Blocks)
				for filters.ExistFilterChannel(filters.Blocks) {
					select {
					case newTipsets, ok := <-incomingTipsets:
						if !ok {
							return
						}
						for _, val := range newTipsets {
							txChannel <- fmt.Sprintf("%v", val.Val.Key())
						}
					default:
						time.Sleep(time.Duration(1) * time.Microsecond)
					}
				}
			}()
		}
		return nil
	}
	return nil
}

func AddTransactionsChannel(ethInfo *EthInfoAPI) error {
	if !filters.ExistFilterChannel(filters.Transactions) {
		txChannel := make(chan interface{})
		// Another goroutine can create channel faster, check it
		if filters.AddFilterChannel(filters.Transactions, &txChannel) {
			mpoolChan, err := ethInfo.MpoolSub(context.Background())
			if err != nil {
				return err
			}
			go func() {
				defer filters.RemoveFilterChannel(filters.Blocks)
				for filters.ExistFilterChannel(filters.Blocks) {
					select {
					case newMPoolMessage, ok := <-mpoolChan:
						if !ok {
							return
						}
						if newMPoolMessage.Type == api.MpoolAdd && newMPoolMessage.Message != nil {
							txChannel <- fmt.Sprintf("%v", newMPoolMessage.Message.Message.Cid())
						}
					default:
						time.Sleep(time.Duration(1) * time.Microsecond)
					}
				}
			}()
		}
		return nil
	}
	return nil
}
