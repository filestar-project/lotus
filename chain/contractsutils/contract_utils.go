package contractsutils

import (
	"bytes"
	"context"
	"encoding/hex"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/lotus/api"
	"github.com/filecoin-project/lotus/chain/actors"
	"github.com/filecoin-project/lotus/chain/actors/aerrors"
	"github.com/filecoin-project/lotus/chain/gen"
	"github.com/filecoin-project/lotus/chain/store"
	"github.com/filecoin-project/lotus/chain/types"
	c "github.com/filecoin-project/lotus/conformance"
	"github.com/filecoin-project/lotus/node/impl"
	"github.com/filecoin-project/specs-actors/v2/actors/builtin"
	contract "github.com/filecoin-project/specs-actors/v2/actors/builtin/contract"
	init0 "github.com/filecoin-project/specs-actors/v2/actors/builtin/init"
	"github.com/ipfs/go-cid"
	r "github.com/stretchr/testify/require"
)

type ChainParams struct {
	Root      cid.Cid
	BaseEpoch abi.ChainEpoch
	Cg        *gen.ChainGen
	Drive     *c.Driver
	Ctx       context.Context
	Tipset    *store.FullTipSet
}

type CreateContractParams struct {
	ContractCode string
}

type CallArguments struct {
	Msg         *types.Message
	ToAddress   address.Address
	FromAddress address.Address
}
type CallContractParams struct {
	Args     CallArguments
	FuncSign string
	MsgValue uint64
}

func UnmarshalContractReturn(data []byte) (contract.ContractResult, error) {
	var ret contract.ContractResult
	err := ret.UnmarshalCBOR(bytes.NewReader(data))
	return ret, err
}

func ConvertAddress(addr common.Address, protocol byte) (address.Address, error) {
	addrWithPrefix := append([]byte{protocol}, addr.Bytes()...)
	newAddress, err := address.NewFromBytes(addrWithPrefix)
	if err != nil {
		return address.Address{}, err
	}
	return newAddress, nil
}

func CreateContract(t *testing.T, params *CreateContractParams, chainParams *ChainParams) CallArguments {
	//Init params to create contract
	code, err := hex.DecodeString(params.ContractCode)
	r.NoError(t, err)
	//Load addresses for tests
	fromAddress := chainParams.Cg.Banker()
	sm := chainParams.Cg.StateManager()
	act, err := sm.LoadActor(chainParams.Ctx, fromAddress, chainParams.Tipset.TipSet())
	r.NoError(t, err)
	contractEnc, err := actors.SerializeParams(&contract.ContractParams{Code: code, Value: big.NewInt(0), Commit: true})
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
	ret, root, err := chainParams.Drive.ExecuteMessage(chainParams.Cg.ChainStore().Blockstore(), c.ExecuteMessageParams{
		Preroot:    chainParams.Root,
		Epoch:      chainParams.BaseEpoch,
		Message:    msg,
		BaseFee:    c.DefaultBaseFee,
		CircSupply: c.DefaultCirculatingSupply,
	})
	msg.Nonce++
	chainParams.Root = root
	r.NoError(t, err)
	r.NotNil(t, ret)
	var result init0.ExecReturn
	if ret.ActorErr == nil {
		err = result.UnmarshalCBOR(bytes.NewReader(ret.MessageReceipt.Return))
		r.NoError(t, err)
	}
	msg.To = result.RobustAddress
	err = contract.SaveContractInfo()
	r.NoError(t, err)
	return CallArguments{Msg: msg, ToAddress: msg.To, FromAddress: fromAddress}
}

func CallContract(t *testing.T, callParams *CallContractParams, chainParams *ChainParams) (*contract.ContractResult, aerrors.ActorError) {
	// Try to call contract
	callParams.Args.Msg.Method = builtin.MethodsContract.CallContract
	//Build function signature
	funcSig, err := hex.DecodeString(callParams.FuncSign)
	r.NoError(t, err)
	//Add contract params
	enc, err := actors.SerializeParams(&contract.ContractParams{Code: funcSig, Value: types.NewInt(callParams.MsgValue), Commit: true})
	r.NoError(t, err)
	callParams.Args.Msg.Params = enc
	//Execute call contract with funcSig
	ret, rt, err := chainParams.Drive.ExecuteMessage(chainParams.Cg.ChainStore().Blockstore(), c.ExecuteMessageParams{
		Preroot:    chainParams.Root,
		Epoch:      chainParams.BaseEpoch + 1,
		Message:    callParams.Args.Msg,
		BaseFee:    c.DefaultBaseFee,
		CircSupply: c.DefaultCirculatingSupply,
	})
	chainParams.Root = rt

	r.NoError(t, err)
	r.NotNil(t, ret)

	//Get contract result
	if ret.ActorErr == nil {
		var result contract.ContractResult
		err = result.UnmarshalCBOR(bytes.NewReader(ret.MessageReceipt.Return))
		r.NoError(t, err)
		err = contract.SaveContractInfo()
		r.NoError(t, err)
		return &result, nil
	}
	return nil, ret.ActorErr
}

func CreateContractOnChain(ctx context.Context, t *testing.T, params *CreateContractParams, full api.FullNode) (CallArguments, *types.SignedMessage) {
	// //Init params to create contract
	code, err := hex.DecodeString(params.ContractCode)
	r.NoError(t, err)
	// //Load addresses for tests
	fromAddress, err := full.WalletDefaultAddress(ctx)
	r.NoError(t, err)
	sm := full.(*impl.FullNodeAPI).StateAPI.StateManager
	tipset, err := full.ChainHead(ctx)
	r.NoError(t, err)
	act, err := sm.LoadActor(ctx, fromAddress, tipset)
	r.NoError(t, err)
	contractEnc, err := actors.SerializeParams(&contract.ContractParams{Code: code, Value: big.NewInt(0), Commit: true})
	r.NoError(t, err)
	enc, err := actors.SerializeParams(&init0.ExecParams{CodeCID: builtin.ContractActorCodeID, ConstructorParams: contractEnc})
	r.NoError(t, err)
	// //Build message to create contract
	msg := &types.Message{
		From:       fromAddress,
		To:         builtin.InitActorAddr,
		Method:     builtin.MethodsInit.Exec,
		Params:     enc,
		GasLimit:   types.TestGasLimit,
		Value:      types.NewInt(0),
		GasPremium: types.NewInt(100000000),
		GasFeeCap:  types.NewInt(100000000),
		Nonce:      act.Nonce + 20,
	}
	//Execute create contract
	signedMessage, err := full.WalletSignMessage(ctx, fromAddress, msg)
	r.NoError(t, err)
	// _, err = full.MpoolPush(chainParams.Ctx, signedMessage)
	// r.NoError(t, err)
	return CallArguments{Msg: msg, FromAddress: fromAddress}, signedMessage
}

func CallContractOnChain(ctx context.Context, t *testing.T, callParams *CallContractParams, full api.FullNode) *types.SignedMessage {
	// Try to call contract
	callParams.Args.Msg.Method = builtin.MethodsContract.CallContract
	//Build function signature
	funcSig, err := hex.DecodeString(callParams.FuncSign)
	r.NoError(t, err)
	//Add contract params
	enc, err := actors.SerializeParams(&contract.ContractParams{Code: funcSig, Value: types.NewInt(callParams.MsgValue), Commit: true})
	r.NoError(t, err)
	callParams.Args.Msg.Params = enc
	//Execute call contract with funcSig
	signedMessage, err := full.WalletSignMessage(ctx, callParams.Args.FromAddress, callParams.Args.Msg)
	r.NoError(t, err)
	_, err = full.MpoolPush(ctx, signedMessage)
	r.NoError(t, err)
	return signedMessage
}
