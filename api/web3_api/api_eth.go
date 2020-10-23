package web3_api

import (
	"context"
	"encoding/hex"
	"fmt"
	"strconv"
)

type HexString string

func checkHexString(hexString HexString) (bool, error) {
	if hexString == "" || hexString == "0x" {
		hexString = "0x0"
		return true, nil
	}
	if len(hexString) < 2 || hexString[:2] != "0x" {
		return false, fmt.Errorf("can't parse HexString: should start with 0x - %s", hexString)
	}
	return false, nil
}

func (hexString HexString) ToInt() (int64, error) {
	empty, err := checkHexString(hexString)
	if empty || err != nil {
		return 0, err
	}
	return strconv.ParseInt(string(hexString[2:]), 16, 0)
}

func (hexString HexString) Bytes() ([]byte, error) {
	empty, err := checkHexString(hexString)
	if empty || err != nil {
		return []byte{}, err
	}
	return hex.DecodeString(string(hexString[2:]))
}

func GetHexString(value int64) HexString {
	return HexString("0x" + strconv.FormatInt(value, 16))
}

// type for input values
type Quantity interface{}

func ConvertQuantity(input Quantity) HexString {
	switch input := input.(type) {
	case string:
		return HexString(input)
	case HexString:
		return input
	default:
		return GetHexString(int64(input.(float64)))
	}
}

type SyncingType struct {
	StartingBlock HexString `json:"startingBlock"`
	CurrentBlock  HexString `json:"currentBlock"`
	HighestBlock  HexString `json:"highestBlock"`
}

type NewFilterInfo struct {
	FromBlock HexString  `json:"fromBlock"`
	ToBlock   HexString  `json:"toBlock"`
	Address   []string   `json:"address"`
	Topics    [][]string `json:"topics"`
}

type FilterQuery struct {
	NewFilterInfo
	BlockHash string `json:"blockHash"`
}

type NewFilterResult struct {
	Removed          bool      `json:"removed"`
	LogIndex         HexString `json:"logIndex"`
	TransactionIndex HexString `json:"transactionIndex"`
	TransactionHash  string    `json:"transactionHash"`
	BlockHash        string    `json:"blockHash"`
	BlockNumber      HexString `json:"blockNumber"`
	BlockIndex       HexString `json:"blockIndex"`
	Address          string    `json:"address"`
	Data             string    `json:"data"`
	Topics           []string  `json:"topics"`
}

type GetLogsResult interface{}

type BlockInfo struct {
	Number           HexString     `json:"number"`
	BlockIndex       HexString     `json:"blockIndex"`
	Hash             string        `json:"hash"`
	ParentHash       string        `json:"parentHash"`
	Nonce            string        `json:"nonce"`
	Sha3Uncles       string        `json:"sha3Uncles"`
	LogsBloom        string        `json:"logsBloom"`
	TransactionsRoot string        `json:"transactionsRoot"`
	StateRoot        string        `json:"stateRoot"`
	ReceiptsRoot     string        `json:"receiptsRoot"`
	Miner            string        `json:"miner"`
	Difficulty       HexString     `json:"difficulty"`
	TotalDifficulty  HexString     `json:"totalDifficulty"`
	ExtraData        string        `json:"extraData"`
	Size             HexString     `json:"size"`
	GasLimit         HexString     `json:"gasLimit"`
	GasUsed          HexString     `json:"gasUsed"`
	Timestamp        HexString     `json:"timestamp"`
	Transactions     []interface{} `json:"transactions"`
	Uncles           []string      `json:"uncles"`
}

type MessageInfo struct {
	From     string    `json:"from"`
	To       string    `json:"to"`
	Gas      HexString `json:"gas"`
	GasPrice HexString `json:"gasPrice"`
	Value    HexString `json:"value"`
	Data     HexString `json:"data"`
	Nonce    HexString `json:"nonce"`
}

type LogsView struct {
	Address string   `json:"address"`
	Topics  []string `json:"topics"`
	Removed bool     `json:"removed"`
	Data    string   `json:"data"`
}

type TransactionInfo struct {
	BlockHash        string    `json:"blockHash"`
	BlockNumber      HexString `json:"blockNumber"`
	BlockIndex       HexString `json:"blockIndex"`
	From             string    `json:"from"`
	To               string    `json:"to"`
	Gas              HexString `json:"gas"`
	GasPrice         HexString `json:"gasPrice"`
	Hash             string    `json:"hash"`
	Input            string    `json:"input"`
	Nonce            HexString `json:"nonce"`
	TransactionIndex HexString `json:"transactionIndex"`
	Value            HexString `json:"value"`
	V                HexString `json:"v"`
	R                string    `json:"r"`
	S                string    `json:"s"`
}

type TransactionReceipt struct {
	TransactionHash   string     `json:"transactionHash"`
	TransactionIndex  HexString  `json:"transactionIndex"`
	BlockHash         string     `json:"blockHash"`
	BlockNumber       HexString  `json:"blockNumber"`
	BlockIndex        HexString  `json:"blockIndex"`
	From              string     `json:"from"`
	To                string     `json:"to"`
	CumulativeGasUsed HexString  `json:"cumulativeGasUsed"`
	GasUsed           HexString  `json:"gasUsed"`
	ContractAddress   string     `json:"contractAddress"`
	Logs              []LogsView `json:"logs"`
	LogsBloom         string     `json:"logsBloom"`
	Status            string     `json:"status"`
}

type WorkInfo struct {
	CurrentBlock string `json:"currentBlock"`
	Seed         string `json:"seed"`
	Boundary     string `json:"boundary"`
}

type EthInfo interface {
	ProtocolVersion(ctx context.Context) (HexString, error)
	Syncing(ctx context.Context) (interface{}, error)
	Coinbase(ctx context.Context) (string, error)
	Mining(ctx context.Context) (bool, error)
	Hashrate(ctx context.Context) (HexString, error)
	GasPrice(ctx context.Context) (HexString, error)
	Accounts(ctx context.Context) ([]string, error)
	BlockNumber(ctx context.Context) (HexString, error)
	GetBalance(ctx context.Context, address string, blocknumOrTag Quantity) (HexString, error)
	GetStorageAt(ctx context.Context, address string, position HexString, blocknumOrTag Quantity) (HexString, error)
	GetTransactionCount(ctx context.Context, address string, blocknumOrTag Quantity) (HexString, error)
	GetBlockByNumber(ctx context.Context, blocknumOrTag Quantity, fullTransactions bool, index Quantity) (BlockInfo, error)
	GetBlockByHash(ctx context.Context, blockhash string, fullTransactions bool) (BlockInfo, error)
	GetUncleByBlockHashAndIndex(ctx context.Context, data string, index Quantity) (BlockInfo, error)
	EstimateGas(ctx context.Context, info MessageInfo, blocknumOrTag Quantity) (HexString, error)
	GetTransactionByHash(ctx context.Context, txhash string) (TransactionInfo, error)
	GetCode(ctx context.Context, address string, blocknumOrTag Quantity) (HexString, error)
	GetBlockTransactionCountByHash(ctx context.Context, blockhash string) (HexString, error)
	GetBlockTransactionCountByNumber(ctx context.Context, blocknumOrTag, index Quantity) (HexString, error)
	GetUncleCountByBlockHash(ctx context.Context, blockhash string) (HexString, error)
	GetUncleCountByBlockNumber(ctx context.Context, blocknumOrTag Quantity) (HexString, error)
	GetTransactionByBlockHashAndIndex(ctx context.Context, blockhash string, index Quantity) (TransactionInfo, error)
	GetTransactionByBlockNumberAndIndex(ctx context.Context, blocknumOrTag Quantity, index, blockIndex Quantity) (TransactionInfo, error)
	GetTransactionReceipt(ctx context.Context, txhash string) (TransactionReceipt, error)
	GetUncleByBlockNumberAndIndex(ctx context.Context, blockhash string, index Quantity) (TransactionInfo, error)
	GetTipsetByHeight(ctx context.Context, blocknumOrTag Quantity) ([]BlockInfo, error)
	GetCompilers(ctx context.Context) ([]string, error)
	CompileSerpent(ctx context.Context, data string) (string, error)
	CompileLLL(ctx context.Context, data string) (string, error)
	CompileSolidity(ctx context.Context, data string) (string, error)
	NewFilter(ctx context.Context, filter NewFilterInfo) (HexString, error)
	NewBlockFilter(ctx context.Context) (HexString, error)
	NewPendingTransactionFilter(ctx context.Context) (HexString, error)
	UninstallFilter(ctx context.Context, id Quantity) (bool, error)
	GetFilterChanges(ctx context.Context, id Quantity) (GetLogsResult, error)
	GetFilterLogs(ctx context.Context, id Quantity) (GetLogsResult, error)
	GetLogs(ctx context.Context, filter FilterQuery) (GetLogsResult, error)
	GetWork(ctx context.Context) (WorkInfo, error)
}

type TraceFilter struct {
	FromBlock   string   `json:"fromBlock"`
	ToBlock     string   `json:"toBlock"`
	FromAddress []string `json:"fromAddress"`
	ToAddress   []string `json:"toAddress"`
	After       Quantity `json:"after"`
	Count       Quantity `json:"count"`
}

type TraceAction struct {
	CallType string    `json:"callType"`
	From     string    `json:"from"`
	Gas      HexString `json:"gas"`
	Input    string    `json:"input"`
	To       string    `json:"to"`
	Value    HexString `json:"value"`
}

type TraceActionResult struct {
	GasUsed HexString `json:"gasUsed"`
	Output  string    `json:"output"`
}

type TraceBlock struct {
	Action              TraceAction       `json:"action"`
	BlockHash           string            `json:"blockHash"`
	BlockNumber         HexString         `json:"blockNumber"`
	Result              TraceActionResult `json:"result"`
	Subtraces           HexString         `json:"subtraces"`
	TraceAddress        []string          `json:"traceAddress"`
	TransactionHash     string            `json:"transactionHash"`
	TransactionPosition HexString         `json:"transactionPosition"`
	Type                string            `json:"type"`
}

type TraceFunctionality interface {
	Filter(ctx context.Context, filter TraceFilter) ([]TraceBlock, error)
}

type EthFunctionality interface {
	EthInfo
	Call(ctx context.Context, info MessageInfo, blocknumOrTag Quantity) (HexString, error)
	SendRawTransaction(ctx context.Context, signedTx HexString) (string, error)
	Sign(ctx context.Context, address string, message HexString) (HexString, error)
	SignTransaction(ctx context.Context, message MessageInfo) (HexString, error)
	SendTransaction(ctx context.Context, message MessageInfo) (string, error)
	SubmitHashrate(ctx context.Context, hashrate, id string) (bool, error)
}
