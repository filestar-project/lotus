package vm_test

import (
	"context"
	"encoding/hex"
	"testing"

	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/lotus/chain/actors"
	"github.com/filecoin-project/lotus/chain/gen"
	"github.com/filecoin-project/lotus/chain/types"
	"github.com/filecoin-project/lotus/chain/wallet"
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

func TestCreateContract2(t *testing.T) {
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

	enc, err := actors.SerializeParams(&account2.ContractParams{Code: code})
	if err != nil {
		t.Fatal(err)
	}

	key, err := wallet.GenerateKey(types.KTSecp256k1)
	if err != nil {
		t.Fatal(err)
	}
	toAddress := key.Address

	sm := cg.StateManager()
	act, err := sm.LoadActor(ctx, cg.Banker(), ts.TipSet.TipSet())
	if err != nil {
		t.Fatal(err)
	}

	msg := &types.Message{
		From:       cg.Banker(),
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
	ret, root, err = d.ExecuteMessage(cg.ChainStore().Blockstore(), c.ExecuteMessageParams{
		Preroot:    root,
		Epoch:      baseEpoch + 1,
		Message:    msg,
		BaseFee:    c.DefaultBaseFee,
		CircSupply: c.DefaultCirculatingSupply,
	})

	log.Infof("return:\n %+v\n%+v\n\n", err, ret)

	r.Error(t, err)
	r.Nil(t, ret)
}
