package vm_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/lotus/build"
	"github.com/filecoin-project/lotus/chain/actors/aerrors"
	"github.com/filecoin-project/lotus/chain/actors/policy"
	utils "github.com/filecoin-project/lotus/chain/contractsutils"
	"github.com/filecoin-project/lotus/chain/gen"
	"github.com/filecoin-project/lotus/chain/state"
	"github.com/filecoin-project/lotus/chain/types"
	c "github.com/filecoin-project/lotus/conformance"
	"github.com/filecoin-project/lotus/lib/bufbstore"
	"github.com/filecoin-project/test-vectors/schema"
	"github.com/filestar-project/evm-adapter/tests"
	"github.com/filestar-project/evm-adapter/tests/mocks"
	r "github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	verifreg0 "github.com/filecoin-project/specs-actors/actors/builtin/verifreg"
	"github.com/filecoin-project/specs-actors/v2/actors/builtin/contract"
	cbor "github.com/ipfs/go-ipld-cbor"
	logging "github.com/ipfs/go-log"
)

type EvmContractSuite struct {
	suite.Suite
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

func (suite *EvmContractSuite) initChain() utils.ChainParams {
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
	return utils.ChainParams{Root: root, BaseEpoch: baseEpoch, Ctx: ctx, Cg: cg, Tipset: ts.TipSet, Drive: d}
}

func (suite *EvmContractSuite) getActor(addr address.Address, chainParams *utils.ChainParams) (*types.Actor, aerrors.ActorError) {
	buf := bufbstore.NewBufferedBstore(chainParams.Cg.ChainStore().Blockstore())
	cst := cbor.NewCborStore(buf)
	state, err := state.LoadStateTree(cst, chainParams.Root)
	if err != nil {
		return &types.Actor{}, aerrors.HandleExternalError(err, "can't load stateTree")
	}

	act, err := state.GetActor(addr)
	if err != nil {
		return act, aerrors.Absorb(err, 1, "failed to find actor")
	}

	return act, nil
}

func (suite *EvmContractSuite) getBalance(addr address.Address, chainParams *utils.ChainParams) (big.Int, aerrors.ActorError) {
	act, err := suite.getActor(addr, chainParams)
	if err != nil {
		return types.EmptyInt, err
	}
	return act.Balance, nil
}

func (suite *EvmContractSuite) TestSuicideContract() {
	t := suite.T()
	createParams := utils.CreateContractParams{ContractCode: tests.HelloWorldContractCode}
	chainParams := suite.initChain()
	args := utils.CreateContract(t, &createParams, &chainParams)
	callParams := utils.CallContractParams{FuncSign: tests.HelloWorldFuncSignature, MsgValue: uint64(0), Args: args}
	//Check balance of sender
	//TODO: Add balance for toAddress
	balanceTo, err := suite.getBalance(callParams.Args.ToAddress, &chainParams)
	r.Nil(t, err)
	balanceFrom, err := suite.getBalance(callParams.Args.FromAddress, &chainParams)
	r.Nil(t, err)
	//HelloWorld function test
	result, _ := utils.CallContract(t, &callParams, &chainParams)
	stringReturn := string(result.Value)
	r.Contains(t, stringReturn, tests.HelloWorldFuncReturn)
	//Suicide function test
	callParams.Args.Msg.Nonce++
	callParams.FuncSign = tests.HelloWorldSelfdestructSignature
	_, _ = utils.CallContract(t, &callParams, &chainParams)
	//check, that contract transfer coins
	balanceNew, err := suite.getBalance(callParams.Args.FromAddress, &chainParams)
	r.Nil(t, err)
	expected := big.NewInt(0).Add(balanceFrom.Int, balanceTo.Int).String()
	r.Equal(t, expected, balanceNew.String())
	//Test that contract dies
	_, err = suite.getActor(callParams.Args.ToAddress, &chainParams)
	r.Error(t, err)
}

func (suite *EvmContractSuite) TestERC20Contract() {
	t := suite.T()
	createParams := utils.CreateContractParams{ContractCode: tests.ERC20ContractCode}
	chainParams := suite.initChain()
	args := utils.CreateContract(t, &createParams, &chainParams)
	callParams := utils.CallContractParams{FuncSign: tests.ERC20totalSupplyFuncSignature, MsgValue: uint64(0), Args: args}
	// TotalSupply function test
	result, err := utils.CallContract(t, &callParams, &chainParams)
	r.NoError(t, err)
	parsedValue, isCorrect := big.NewInt(0).SetString(fmt.Sprintf("%x", result.Value), 16)
	r.True(t, isCorrect)
	r.Equal(t, big.NewInt(tests.ERC20totalSupply), big.NewInt(parsedValue.Int64()))
	//Transfer function test
	callParams.Args.Msg.Nonce++
	callParams.FuncSign = tests.ERC20transferFuncSignature + mocks.ConvertPayload(callParams.Args.ToAddress.Payload()) + tests.Uint256Number10
	result, _ = utils.CallContract(t, &callParams, &chainParams)
	parsedValue, isCorrect = big.NewInt(0).SetString(fmt.Sprintf("%x", result.Value), 16)
	r.True(t, isCorrect)
	r.Equal(t, big.NewInt(1), big.NewInt(parsedValue.Int64()))
	// Balance function test
	callParams.Args.Msg.Nonce++
	callParams.FuncSign = tests.ERC20balanceOfFuncSignature + mocks.ConvertPayload(callParams.Args.FromAddress.Payload()) + tests.Uint256Number10
	result, _ = utils.CallContract(t, &callParams, &chainParams)
	parsedValue, isCorrect = big.NewInt(0).SetString(fmt.Sprintf("%x", result.Value), 16)
	r.True(t, isCorrect)
	r.Equal(t, big.NewInt(99990), big.NewInt(parsedValue.Int64()))
	// Approve function test
	callParams.Args.Msg.Nonce++
	callParams.FuncSign = tests.ERC20approveFuncSignature + mocks.ConvertPayload(callParams.Args.FromAddress.Payload()) + tests.Uint256Number21
	result, _ = utils.CallContract(t, &callParams, &chainParams)
	parsedValue, isCorrect = big.NewInt(0).SetString(fmt.Sprintf("%x", result.Value), 16)
	r.True(t, isCorrect)
	r.Equal(t, big.NewInt(1), big.NewInt(parsedValue.Int64()))
	// TransferFrom function test begin
	callParams.Args.Msg.Nonce++
	callParams.FuncSign = tests.ERC20transferFromFuncSignature + mocks.ConvertPayload(callParams.Args.FromAddress.Payload()) + mocks.ConvertPayload(callParams.Args.ToAddress.Payload()) + tests.Uint256Number1
	result, _ = utils.CallContract(t, &callParams, &chainParams)
	parsedValue, isCorrect = big.NewInt(0).SetString(fmt.Sprintf("%x", result.Value), 16)
	r.True(t, isCorrect)
	r.Equal(t, big.NewInt(1), big.NewInt(parsedValue.Int64()))
}

func (suite *EvmContractSuite) TestStorageContract() {
	t := suite.T()
	createParams := utils.CreateContractParams{ContractCode: tests.StorageContractCode}
	chainParams := suite.initChain()
	args := utils.CreateContract(t, &createParams, &chainParams)
	callParams := utils.CallContractParams{FuncSign: tests.StorageSetFuncSignature + tests.Uint256Number1 + tests.Uint256Number10, MsgValue: uint64(0), Args: args}
	// Set function test
	_, _ = utils.CallContract(t, &callParams, &chainParams)
	// Get function test
	callParams.Args.Msg.Nonce++
	callParams.FuncSign = tests.StorageGetFuncSignature + tests.Uint256Number1
	result, _ := utils.CallContract(t, &callParams, &chainParams)
	parsedValue, isCorrect := big.NewInt(0).SetString(fmt.Sprintf("%x", result.Value), 16)
	r.True(t, isCorrect)
	r.Equal(t, big.NewInt(10), big.NewInt(parsedValue.Int64()))
	// Remove function test
	callParams.Args.Msg.Nonce++
	callParams.FuncSign = tests.StorageRemoveFuncSignature + tests.Uint256Number1
	_, _ = utils.CallContract(t, &callParams, &chainParams)
	// Test Get after Remove
	callParams.Args.Msg.Nonce++
	callParams.FuncSign = tests.StorageGetFuncSignature + tests.Uint256Number1
	result, _ = utils.CallContract(t, &callParams, &chainParams)
	parsedValue, isCorrect = big.NewInt(0).SetString(fmt.Sprintf("%x", result.Value), 16)
	r.True(t, isCorrect)
	r.Equal(t, big.NewInt(0), big.NewInt(parsedValue.Int64()))
}

func (suite *EvmContractSuite) TestLargeStorageContract() {
	t := suite.T()
	createParams := utils.CreateContractParams{ContractCode: tests.LargeContractCode}
	chainParams := suite.initChain()
	args := utils.CreateContract(t, &createParams, &chainParams)
	callParams := utils.CallContractParams{FuncSign: tests.LargeSetStorageFuncSignature, MsgValue: uint64(0), Args: args}
	// LargeSet function test
	_, _ = utils.CallContract(t, &callParams, &chainParams)
	// GetArray function test
	callParams.Args.Msg.Nonce++
	callParams.FuncSign = tests.LargeGetArrayFuncSignature
	result, _ := utils.CallContract(t, &callParams, &chainParams)
	for i := 0; i < tests.LargeGetArraySize; i++ {
		left := 32 * i
		right := left + 32
		parsedValue, isCorrect := big.NewInt(0).SetString(fmt.Sprintf("%x", result.Value[left:right]), 16)
		r.True(t, isCorrect)
		r.Equal(t, big.NewInt(int64(i)), big.NewInt(parsedValue.Int64()))
	}
	// GetString function test
	callParams.Args.Msg.Nonce++
	callParams.FuncSign = tests.LargeGetStringFuncSignature
	result, _ = utils.CallContract(t, &callParams, &chainParams)
	stringReturn := string(result.Value)
	r.Contains(t, stringReturn, tests.LargeGetStringReturn)
}

func (suite *EvmContractSuite) TestErrorContract() {
	t := suite.T()
	amount := 10000
	lessAmount := uint64(10)
	createParams := utils.CreateContractParams{ContractCode: tests.ErrorContractCode}
	chainParams := suite.initChain()
	args := utils.CreateContract(t, &createParams, &chainParams)
	callParams := utils.CallContractParams{FuncSign: tests.SuccessOrErrorFuncSignature, MsgValue: uint64(amount), Args: args}
	//Success function test
	_, _ = utils.CallContract(t, &callParams, &chainParams)
	// Check balance
	balance, err := suite.getBalance(callParams.Args.ToAddress, &chainParams)
	r.NoError(t, err)
	r.Equal(t, big.NewInt(int64(amount)), balance)
	// GetI function test
	callParams.Args.Msg.Nonce++
	callParams.FuncSign = tests.GetIFuncSignature
	callParams.MsgValue = 0
	result, _ := utils.CallContract(t, &callParams, &chainParams)
	parsedValue, isCorrect := big.NewInt(0).SetString(fmt.Sprintf("%x", result.Value), 16)
	r.True(t, isCorrect)
	r.Equal(t, big.NewInt(int64(amount)), big.NewInt(parsedValue.Int64()))
	// Error function test
	callParams.Args.Msg.Nonce++
	callParams.FuncSign = tests.SuccessOrErrorFuncSignature
	callParams.MsgValue = lessAmount
	_, err = utils.CallContract(t, &callParams, &chainParams)
	r.Error(t, err)
	// Check balance
	balance, err = suite.getBalance(callParams.Args.ToAddress, &chainParams)
	r.NoError(t, err)
	r.Equal(t, big.NewInt(int64(amount)), balance)
	// GetI function test
	callParams.Args.Msg.Nonce++
	callParams.FuncSign = tests.GetIFuncSignature
	callParams.MsgValue = 0
	result, _ = utils.CallContract(t, &callParams, &chainParams)
	parsedValue, isCorrect = big.NewInt(0).SetString(fmt.Sprintf("%x", result.Value), 16)
	r.True(t, isCorrect)
	r.Equal(t, big.NewInt(int64(amount)), big.NewInt(parsedValue.Int64()))
}

func (suite *EvmContractSuite) TestFactoryCreateContract() {
	t := suite.T()
	createParams := utils.CreateContractParams{ContractCode: tests.FactoryContractCode}
	chainParams := suite.initChain()
	args := utils.CreateContract(t, &createParams, &chainParams)
	callParams := utils.CallContractParams{FuncSign: tests.MakeNewFoo, MsgValue: uint64(0), Args: args}
	// MakeNewFoo function test
	result, _ := utils.CallContract(t, &callParams, &chainParams)
	newContractAddress, err := utils.ConvertAddress(common.BytesToAddress(common.TrimLeftZeroes(result.Value)), address.Actor)
	r.Nil(t, err)
	// Try to call contract created by opCreate
	callParams.Args.ToAddress = newContractAddress
	callParams.Args.Msg.To = newContractAddress
	callParams.Args.Msg.Nonce++
	callParams.FuncSign = tests.FooTest
	result, _ = utils.CallContract(t, &callParams, &chainParams)
	stringReturn := string(result.Value)
	r.Contains(t, stringReturn, tests.FooTestValue)
}

func (suite *EvmContractSuite) TestFactoryCreateFailContract() {
	t := suite.T()
	createParams := utils.CreateContractParams{ContractCode: tests.FactoryContractCodeFail}
	chainParams := suite.initChain()
	args := utils.CreateContract(t, &createParams, &chainParams)
	callParams := utils.CallContractParams{FuncSign: tests.MakeNewFoo, MsgValue: uint64(0), Args: args}
	// MakeNewFoo function test
	_, err := utils.CallContract(t, &callParams, &chainParams)
	r.NotNil(t, err)
}

// func (suite *EvmContractSuite) TestCallActorBool() {
// 	t := suite.T()
// 	createParams := utils.CreateContractParams{ContractCode: tests.CallActorContractCode}
// 	chainParams := suite.initChain()
// 	args := utils.CreateContract(t, &createParams, &chainParams)
// 	callParams := utils.CallContractParams{FuncSign: tests.CallActorMarshalledCallmeBool, MsgValue: uint64(0), Args: args}
// 	// Call contract
// 	result, _ := utils.CallContract(t, &callParams, &chainParams)
// 	stringReturn := fmt.Sprintf("%x", result.Value)
// 	fmt.Printf("%s", stringReturn)
// 	// Find index of marshalled structure
// 	index := strings.Index(stringReturn, "a46556616c756558")
// 	data, err := hex.DecodeString(stringReturn[index:])
// 	r.NoError(t, err)
// 	// Unmarshal contract return
// 	ret, err := utils.UnmarshalContractReturn(data)
// 	r.NoError(t, err)
// 	// Result should be true
// 	r.Equal(t, tests.Uint256Number1, fmt.Sprintf("%x", ret.Value))
// }

// func (suite *EvmContractSuite) TestCallActorString() {
// 	t := suite.T()
// 	createParams := utils.CreateContractParams{ContractCode: tests.CallActorContractCode}
// 	chainParams := suite.initChain()
// 	args := utils.CreateContract(t, &createParams, &chainParams)
// 	callParams := utils.CallContractParams{FuncSign: tests.CallActorMarshalledCallmeString, MsgValue: uint64(0), Args: args}
// 	// Call contract
// 	result, _ := utils.CallContract(t, &callParams, &chainParams)
// 	stringReturn := fmt.Sprintf("%x", result.Value)
// 	// Find index of marshalled structure
// 	fmt.Printf("%s", stringReturn)
// 	index := strings.Index(stringReturn, "a46556616c7565586")
// 	data, err := hex.DecodeString(stringReturn[index:])
// 	r.NoError(t, err)
// 	// Unmarshal contract return
// 	ret, err := utils.UnmarshalContractReturn(data)
// 	r.NoError(t, err)
// 	stringReturn = string(ret.Value)
// 	// Result should be "CALLME" string
// 	r.Contains(t, stringReturn, tests.CallActorReturnString)
// }

// func (suite *EvmContractSuite) TestCallActorFailBool() {
// 	t := suite.T()
// 	createParams := utils.CreateContractParams{ContractCode: tests.CallActorContractCodeFail}
// 	chainParams := suite.initChain()
// 	args := utils.CreateContract(t, &createParams, &chainParams)
// 	callParams := utils.CallContractParams{FuncSign: tests.CallActorMarshalledCallmeBool, MsgValue: uint64(0), Args: args}
// 	// Call contract
// 	_, err := utils.CallContract(t, &callParams, &chainParams)
// 	r.Error(t, err)
// 	callParams.FuncSign = tests.CallActorGetValue
// 	callParams.Args.Msg.Nonce++
// 	// Call contract
// 	result, err2 := utils.CallContract(t, &callParams, &chainParams)
// 	r.NoError(t, err2)
// 	// Unmarshal contract return
// 	stringReturn := fmt.Sprintf("%x", result.Value)
// 	// Result should be true
// 	r.Equal(t, tests.Uint256Number1, stringReturn)
// }

// func (suite *EvmContractSuite) TestCallActorFailString() {
// 	t := suite.T()
// 	createParams := utils.CreateContractParams{ContractCode: tests.CallActorContractCodeFail}
// 	chainParams := suite.initChain()
// 	args := utils.CreateContract(t, &createParams, &chainParams)
// 	callParams := utils.CallContractParams{FuncSign: tests.CallActorMarshalledCallmeString, MsgValue: uint64(0), Args: args}
// 	// Call contract
// 	_, err := utils.CallContract(t, &callParams, &chainParams)
// 	r.Error(t, err)
// 	callParams.FuncSign = tests.CallActorGetValue
// 	callParams.Args.Msg.Nonce++
// 	// Call contract
// 	result, err2 := utils.CallContract(t, &callParams, &chainParams)
// 	r.NoError(t, err2)
// 	// Unmarshal contract return
// 	stringReturn := fmt.Sprintf("%x", result.Value)
// 	// Result should be true
// 	r.Equal(t, tests.Uint256Number1, stringReturn)
// }

func TestEvmContract(t *testing.T) {
	err := contract.InitStateDBManager()
	if err != nil {
		panic(err)
	}
	suite.Run(t, newEvmContractSuite())
}
