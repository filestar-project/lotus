package rpc_test

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"math/big"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/eth/protocols/eth"
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/lotus/api"
	web3 "github.com/filecoin-project/lotus/api/web3_api"
	"github.com/filecoin-project/lotus/build"
	"github.com/filecoin-project/lotus/chain/actors"
	"github.com/filecoin-project/lotus/chain/actors/policy"
	utils "github.com/filecoin-project/lotus/chain/contractsutils"
	"github.com/filecoin-project/lotus/chain/gen"
	"github.com/filecoin-project/lotus/chain/store"
	"github.com/filecoin-project/lotus/chain/types"
	"github.com/filecoin-project/lotus/node"
	"github.com/filecoin-project/lotus/node/impl"
	"github.com/filecoin-project/lotus/node/modules"
	"github.com/filecoin-project/lotus/node/repo"
	"github.com/filecoin-project/specs-actors/v2/actors/builtin/contract"
	init0 "github.com/filecoin-project/specs-actors/v2/actors/builtin/init"
	"github.com/filestar-project/evm-adapter/tests"
	"github.com/ipfs/go-cid"
	logging "github.com/ipfs/go-log/v2"
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"
	"github.com/mitchellh/go-homedir"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type RPCTestSuite struct {
	suite.Suite
	utils *RPCTestUtil
}

const chainHeight = 10
const extraMsgCount = 2
const genMessages = 20
const repoPath = "~/.lotusDevnet/rpc_tests"

func newRPCTestSuite(t *testing.T) *RPCTestSuite {
	return &RPCTestSuite{utils: prepRpcTest(t, chainHeight)}
}

func init() {
	build.InsecurePoStValidation = true
	err := os.Setenv("TRUST_PARAMS", "1")
	if err != nil {
		panic(err)
	}
	policy.SetSupportedProofTypes(abi.RegisteredSealProof_StackedDrg2KiBV1)
	policy.SetConsensusMinerMinPower(abi.NewStoragePower(2048))
	policy.SetMinVerifiedDealSize(abi.NewStoragePower(256))
}

func (suite *RPCTestUtil) repoWithChain(t testing.TB, h int) (repo.Repo, []byte, []*store.FullTipSet) {
	blks := make([]*store.FullTipSet, h)

	for i := 0; i < h; i++ {
		mts, err := suite.g.NextTipSet()
		require.NoError(t, err)

		blks[i] = mts.TipSet
	}

	r, err := suite.g.YieldRepo()
	require.NoError(t, err)

	genb, err := suite.g.GenesisCar()
	require.NoError(t, err)

	return r, genb, blks
}

type RPCTestUtil struct {
	t testing.TB

	ctx    context.Context
	cancel func()

	mn mocknet.Mocknet

	g *gen.ChainGen

	genesis []byte
	blocks  []*store.FullTipSet

	fullAPI []api.FullNode
	rpcAPIs []web3.FullWeb3Interface
}

func prepRpcTest(t testing.TB, h int) *RPCTestUtil {
	logging.SetLogLevel("*", "INFO")

	contract.PathToRepo = repoPath
	fpath, err := homedir.Expand(repoPath)
	require.NoError(t, err)
	_, err = os.Stat(fpath)
	if err == nil {
		err = os.RemoveAll(fpath)
		require.NoError(t, err)
	}
	require.NoError(t, contract.InitStateDBManager())

	g, err := gen.NewGenerator()
	if err != nil {
		t.Fatalf("%+v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	suite := &RPCTestUtil{
		t:      t,
		ctx:    ctx,
		cancel: cancel,

		mn: mocknet.New(ctx),
		g:  g,
	}
	suite.addSourceNode(h)
	// separate logs
	fmt.Println("\x1b[31m///////////////////////////////////////////////////\x1b[39b")

	return suite
}

func (suite *RPCTestUtil) addSourceNode(gen int) {
	if suite.genesis != nil {
		suite.t.Fatal("source node already exists")
	}

	sourceRepo, genesis, blocks := suite.repoWithChain(suite.t, gen)
	var out api.FullNode
	var web3RPC web3.FullWeb3Interface

	stop, err := node.New(suite.ctx,
		node.FullAPI(&out),
		node.Online(),
		node.Repo(sourceRepo),
		node.MockHost(suite.mn),
		node.Test(),

		node.Override(new(modules.Genesis), modules.LoadGenesis(genesis)),
		node.Web3FullAPI(&web3RPC),
	)
	require.NoError(suite.t, err)
	suite.t.Cleanup(func() { _ = stop(context.Background()) })
	build.RunningNodeType = build.NodeFull
	lastTs := blocks[len(blocks)-1].Blocks
	// Add structs from gen to fullNodeAPI
	// out.(*impl.FullNodeAPI).API.Chain = suite.g.ChainStore()
	// out.(*impl.FullNodeAPI).ChainAPI.Chain = suite.g.ChainStore()
	// out.(*impl.FullNodeAPI).StateAPI.Chain = suite.g.ChainStore()
	// out.(*impl.FullNodeAPI).GasAPI.Chain = suite.g.ChainStore()
	// out.(*impl.FullNodeAPI).GasAPI.Stmgr = suite.g.StateManager()
	// out.(*impl.FullNodeAPI).StateAPI.StateManager = suite.g.StateManager()
	for _, lastB := range lastTs {
		cs := out.(*impl.FullNodeAPI).ChainAPI.Chain
		require.NoError(suite.t, cs.AddToTipSetTracker(lastB.Header))
		err = cs.AddBlock(suite.ctx, lastB.Header)
		require.NoError(suite.t, err)
	}

	suite.genesis = genesis
	suite.blocks = blocks
	err = out.WalletSetDefault(suite.ctx, suite.g.Banker())
	require.NoError(suite.t, err)
	suite.fullAPI = append(suite.fullAPI, out)
	suite.rpcAPIs = append(suite.rpcAPIs, web3RPC)
}

func (suite *RPCTestUtil) addClientNode() int {
	if suite.genesis == nil {
		suite.t.Fatal("source doesn't exists")
	}

	var out api.FullNode
	var web3RPC web3.FullWeb3Interface

	stop, err := node.New(suite.ctx,
		node.FullAPI(&out),
		node.Online(),
		node.Repo(repo.NewMemory(nil)),
		node.MockHost(suite.mn),
		node.Test(),

		node.Override(new(modules.Genesis), modules.LoadGenesis(suite.genesis)),
		node.Web3FullAPI(&web3RPC),
	)
	require.NoError(suite.t, err)
	suite.t.Cleanup(func() { _ = stop(context.Background()) })

	suite.fullAPI = append(suite.fullAPI, out)
	suite.rpcAPIs = append(suite.rpcAPIs, web3RPC)
	return len(suite.fullAPI) - 1
}

func (suite *RPCTestUtil) connect(from, to int) {
	toPI, err := suite.fullAPI[to].NetAddrsListen(suite.ctx)
	require.NoError(suite.t, err)

	err = suite.fullAPI[from].NetConnect(suite.ctx, toPI)
	require.NoError(suite.t, err)
}

// Mine new blocks for chain functional
func (suite *RPCTestUtil) mineOnBlock(blk *store.FullTipSet, to int, miners []int, wait, fail bool, msgs [][]*types.SignedMessage) *store.FullTipSet {
	if miners == nil {
		for i := range suite.g.Miners {
			miners = append(miners, i)
		}
	}

	var maddrs []address.Address
	for _, i := range miners {
		maddrs = append(maddrs, suite.g.Miners[i])
	}

	var nts *store.FullTipSet
	var err error
	if msgs != nil {
		nts, err = suite.g.NextTipSetFromMinersWithMessages(blk.TipSet(), maddrs, msgs)
		require.NoError(suite.t, err)
	} else {
		mt, err := suite.g.NextTipSetFromMiners(blk.TipSet(), maddrs)
		require.NoError(suite.t, err)
		nts = mt.TipSet
	}

	if fail {
		suite.pushTsExpectErr(to, nts, true)
	} else {
		suite.pushFtsAndWait(to, nts, wait)
	}

	return nts
}

func (suite *RPCTestUtil) mineNewBlock(src int, miners []int, msgs [][]*types.SignedMessage) {
	mts := suite.mineOnBlock(suite.g.CurTipset, src, miners, true, false, msgs)
	suite.g.CurTipset = mts
}

// Add new tipset for chain (with sync) functional
func (suite *RPCTestUtil) pushFtsAndWait(to int, fts *store.FullTipSet, wait bool) {
	// TODO: would be great if we could pass a whole tipset here...
	suite.pushTsExpectErr(to, fts, false)

	if wait {
		start := time.Now()
		h, err := suite.fullAPI[to].ChainHead(suite.ctx)
		require.NoError(suite.t, err)
		for !h.Equals(fts.TipSet()) {
			time.Sleep(time.Millisecond * 50)
			h, err = suite.fullAPI[to].ChainHead(suite.ctx)
			require.NoError(suite.t, err)

			if time.Since(start) > time.Second*10 {
				suite.t.Fatal("took too long waiting for block to be accepted")
			}
		}
	}
}

func (suite *RPCTestUtil) pushTsExpectErr(to int, fts *store.FullTipSet, experr bool) {
	for _, fb := range fts.Blocks {
		var b types.BlockMsg

		// -1 to match block.Height
		b.Header = fb.Header
		for _, msg := range fb.SecpkMessages {
			c, err := suite.fullAPI[to].(*impl.FullNodeAPI).ChainAPI.Chain.PutMessage(msg)
			require.NoError(suite.t, err)

			b.SecpkMessages = append(b.SecpkMessages, c)
		}

		for _, msg := range fb.BlsMessages {
			c, err := suite.fullAPI[to].(*impl.FullNodeAPI).ChainAPI.Chain.PutMessage(msg)
			require.NoError(suite.t, err)

			b.BlsMessages = append(b.BlsMessages, c)
		}
		err := suite.fullAPI[to].SyncSubmitBlock(suite.ctx, &b)
		if experr {
			require.Error(suite.t, err, "expected submit block to fail")
		} else {
			require.NoError(suite.t, err)
		}
	}
}

func (suite *RPCTestUtil) CreateContract(t *testing.T, createParams utils.CreateContractParams) utils.CallArguments {
	// Add create contract message to chain
	args, signedMsg := utils.CreateContractOnChain(suite.ctx, t, &createParams, suite.fullAPI[0])
	// mine new tipsets
	suite.mineNewBlock(0, []int{0}, [][]*types.SignedMessage{{signedMsg}})
	// increase chaingen nonce
	suite.g.IncreaseBankerNonce()
	args.Msg.Nonce++
	// Send 10*20 messages
	for i := 0; i < extraMsgCount; i++ {
		suite.mineNewBlock(0, []int{0}, nil)
		args.Msg.Nonce += genMessages
	}

	ret, err := suite.fullAPI[0].StateGetReceipt(suite.ctx, signedMsg.Cid(), types.EmptyTSK)
	require.NoError(t, err)
	require.NotNil(t, ret)
	var result init0.ExecReturn
	if ret.ExitCode == 0 {
		err = result.UnmarshalCBOR(bytes.NewReader(ret.Return))
		require.NoError(t, err)
	}
	args.Msg.To = result.RobustAddress
	args.ToAddress = result.RobustAddress
	return args
}

func (suite *RPCTestUtil) CallContract(t *testing.T, callParams utils.CallContractParams) *contract.ContractResult {
	// Add call contract message to chain
	signedMsg := utils.CallContractOnChain(suite.ctx, t, &callParams, suite.fullAPI[0])
	suite.mineNewBlock(0, []int{0}, [][]*types.SignedMessage{{signedMsg}})
	// increase chaingen nonce
	suite.g.IncreaseBankerNonce()
	// mine new tipsets
	for i := 0; i < extraMsgCount; i++ {
		suite.mineNewBlock(0, []int{0}, nil)
	}
	ret, err := suite.fullAPI[0].StateGetReceipt(suite.ctx, callParams.Args.Msg.Cid(), types.EmptyTSK)
	callParams.Args.Msg.Nonce += genMessages*extraMsgCount + 1
	require.NoError(t, err)
	require.NotNil(t, ret)
	//Get contract result
	if ret.ExitCode == 0 {
		var result contract.ContractResult
		err = result.UnmarshalCBOR(bytes.NewReader(ret.Return))
		require.NoError(t, err)
		return &result
	}
	return nil
}

// Test Web3 functions
func (suite *RPCTestSuite) TestWeb3ClientVersion() {
	t := suite.T()
	versionRPC, err := suite.utils.rpcAPIs[0].Web3.ClientVersion(suite.utils.ctx)
	require.NoError(t, err)
	versionFullAPI, err := suite.utils.fullAPI[0].Version(suite.utils.ctx)
	require.NoError(t, err)
	require.Equal(t, versionFullAPI.Version, versionRPC)
}

func (suite *RPCTestSuite) TestWeb3SHA3() {
	t := suite.T()
	data := web3.HexString("0x6465616462656566") // string deadbeef
	shaRPC, err := suite.utils.rpcAPIs[0].Web3.Sha3(suite.utils.ctx, data)
	require.NoError(t, err)
	// expected keccak-256 value for deadbeef
	expected := "0x9f24c52e0fcd1ac696d00405c3bd5adc558c48936919ac5ab3718fcb7d70f93f"
	require.Equal(t, expected, shaRPC)
}

// Test Net functions
func (suite *RPCTestSuite) TestNetPeerCount() {
	t := suite.T()
	peerCount, err := suite.utils.rpcAPIs[0].Net.PeerCount(suite.utils.ctx)
	require.NoError(t, err)
	// Actual no peers, only localhost test
	require.Equal(t, web3.GetHexString(0), peerCount)

	//Add new client to node
	client := suite.utils.addClientNode()
	require.NoError(t, suite.utils.mn.LinkAll())
	//Connect client to our node
	suite.utils.connect(client, 0)
	peerCount, err = suite.utils.rpcAPIs[0].Net.PeerCount(suite.utils.ctx)
	require.NoError(t, err)
	require.Equal(t, web3.GetHexString(int64(client)), peerCount)
}
func (suite *RPCTestSuite) TestNetListening() {
	t := suite.T()
	listen, err := suite.utils.rpcAPIs[0].Net.Listening(suite.utils.ctx)
	require.NoError(t, err)
	require.Equal(t, true, listen)
}
func (suite *RPCTestSuite) TestNetVersion() {
	t := suite.T()
	version, err := suite.utils.rpcAPIs[0].Net.Version(suite.utils.ctx)
	require.NoError(t, err)
	expected, err := suite.utils.fullAPI[0].StateNetworkVersion(suite.utils.ctx, suite.utils.g.CurTipset.TipSet().Key())
	require.NoError(t, err)
	require.Equal(t, fmt.Sprintf("%v", expected), version)
}

// Test Eth functions
// Test EthInfo functional
func (suite *RPCTestSuite) TestEthProtocolVersion() {
	t := suite.T()
	protocolVersion, err := suite.utils.rpcAPIs[0].Eth.ProtocolVersion(suite.utils.ctx)
	require.NoError(t, err)
	expected := web3.GetHexString(eth.ETH65)
	require.Equal(t, expected, protocolVersion)
}
func (suite *RPCTestSuite) TestEthSyncing() {
	t := suite.T()
	sync, err := suite.utils.rpcAPIs[0].Eth.Syncing(suite.utils.ctx)
	require.NoError(t, err)
	// Actual no sync, only source
	expected := false
	require.Equal(t, expected, sync)
}
func (suite *RPCTestSuite) TestEthCoinbase() {
	t := suite.T()
	coinbase, err := suite.utils.rpcAPIs[0].Eth.Coinbase(suite.utils.ctx)
	require.NoError(t, err)
	// Should be a deffault wallet address
	expected, err := suite.utils.fullAPI[0].WalletDefaultAddress(suite.utils.ctx)
	require.NoError(t, err)
	require.Equal(t, expected.String(), coinbase)
}
func (suite *RPCTestSuite) TestEthMining() {
	t := suite.T()
	mining, err := suite.utils.rpcAPIs[0].Eth.Mining(suite.utils.ctx)
	require.NoError(t, err)
	// Get list of all miners
	// If it's not empty, then mining should be equal to true, otherwise false
	expected, err := suite.utils.fullAPI[0].StateListMiners(suite.utils.ctx, suite.utils.g.CurTipset.TipSet().Key())
	require.NoError(t, err)
	require.Equal(t, len(expected) != 0, mining)
}
func (suite *RPCTestSuite) TestEthGasPrice() {
	t := suite.T()
	gasPrice, err := suite.utils.rpcAPIs[0].Eth.GasPrice(suite.utils.ctx)
	require.NoError(t, err)
	// Estimate dynamic gas premium
	estimGasPremium, err := suite.utils.fullAPI[0].GasEstimateGasPremium(suite.utils.ctx, 0, address.Undef, 0, suite.utils.g.CurTipset.TipSet().Key())
	require.NoError(t, err)
	msgWithGasPremium := &types.Message{GasPremium: estimGasPremium}
	// Estimate dynamic gas price (with estimated gas premium)
	estimGasPrice, err := suite.utils.fullAPI[0].GasEstimateFeeCap(suite.utils.ctx, msgWithGasPremium, 0, suite.utils.g.CurTipset.TipSet().Key())
	require.NoError(t, err)
	// They can't be equal, because estimating add some noise
	bigGasPrice, ok := big.NewInt(0).SetString(string(gasPrice), 0)
	require.Equal(t, true, ok)
	differ := estimGasPrice.Sub(estimGasPrice.Int, bigGasPrice)
	differ = differ.Abs(differ)
	// Constant for our network
	maxDiff := int64(10000)
	require.LessOrEqual(t, differ.Int64(), maxDiff)
}
func (suite *RPCTestSuite) TestEthAccounts() {
	t := suite.T()
	accounts, err := suite.utils.rpcAPIs[0].Eth.Accounts(suite.utils.ctx)
	require.NoError(t, err)
	accountAddresses, err := suite.utils.fullAPI[0].WalletList(suite.utils.ctx)
	require.NoError(t, err)
	for i, address := range accountAddresses {
		require.Equal(t, address.String(), accounts[i])
	}
}
func (suite *RPCTestSuite) TestEthBlockNumber() {
	t := suite.T()
	blockNum, err := suite.utils.rpcAPIs[0].Eth.BlockNumber(suite.utils.ctx)
	require.NoError(t, err)
	require.Equal(t, web3.GetHexString(int64(chainHeight)), blockNum)
}
func (suite *RPCTestSuite) TestEthGetBalance() {
	t := suite.T()
	defaultAddress, err := suite.utils.fullAPI[0].WalletDefaultAddress(suite.utils.ctx)
	require.NoError(t, err)
	balance, err := suite.utils.rpcAPIs[0].Eth.GetBalance(suite.utils.ctx, defaultAddress.String(), "latest")
	require.NoError(t, err)
	actor, err := suite.utils.fullAPI[0].StateGetActor(suite.utils.ctx, defaultAddress, types.EmptyTSK)
	require.NoError(t, err)
	require.Equal(t, web3.HexString("0x"+actor.Balance.Text(16)), balance)
}
func (suite *RPCTestSuite) TestEthGetStorageAt() {
	t := suite.T()
	createParams := utils.CreateContractParams{ContractCode: tests.GetStorageAtCode}
	args := suite.utils.CreateContract(t, createParams)
	res, err := suite.utils.rpcAPIs[0].Eth.GetStorageAt(suite.utils.ctx, args.ToAddress.String(), "0x0", "latest")
	require.NoError(t, err)
	expected := web3.HexString("0x00000000000000000000000000000000000000000000000000000000000004d2")
	require.Equal(t, expected, res)
	// Update value
	callParams := utils.CallContractParams{FuncSign: tests.GetStorageUpdate, MsgValue: uint64(0), Args: args}
	_ = suite.utils.CallContract(t, callParams)
	res, err = suite.utils.rpcAPIs[0].Eth.GetStorageAt(suite.utils.ctx, args.ToAddress.String(), "0x0", "latest")
	require.NoError(t, err)
	expected = web3.HexString("0x00000000000000000000000000000000000000000000000000000000000010e1")
	require.Equal(t, expected, res)
}
func (suite *RPCTestSuite) TestEthGetTransactionCount() {
	t := suite.T()
	defaultAddress, err := suite.utils.fullAPI[0].WalletDefaultAddress(suite.utils.ctx)
	require.NoError(t, err)
	nonce, err := suite.utils.rpcAPIs[0].Eth.GetTransactionCount(suite.utils.ctx, defaultAddress.String(), "latest")
	require.NoError(t, err)
	actor, err := suite.utils.fullAPI[0].StateGetActor(suite.utils.ctx, defaultAddress, types.EmptyTSK)
	require.NoError(t, err)
	require.Equal(t, web3.GetHexString(int64(actor.Nonce)), nonce)
}

func (suite *RPCTestSuite) TestEthGetBlockByNumber() {
	t := suite.T()
	head, err := suite.utils.fullAPI[0].ChainHead(suite.utils.ctx)
	require.NoError(t, err)
	for i, block := range head.Blocks() {
		blockInfo, err := suite.utils.rpcAPIs[0].Eth.GetBlockByNumber(suite.utils.ctx, "latest", false, float64(i))
		require.NoError(t, err)
		require.Equal(t, block.Cid().String(), blockInfo.Hash)
	}
	// take random tipset
	height := int64(5)
	tipset, err := suite.utils.fullAPI[0].ChainGetTipSetByHeight(suite.utils.ctx, abi.ChainEpoch(height), types.EmptyTSK)
	require.NoError(t, err)
	for i, block := range tipset.Blocks() {
		blockInfo, err := suite.utils.rpcAPIs[0].Eth.GetBlockByNumber(suite.utils.ctx, float64(height), false, float64(i))
		require.NoError(t, err)
		require.Equal(t, block.Cid().String(), blockInfo.Hash)
	}
}
func (suite *RPCTestSuite) TestEthGetBlockByHash() {
	t := suite.T()
	head, err := suite.utils.fullAPI[0].ChainHead(suite.utils.ctx)
	require.NoError(t, err)
	for _, block := range head.Blocks() {
		blockInfo, err := suite.utils.rpcAPIs[0].Eth.GetBlockByHash(suite.utils.ctx, block.Cid().String(), false)
		require.NoError(t, err)
		require.Equal(t, block.Cid().String(), blockInfo.Hash)
	}
	// take random tipset
	height := int64(5)
	tipset, err := suite.utils.fullAPI[0].ChainGetTipSetByHeight(suite.utils.ctx, abi.ChainEpoch(height), types.EmptyTSK)
	require.NoError(t, err)
	for _, block := range tipset.Blocks() {
		blockInfo, err := suite.utils.rpcAPIs[0].Eth.GetBlockByHash(suite.utils.ctx, block.Cid().String(), false)
		require.NoError(t, err)
		require.Equal(t, block.Cid().String(), blockInfo.Hash)
	}
}
func (suite *RPCTestSuite) TestEthEstimateGas() {
	t := suite.T()
	createParams := utils.CreateContractParams{ContractCode: tests.HelloWorldContractCode}
	args := suite.utils.CreateContract(t, createParams)
	callParams := utils.CallContractParams{FuncSign: tests.HelloWorldFuncSignature, MsgValue: uint64(0), Args: args}
	ret := suite.utils.CallContract(t, callParams)
	fmt.Printf("%x", ret.Value)
	code, err := hex.DecodeString(tests.HelloWorldFuncSignature)
	require.NoError(t, err)
	enc, aerr := actors.SerializeParams(&contract.ContractParams{Code: code, Value: types.NewInt(0), Commit: false})
	require.NoError(t, aerr)
	args.Msg.Params = enc
	expected, err := suite.utils.fullAPI[0].GasEstimateGasLimit(suite.utils.ctx, args.Msg, suite.utils.g.CurTipset.TipSet().Key())
	require.NoError(t, err)
	info := web3.MessageInfo{
		From:     args.FromAddress.String(),
		To:       args.ToAddress.String(),
		Gas:      web3.GetHexString(args.Msg.GasLimit),
		GasPrice: web3.GetHexString(args.Msg.GasFeeCap.Int64()),
		Value:    web3.GetHexString(args.Msg.Value.Int64()),
		Data:     web3.HexString("0x" + tests.HelloWorldFuncSignature),
		Nonce:    web3.GetHexString(int64(args.Msg.Nonce)),
	}
	estimate, err := suite.utils.rpcAPIs[0].Eth.EstimateGas(suite.utils.ctx, info, float64(suite.utils.g.CurTipset.TipSet().Height()))
	require.NoError(t, err)
	require.Equal(t, web3.GetHexString(expected), estimate)
}
func (suite *RPCTestSuite) TestEthGetTransactionByHash() {
	t := suite.T()
	height := int64(7)
	blockInfo, err := suite.utils.rpcAPIs[0].Eth.GetBlockByNumber(suite.utils.ctx, "0x"+strconv.FormatInt(height, 16), false, float64(0))
	require.NoError(t, err)
	for _, txhash := range blockInfo.Transactions {
		txcid, err := cid.Decode(txhash.(string))
		require.NoError(t, err)
		msg, err := suite.utils.fullAPI[0].ChainGetMessage(suite.utils.ctx, txcid)
		require.NoError(t, err)
		txInfo, err := suite.utils.rpcAPIs[0].Eth.GetTransactionByHash(suite.utils.ctx, txhash.(string))
		require.NoError(t, err)
		require.Equal(t, msg.From.String(), txInfo.From)
		require.Equal(t, msg.To.String(), txInfo.To)
		require.Equal(t, web3.GetHexString(int64(msg.Nonce)), txInfo.Nonce)
		require.Equal(t, hex.EncodeToString(msg.Params), txInfo.Input)
		require.Equal(t, blockInfo.Number, txInfo.BlockNumber)
		require.Equal(t, blockInfo.Hash, txInfo.BlockHash)
	}
}
func (suite *RPCTestSuite) TestEthGetCode() {
	t := suite.T()
	createParams := utils.CreateContractParams{ContractCode: tests.StorageContractCode}
	args := suite.utils.CreateContract(t, createParams)
	res, err := suite.utils.rpcAPIs[0].Eth.GetCode(suite.utils.ctx, args.ToAddress.String(), "latest")
	require.NoError(t, err)
	// after creation evm should return a byte code for created contract
	// it's not equal to ContractCode
	// for details see 'create' EVM method
	expected := web3.HexString("0x608060405234801561001057600080fd5b50600436106100415760003560e01c80633944e8121461004657806378cf36971461007e578063a6472b53146100c0575b600080fd5b61007c6004803603604081101561005c57600080fd5b8101908080359060200190929190803590602001909291905050506100ee565b005b6100aa6004803603602081101561009457600080fd5b8101908080359060200190929190505050610109565b6040518082815260200191505060405180910390f35b6100ec600480360360208110156100d657600080fd5b8101908080359060200190929190505050610125565b005b80600080848152602001908152602001600020819055505050565b6000806000838152602001908152602001600020549050919050565b600080828152602001908152602001600020600090555056fea265627a7a72315820f271d4783876efbda566e5704efb8032d5b576217a25b9a40029854e2bfda14964736f6c63430005110032")
	require.Equal(t, expected, res)
}

// from 0 to 9 all blocks have exactly 20 transactions, check gen methods for details
func (suite *RPCTestSuite) TestEthGetBlockTransactionCountByHash() {
	t := suite.T()
	height := int64(7)
	blockInfo, err := suite.utils.rpcAPIs[0].Eth.GetBlockByNumber(suite.utils.ctx, "0x"+strconv.FormatInt(height, 16), false, float64(0))
	require.NoError(t, err)
	count, err := suite.utils.rpcAPIs[0].Eth.GetBlockTransactionCountByHash(suite.utils.ctx, blockInfo.Hash)
	require.NoError(t, err)
	require.Equal(t, web3.GetHexString(genMessages), count)
}
func (suite *RPCTestSuite) TestEthGetBlockTransactionCountByNumber() {
	t := suite.T()
	height := int64(7)
	count, err := suite.utils.rpcAPIs[0].Eth.GetBlockTransactionCountByNumber(suite.utils.ctx, float64(height), float64(0))
	require.NoError(t, err)
	require.Equal(t, web3.GetHexString(genMessages), count)
}
func (suite *RPCTestSuite) TestEthGetTransactionByBlockHashAndIndex() {
	t := suite.T()
	height := int64(7)
	blockInfo, err := suite.utils.rpcAPIs[0].Eth.GetBlockByNumber(suite.utils.ctx, "0x"+strconv.FormatInt(height, 16), false, float64(0))
	require.NoError(t, err)
	for i, txhash := range blockInfo.Transactions {
		expectedTxInfo, err := suite.utils.rpcAPIs[0].Eth.GetTransactionByHash(suite.utils.ctx, txhash.(string))
		require.NoError(t, err)
		actualTxInfo, err := suite.utils.rpcAPIs[0].Eth.GetTransactionByBlockHashAndIndex(suite.utils.ctx, blockInfo.Hash, float64(i))
		require.NoError(t, err)
		require.Equal(t, expectedTxInfo, actualTxInfo)
	}
}
func (suite *RPCTestSuite) TestEthGetTransactionByBlockNumberAndIndex() {
	t := suite.T()
	height := int64(7)
	blockInfo, err := suite.utils.rpcAPIs[0].Eth.GetBlockByNumber(suite.utils.ctx, "0x"+strconv.FormatInt(height, 16), false, float64(0))
	require.NoError(t, err)
	for i, txhash := range blockInfo.Transactions {
		expectedTxInfo, err := suite.utils.rpcAPIs[0].Eth.GetTransactionByHash(suite.utils.ctx, txhash.(string))
		require.NoError(t, err)
		actualTxInfo, err := suite.utils.rpcAPIs[0].Eth.GetTransactionByBlockNumberAndIndex(suite.utils.ctx, float64(height), float64(i), float64(0))
		require.NoError(t, err)
		require.Equal(t, expectedTxInfo, actualTxInfo)
	}
}

func (suite *RPCTestSuite) TestEthGetTransactionReceipt() {
	t := suite.T()
	height := int64(7)
	blockInfo, err := suite.utils.rpcAPIs[0].Eth.GetBlockByNumber(suite.utils.ctx, "0x"+strconv.FormatInt(height, 16), false, float64(0))
	require.NoError(t, err)
	for i, txhash := range blockInfo.Transactions {
		txcid, err := cid.Decode(txhash.(string))
		require.NoError(t, err)
		msg, err := suite.utils.fullAPI[0].ChainGetMessage(suite.utils.ctx, txcid)
		require.NoError(t, err)
		receipt, err := suite.utils.rpcAPIs[0].Eth.GetTransactionReceipt(suite.utils.ctx, txhash.(string))
		require.NoError(t, err)
		require.Equal(t, web3.GetHexString(0), receipt.BlockIndex)
		require.Equal(t, blockInfo.Hash, receipt.BlockHash)
		require.Equal(t, txhash, receipt.TransactionHash)
		require.Equal(t, msg.From.String(), receipt.From)
		require.Equal(t, msg.To.String(), receipt.To)
		require.Equal(t, web3.GetHexString(int64(i)), receipt.TransactionIndex)
	}
}

// Tests for new filter + getLogs + getFilterLogs + unsubscribe + GetFilterChanges
func (suite *RPCTestSuite) TestEthLogsFilterFunctional() {
	t := suite.T()
	createParams := utils.CreateContractParams{ContractCode: tests.SimpleEventCode}
	args := suite.utils.CreateContract(t, createParams)
	filter := web3.NewFilterInfo{
		FromBlock: web3.GetHexString(0),
		ToBlock:   web3.GetHexString(40),
		Address:   []string{hex.EncodeToString(args.ToAddress.Payload())},
	}
	id, err := suite.utils.rpcAPIs[0].Eth.NewFilter(suite.utils.ctx, filter)
	require.NoError(t, err)
	// Emit event
	callParams := utils.CallContractParams{FuncSign: tests.SimpleEventDeposit, MsgValue: uint64(0), Args: args}
	_ = suite.utils.CallContract(t, callParams)
	// getFilterLogs
	changes, err := suite.utils.rpcAPIs[0].Eth.GetFilterChanges(suite.utils.ctx, id)
	require.NoError(t, err)
	require.Equal(t, 1, len(changes.([]web3.LogsView)))
	require.Equal(t, "0x"+hex.EncodeToString(args.ToAddress.Payload()), changes.([]web3.LogsView)[0].Address)
	// getFilterLogs
	res, err := suite.utils.rpcAPIs[0].Eth.GetFilterLogs(suite.utils.ctx, id)
	require.NoError(t, err)
	require.Equal(t, changes, res)
	// GetLogs
	res, err = suite.utils.rpcAPIs[0].Eth.GetLogs(suite.utils.ctx, web3.FilterQuery{NewFilterInfo: filter})
	require.NoError(t, err)
	require.Equal(t, changes, res)
	// Uninstall filter
	uninst, err := suite.utils.rpcAPIs[0].Eth.UninstallFilter(suite.utils.ctx, id)
	require.NoError(t, err)
	require.True(t, uninst)
	_, err = suite.utils.rpcAPIs[0].Eth.GetFilterLogs(suite.utils.ctx, id)
	require.Error(t, err)
}

func (suite *RPCTestSuite) TestEthNewBlockFilter() {
	t := suite.T()
	createParams := utils.CreateContractParams{ContractCode: tests.SimpleEventCode}
	args := suite.utils.CreateContract(t, createParams)
	id, err := suite.utils.rpcAPIs[0].Eth.NewBlockFilter(suite.utils.ctx)
	require.NoError(t, err)
	head, err := suite.utils.fullAPI[0].ChainHead(suite.utils.ctx)
	require.NoError(t, err)
	callParams := utils.CallContractParams{FuncSign: tests.SimpleEventDeposit, MsgValue: uint64(0), Args: args}
	_ = suite.utils.CallContract(t, callParams)
	// getFilterChanges
	changes, err := suite.utils.rpcAPIs[0].Eth.GetFilterChanges(suite.utils.ctx, id)
	require.NoError(t, err)
	require.Equal(t, extraMsgCount+2, len(changes.([]string)))
	for i := 0; i < extraMsgCount+2; i++ {
		blockCidStr := strings.Trim(changes.([]string)[i], "[]")
		blockCid, err := cid.Decode(blockCidStr)
		require.NoError(t, err)
		header, err := suite.utils.fullAPI[0].ChainGetBlock(suite.utils.ctx, blockCid)
		require.NoError(t, err)
		require.Equal(t, int(head.Height())+i, int(header.Height))
	}
}

func (suite *RPCTestSuite) TestEthCall() {
	t := suite.T()
	createParams := utils.CreateContractParams{ContractCode: tests.GetStorageAtCode}
	args := suite.utils.CreateContract(t, createParams)
	res, err := suite.utils.rpcAPIs[0].Eth.GetStorageAt(suite.utils.ctx, args.ToAddress.String(), "0x0", "latest")
	require.NoError(t, err)
	expected := web3.HexString("0x00000000000000000000000000000000000000000000000000000000000004d2")
	require.Equal(t, expected, res)
	// Update value
	info := web3.MessageInfo{
		From:     args.FromAddress.String(),
		To:       args.ToAddress.String(),
		Gas:      web3.GetHexString(args.Msg.GasLimit),
		GasPrice: web3.GetHexString(args.Msg.GasFeeCap.Int64()),
		Value:    web3.GetHexString(args.Msg.Value.Int64()),
		Data:     web3.HexString("0x" + tests.GetStorageUpdate),
		Nonce:    web3.GetHexString(int64(args.Msg.Nonce)),
	}
	// Call, nothing should change
	_, err = suite.utils.rpcAPIs[0].Eth.Call(suite.utils.ctx, info, "latest")
	require.NoError(t, err)
	res, err = suite.utils.rpcAPIs[0].Eth.GetStorageAt(suite.utils.ctx, args.ToAddress.String(), "0x0", "latest")
	require.NoError(t, err)
	expected = web3.HexString("0x00000000000000000000000000000000000000000000000000000000000004d2")
	require.Equal(t, expected, res)
}

// TO DO: chainGen doesn's work with mpool, this tests work with it

// func (suite *RPCTestSuite) TestEthNewPendingTransactionFilter() {
// 	t := suite.T()
// 	id, err := suite.utils.rpcAPIs[0].Eth.NewPendingTransactionFilter(suite.utils.ctx)
// 	require.NoError(t, err)
// 	createParams := utils.CreateContractParams{ContractCode: tests.SimpleEventCode}
// 	_, signedMsg := utils.CreateContractOnChain(t, &createParams, suite.utils.fullAPI[0], &suite.utils.chainParams)
// 	expectedMsgCid, err := suite.utils.fullAPI[0].MpoolPush(suite.utils.ctx, signedMsg)
// 	require.NoError(t, err)
// 	// getFilterChanges
// 	changes, err := suite.utils.rpcAPIs[0].Eth.GetFilterChanges(suite.utils.ctx, id)
// 	require.NoError(t, err)
// 	require.Equal(t, 1, len(changes.([]string)))
// 	msgCidStr := strings.Trim(changes.([]string)[0], "[]")
// 	msgCid, err := cid.Decode(msgCidStr)
// 	require.NoError(t, err)
// 	require.True(t, expectedMsgCid == msgCid)
// }

// Also test eth_sendRawTransaction
// func (suite *RPCTestSuite) TestEthSignTransaction() {
// 	t := suite.T()
// 	createParams := utils.CreateContractParams{ContractCode: tests.GetStorageAtCode}
// 	args := suite.utils.CreateContract(t, createParams)
// 	info := web3.MessageInfo{
// 		From:     args.FromAddress.String(),
// 		To:       args.ToAddress.String(),
// 		Gas:      web3.GetHexString(args.Msg.GasLimit),
// 		GasPrice: web3.GetHexString(args.Msg.GasFeeCap.Int64()),
// 		Value:    web3.GetHexString(args.Msg.Value.Int64()),
// 		Data:     web3.HexString("0x" + tests.GetStorageUpdate),
// 		Nonce:    web3.GetHexString(int64(args.Msg.Nonce)),
// 	}
// 	signedTx, err := suite.utils.rpcAPIs[0].Eth.SignTransaction(suite.utils.ctx, info)
// 	require.NoError(t, err)
// 	// Add message to message pool
// 	_, err = suite.utils.rpcAPIs[0].Eth.SendRawTransaction(suite.utils.ctx, signedTx)
// 	require.NoError(t, err)
// 	// Update value
// 	res, err := suite.utils.rpcAPIs[0].Eth.GetStorageAt(suite.utils.ctx, args.ToAddress.String(), "0x0", "latest")
// 	require.NoError(t, err)
// 	expected := web3.HexString("0x00000000000000000000000000000000000000000000000000000000000010e1")
// 	require.Equal(t, expected, res)
// }

// func (suite *RPCTestSuite) TestEthSendTransaction() {
// 	t := suite.T()
// 	createParams := utils.CreateContractParams{ContractCode: tests.GetStorageAtCode}
// 	args := suite.utils.CreateContract(t, createParams)
// 	info := web3.MessageInfo{
// 		From:     args.FromAddress.String(),
// 		To:       args.ToAddress.String(),
// 		Gas:      web3.GetHexString(args.Msg.GasLimit),
// 		GasPrice: web3.GetHexString(args.Msg.GasFeeCap.Int64()),
// 		Value:    web3.GetHexString(args.Msg.Value.Int64()),
// 		Data:     web3.HexString("0x" + tests.GetStorageUpdate),
// 		Nonce:    web3.GetHexString(int64(args.Msg.Nonce)),
// 	}
// 	_, err := suite.utils.rpcAPIs[0].Eth.SendTransaction(suite.utils.ctx, info)
// 	require.NoError(t, err)
// 	// Update value
// 	res, err := suite.utils.rpcAPIs[0].Eth.GetStorageAt(suite.utils.ctx, args.ToAddress.String(), "0x0", "latest")
// 	require.NoError(t, err)
// 	expected := web3.HexString("0x00000000000000000000000000000000000000000000000000000000000010e1")
// 	require.Equal(t, expected, res)
// }

//TO DO: add test with ethrecover
func (suite *RPCTestSuite) TestEthSign() {
	t := suite.T()
	data := []byte{0xde, 0xad, 0xbe, 0xef}
	msg := fmt.Sprintf("\x19Ethereum Signed Message:\n%d%s", len(data), string(data))
	msgHash := crypto.Keccak256([]byte(msg))
	ethSign, err := suite.utils.rpcAPIs[0].Eth.Sign(suite.utils.ctx, suite.utils.g.Banker().String(), web3.HexString("0xdeadbeef"))
	require.NoError(t, err)
	sign, err := suite.utils.fullAPI[0].WalletSign(suite.utils.ctx, suite.utils.g.Banker(), msgHash)
	require.NoError(t, err)
	require.Equal(t, web3.HexString("0x"+hex.EncodeToString(sign.Data)), ethSign)
}

func TestRPCFunctional(t *testing.T) {
	suite.Run(t, newRPCTestSuite(t))
}
