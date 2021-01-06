package vm_test

import (
	"os"
	"bytes"
	"context"
	"encoding/hex"
	"testing"

	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/lotus/chain/actors"
	"github.com/filecoin-project/lotus/chain/gen"
	"github.com/filecoin-project/lotus/chain/types"
	"github.com/filecoin-project/lotus/build"
	"github.com/filecoin-project/lotus/chain/actors/policy"
	c "github.com/filecoin-project/lotus/conformance"
	"github.com/filecoin-project/test-vectors/schema"
	"github.com/filestar-project/evm-adapter/tests"
	r "github.com/stretchr/testify/require"

	verifreg0 "github.com/filecoin-project/specs-actors/actors/builtin/verifreg"
	"github.com/filecoin-project/specs-actors/v2/actors/builtin"
	account2 "github.com/filecoin-project/specs-actors/v2/actors/builtin/account"
	logging "github.com/ipfs/go-log"
)

var log = logging.Logger("test")

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

func TestHelloWorldContract(t *testing.T) {
	logging.SetAllLoggers(logging.LevelDebug)

	verifreg0.MinVerifiedDealSize = big.Zero()

	var (
		ctx       = context.Background()
		baseEpoch = abi.ChainEpoch(50)
	)

	cg, err := gen.NewGenerator()
	if err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 5; i++ {
		_, err := cg.NextTipSet()
		if err != nil {
			t.Fatal(err)
		}
	}

	ts, err := cg.NextTipSet()

	if err != nil {
		t.Fatal(err)
	}

	root := ts.TipSet.TipSet().ParentState()

	code, err := hex.DecodeString(tests.HelloWorldContractCode)
	r.NoError(t, err)

	fromAddress := cg.Banker()
	sm := cg.StateManager()
	act, err := sm.LoadActor(ctx, fromAddress, ts.TipSet.TipSet())
	if err != nil {
		t.Fatal(err)
	}

	toAddress, salt, err := account2.PrecomputeContractAddress(fromAddress, code)
	if err != nil {
		t.Fatal(err)
	}

	enc, err := actors.SerializeParams(&account2.ContractParams{Code: code, Salt: salt})
	if err != nil {
		t.Fatal(err)
	}

	msg := &types.Message{
		From:       fromAddress,
		To:         toAddress,
		Method:     builtin.MethodsAccount.CreateContract,
		Params:     enc,
		GasLimit:   types.TestGasLimit,
		Value:      types.NewInt(0),
		GasPremium: types.NewInt(0),
		GasFeeCap:  types.NewInt(0),
		Nonce:      act.Nonce,
	}

	d := c.NewDriver(ctx, schema.Selector{}, c.DriverOpts{})

	ret, root, err := d.ExecuteMessage(cg.ChainStore().Blockstore(), c.ExecuteMessageParams{
		Preroot:    root,
		Epoch:      baseEpoch,
		Message:    msg,
		BaseFee:    c.DefaultBaseFee,
		CircSupply: c.DefaultCirculatingSupply,
	})

	log.Infof("return: %+v\n\n", ret)

	r.NoError(t, err)
	r.NotNil(t, ret)

	msg.Nonce++

	// try to create the same contract one more time
	ret, _, err = d.ExecuteMessage(cg.ChainStore().Blockstore(), c.ExecuteMessageParams{
		Preroot:    root,
		Epoch:      baseEpoch + 1,
		Message:    msg,
		BaseFee:    c.DefaultBaseFee,
		CircSupply: c.DefaultCirculatingSupply,
	})

	log.Infof("return:\n %+v\n%+v\n\n", err, ret)

	r.Error(t, err)
	r.Nil(t, ret)

	// try to call contract
	msg.Method = builtin.MethodsAccount.CallContract

	funcSig, err := hex.DecodeString(tests.HelloWorldFuncSignature)
	if err != nil {
		t.Fatal(err)
	}

	enc, err = actors.SerializeParams(&account2.ContractParams{Code: funcSig})
	if err != nil {
		t.Fatal(err)
	}
	msg.Params = enc

	ret, root, err = d.ExecuteMessage(cg.ChainStore().Blockstore(), c.ExecuteMessageParams{
		Preroot:    root,
		Epoch:      baseEpoch + 1,
		Message:    msg,
		BaseFee:    c.DefaultBaseFee,
		CircSupply: c.DefaultCirculatingSupply,
	})

	log.Infof("return: %+v\n\n", ret)

	r.NoError(t, err)
	r.NotNil(t, ret)

	var result account2.ContractResult
	err = result.UnmarshalCBOR(bytes.NewReader(ret.MessageReceipt.Return))
	r.NoError(t, err)

	stringReturn := string(result.Value)
	r.Contains(t, stringReturn, tests.HelloWorldFuncReturn)

	log.Infof("return string: %v\n", stringReturn)
	log.Infof("return value: %x\n", result.Value)
}
