package vm_test

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"os"
	"testing"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/lotus/build"
	"github.com/filecoin-project/lotus/chain/actors"
	"github.com/filecoin-project/lotus/chain/actors/aerrors"
	"github.com/filecoin-project/lotus/chain/actors/policy"
	"github.com/filecoin-project/lotus/chain/gen"
	"github.com/filecoin-project/lotus/chain/types"
	c "github.com/filecoin-project/lotus/conformance"
	"github.com/filecoin-project/test-vectors/schema"
	"github.com/filestar-project/evm-adapter/tests"
	"github.com/filestar-project/evm-adapter/tests/mocks"
	r "github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	verifreg0 "github.com/filecoin-project/specs-actors/actors/builtin/verifreg"
	"github.com/filecoin-project/specs-actors/v2/actors/builtin"
	contract "github.com/filecoin-project/specs-actors/v2/actors/builtin/contract"
	init0 "github.com/filecoin-project/specs-actors/v2/actors/builtin/init"
	"github.com/ipfs/go-cid"
	logging "github.com/ipfs/go-log"
)

var log = logging.Logger("test")

type EvmContractSuite struct {
	suite.Suite
}

type ChainParams struct {
	root      cid.Cid
	baseEpoch abi.ChainEpoch
	cg        *gen.ChainGen
	drive     *c.Driver
	ctx       context.Context
	tipset    *gen.MinedTipSet
}

type CreateContractParams struct {
	contractCode string
}

type CallArguments struct {
	msg         *types.Message
	toAddress   address.Address
	fromAddress address.Address
}
type CallContractParams struct {
	args     CallArguments
	funcSign string
	msgValue uint64
}

func newEvmContractSuite() *EvmContractSuite {
	return &EvmContractSuite{}
}

// use init() from other chain/vm tests
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

func (suite *EvmContractSuite) initChain() ChainParams {
	t := suite.T()
	logging.SetAllLoggers(logging.LevelDebug)
	verifreg0.MinVerifiedDealSize = big.Zero()
	var (
		ctx       = context.Background()
		baseEpoch = abi.ChainEpoch(50)
	)
	//Generate test chain
	cg, err := gen.NewGenerator()
	r.NoError(t, err)
	for i := 0; i < 5; i++ {
		_, err := cg.NextTipSet()
		if err != nil {
			t.Fatal(err)
		}
	}
	ts, err := cg.NextTipSet()
	r.NoError(t, err)
	root := ts.TipSet.TipSet().ParentState()
	d := c.NewDriver(ctx, schema.Selector{}, c.DriverOpts{})
	return ChainParams{root: root, baseEpoch: baseEpoch, ctx: ctx, cg: cg, tipset: ts, drive: d}
}

func (suite *EvmContractSuite) createContract(params *CreateContractParams, chainParams *ChainParams) CallArguments {
	t := suite.T()
	//Init params to create contract
	code, err := hex.DecodeString(params.contractCode)
	r.NoError(t, err)
	//Load addresses for tests
	fromAddress := chainParams.cg.Banker()
	sm := chainParams.cg.StateManager()
	act, err := sm.LoadActor(chainParams.ctx, fromAddress, chainParams.tipset.TipSet.TipSet())
	r.NoError(t, err)
	contractEnc, err := actors.SerializeParams(&contract.ContractParams{Code: code, Value: big.NewInt(0), CommitStatus: true})
	r.NoError(t, err)
	enc, err := actors.SerializeParams(&init0.ExecParams{CodeCID: builtin.ContractActorCodeID, ConstructorParams: contractEnc})
	r.NoError(t, err)
	//Build message to create contract
	msg := &types.Message{
		From:       fromAddress,
		To:         builtin.InitActorAddr,
		Method:     builtin.MethodsInit.Exec,
		Params:     enc,
		GasLimit:   types.TestGasLimit,
		Value:      types.NewInt(0),
		GasPremium: types.NewInt(0),
		GasFeeCap:  types.NewInt(0),
		Nonce:      act.Nonce,
	}
	//Execute create contract
	ret, root, err := chainParams.drive.ExecuteMessage(chainParams.cg.ChainStore().Blockstore(), c.ExecuteMessageParams{
		Preroot:    chainParams.root,
		Epoch:      chainParams.baseEpoch,
		Message:    msg,
		BaseFee:    c.DefaultBaseFee,
		CircSupply: c.DefaultCirculatingSupply,
	})
	msg.Nonce++
	chainParams.root = root
	r.NoError(t, err)
	r.NotNil(t, ret)
	var result init0.ExecReturn
	if ret.ActorErr == nil {
		err = result.UnmarshalCBOR(bytes.NewReader(ret.MessageReceipt.Return))
		r.NoError(t, err)
	}
	msg.To = result.RobustAddress
	return CallArguments{msg: msg, toAddress: msg.To, fromAddress: fromAddress}
}

func (suite *EvmContractSuite) callContract(callParams *CallContractParams, chainParams *ChainParams) ([]byte, aerrors.ActorError) {
	t := suite.T()
	// Try to call contract
	callParams.args.msg.Method = builtin.MethodsContract.CallContract
	//Build function signature
	funcSig, err := hex.DecodeString(callParams.funcSign)
	r.NoError(t, err)
	//Add contract params
	enc, err := actors.SerializeParams(&contract.ContractParams{Code: funcSig, Value: types.NewInt(callParams.msgValue), CommitStatus: true})
	r.NoError(t, err)
	callParams.args.msg.Params = enc
	//Execute call contract with funcSig
	ret, rt, err := chainParams.drive.ExecuteMessage(chainParams.cg.ChainStore().Blockstore(), c.ExecuteMessageParams{
		Preroot:    chainParams.root,
		Epoch:      chainParams.baseEpoch + 1,
		Message:    callParams.args.msg,
		BaseFee:    c.DefaultBaseFee,
		CircSupply: c.DefaultCirculatingSupply,
	})
	chainParams.root = rt

	r.NoError(t, err)
	r.NotNil(t, ret)

	//Get contract result
	if ret.ActorErr == nil {
		var result contract.ContractResult
		err = result.UnmarshalCBOR(bytes.NewReader(ret.MessageReceipt.Return))
		r.NoError(t, err)
		return result.Value, nil
	}
	return nil, ret.ActorErr
}

func (suite *EvmContractSuite) TestSuicideContract() {
	t := suite.T()
	createParams := CreateContractParams{contractCode: tests.HelloWorldContractCode}
	chainParams := suite.initChain()
	args := suite.createContract(&createParams, &chainParams)
	callParams := CallContractParams{funcSign: tests.HelloWorldFuncSignature, msgValue: uint64(0), args: args}
	//HelloWorld function test
	result, _ := suite.callContract(&callParams, &chainParams)
	stringReturn := string(result)
	r.Contains(t, stringReturn, tests.HelloWorldFuncReturn)
	//Suicide function test
	callParams.args.msg.Nonce++
	callParams.funcSign = tests.HelloWorldSelfdestructSignature
	result, _ = suite.callContract(&callParams, &chainParams)
	//Test that contract dies
	callParams.args.msg.Nonce++
	callParams.funcSign = tests.HelloWorldFuncSignature
	result, err := suite.callContract(&callParams, &chainParams)
	r.Nil(t, result)
	stringReturn = string(err.Error())
	r.Contains(t, stringReturn, "no such actor")
}

func (suite *EvmContractSuite) TestERC20Contract() {
	t := suite.T()
	createParams := CreateContractParams{contractCode: tests.ERC20ContractCode}
	chainParams := suite.initChain()
	args := suite.createContract(&createParams, &chainParams)
	callParams := CallContractParams{funcSign: tests.ERC20totalSupplyFuncSignature, msgValue: uint64(0), args: args}
	// TotalSupply function test
	result, _ := suite.callContract(&callParams, &chainParams)
	parsedValue, isCorrect := big.NewInt(0).SetString(fmt.Sprintf("%x", result), 16)
	r.True(t, isCorrect)
	r.Equal(t, big.NewInt(tests.ERC20totalSupply), big.NewInt(parsedValue.Int64()))
	//Transfer function test
	callParams.args.msg.Nonce++
	callParams.funcSign = tests.ERC20transferFuncSignature + mocks.ConvertPayload(callParams.args.toAddress.Payload()) + tests.Uint256Number10
	result, _ = suite.callContract(&callParams, &chainParams)
	parsedValue, isCorrect = big.NewInt(0).SetString(fmt.Sprintf("%x", result), 16)
	r.True(t, isCorrect)
	r.Equal(t, big.NewInt(1), big.NewInt(parsedValue.Int64()))
	// Balance function test
	callParams.args.msg.Nonce++
	callParams.funcSign = tests.ERC20balanceOfFuncSignature + mocks.ConvertPayload(callParams.args.fromAddress.Payload()) + tests.Uint256Number10
	result, _ = suite.callContract(&callParams, &chainParams)
	parsedValue, isCorrect = big.NewInt(0).SetString(fmt.Sprintf("%x", result), 16)
	r.True(t, isCorrect)
	r.Equal(t, big.NewInt(99990), big.NewInt(parsedValue.Int64()))
	// Approve function test
	callParams.args.msg.Nonce++
	callParams.funcSign = tests.ERC20approveFuncSignature + mocks.ConvertPayload(callParams.args.fromAddress.Payload()) + tests.Uint256Number21
	result, _ = suite.callContract(&callParams, &chainParams)
	parsedValue, isCorrect = big.NewInt(0).SetString(fmt.Sprintf("%x", result), 16)
	r.True(t, isCorrect)
	r.Equal(t, big.NewInt(1), big.NewInt(parsedValue.Int64()))
	// TransferFrom function test begin
	callParams.args.msg.Nonce++
	callParams.funcSign = tests.ERC20transferFromFuncSignature + mocks.ConvertPayload(callParams.args.fromAddress.Payload()) + mocks.ConvertPayload(callParams.args.toAddress.Payload()) + tests.Uint256Number1
	result, _ = suite.callContract(&callParams, &chainParams)
	parsedValue, isCorrect = big.NewInt(0).SetString(fmt.Sprintf("%x", result), 16)
	r.True(t, isCorrect)
	r.Equal(t, big.NewInt(1), big.NewInt(parsedValue.Int64()))
}

func (suite *EvmContractSuite) TestStorageContract() {
	t := suite.T()
	createParams := CreateContractParams{contractCode: tests.StorageContractCode}
	chainParams := suite.initChain()
	args := suite.createContract(&createParams, &chainParams)
	callParams := CallContractParams{funcSign: tests.StorageSetFuncSignature + tests.Uint256Number1 + tests.Uint256Number10, msgValue: uint64(0), args: args}
	// Set function test
	result, _ := suite.callContract(&callParams, &chainParams)
	// Get function test
	callParams.args.msg.Nonce++
	callParams.funcSign = tests.StorageGetFuncSignature + tests.Uint256Number1
	result, _ = suite.callContract(&callParams, &chainParams)
	parsedValue, isCorrect := big.NewInt(0).SetString(fmt.Sprintf("%x", result), 16)
	r.True(t, isCorrect)
	r.Equal(t, big.NewInt(10), big.NewInt(parsedValue.Int64()))
	// Remove function test
	callParams.args.msg.Nonce++
	callParams.funcSign = tests.StorageRemoveFuncSignature + tests.Uint256Number1
	result, _ = suite.callContract(&callParams, &chainParams)
	// Test Get after Remove
	callParams.args.msg.Nonce++
	callParams.funcSign = tests.StorageGetFuncSignature + tests.Uint256Number1
	result, _ = suite.callContract(&callParams, &chainParams)
	parsedValue, isCorrect = big.NewInt(0).SetString(fmt.Sprintf("%x", result), 16)
	r.True(t, isCorrect)
	r.Equal(t, big.NewInt(0), big.NewInt(parsedValue.Int64()))
}

func (suite *EvmContractSuite) TestLargeStorageContract() {
	t := suite.T()
	createParams := CreateContractParams{contractCode: tests.LargeContractCode}
	chainParams := suite.initChain()
	args := suite.createContract(&createParams, &chainParams)
	callParams := CallContractParams{funcSign: tests.LargeSetStorageFuncSignature, msgValue: uint64(0), args: args}
	// LargeSet function test
	result, _ := suite.callContract(&callParams, &chainParams)
	// GetArray function test
	callParams.args.msg.Nonce++
	callParams.funcSign = tests.LargeGetArrayFuncSignature
	result, _ = suite.callContract(&callParams, &chainParams)
	for i := 0; i < tests.LargeGetArraySize; i++ {
		left := 32 * i
		right := left + 32
		parsedValue, isCorrect := big.NewInt(0).SetString(fmt.Sprintf("%x", result[left:right]), 16)
		r.True(t, isCorrect)
		r.Equal(t, big.NewInt(int64(i)), big.NewInt(parsedValue.Int64()))
	}
	// GetString function test
	callParams.args.msg.Nonce++
	callParams.funcSign = tests.LargeGetStringFuncSignature
	result, _ = suite.callContract(&callParams, &chainParams)
	stringReturn := string(result)
	r.Contains(t, stringReturn, tests.LargeGetStringReturn)
}

func (suite *EvmContractSuite) TestErrorContract() {
	t := suite.T()
	createParams := CreateContractParams{contractCode: tests.ErrorContractCode}
	chainParams := suite.initChain()
	args := suite.createContract(&createParams, &chainParams)
	callParams := CallContractParams{funcSign: tests.SuccessOrErrorFuncSignature, msgValue: uint64(10000), args: args}
	//Success function test
	result, _ := suite.callContract(&callParams, &chainParams)
	// GetI function test
	callParams.args.msg.Nonce++
	callParams.funcSign = tests.GetIFuncSignature
	callParams.msgValue = 0
	result, _ = suite.callContract(&callParams, &chainParams)
	parsedValue, isCorrect := big.NewInt(0).SetString(fmt.Sprintf("%x", result), 16)
	r.True(t, isCorrect)
	r.Equal(t, big.NewInt(10000), big.NewInt(parsedValue.Int64()))
	// Error function test
	callParams.args.msg.Nonce++
	callParams.funcSign = tests.SuccessOrErrorFuncSignature
	callParams.msgValue = 10
	result, err := suite.callContract(&callParams, &chainParams)
	r.NotNil(t, err)
	// GetI function test
	callParams.args.msg.Nonce++
	callParams.funcSign = tests.GetIFuncSignature
	callParams.msgValue = 0
	result, _ = suite.callContract(&callParams, &chainParams)
	parsedValue, isCorrect = big.NewInt(0).SetString(fmt.Sprintf("%x", result), 16)
	r.True(t, isCorrect)
	r.Equal(t, big.NewInt(10000), big.NewInt(parsedValue.Int64()))
}

func TestEvmContract(t *testing.T) {
	suite.Run(t, newEvmContractSuite())
}
