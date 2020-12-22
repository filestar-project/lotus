package vm_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/lotus/chain/actors"
	"github.com/filecoin-project/lotus/chain/gen"
	"github.com/filecoin-project/lotus/chain/stmgr"
	"github.com/filecoin-project/lotus/chain/types"
	"github.com/filecoin-project/lotus/chain/vm"
	"github.com/filecoin-project/lotus/chain/wallet"
	"github.com/filestar-project/evm-adapter/tests"
	"github.com/stretchr/testify/require"

	verifreg0 "github.com/filecoin-project/specs-actors/actors/builtin/verifreg"
	"github.com/filecoin-project/specs-actors/v2/actors/builtin"
	account2 "github.com/filecoin-project/specs-actors/v2/actors/builtin/account"
	logging "github.com/ipfs/go-log"
)

func TestCreateContract(t *testing.T) {
	logging.SetAllLoggers(logging.LevelInfo)

	verifreg0.MinVerifiedDealSize = big.Zero()
	ctx := context.TODO()

	cg, err := gen.NewGenerator()
	if err != nil {
		t.Fatal(err)
	}

	sm := stmgr.NewStateManager(cg.ChainStore())

	inv := vm.NewActorRegistry()
	inv.Register(nil, account2.Actor{})

	sm.SetVMConstructor(func(ctx context.Context, vmopt *vm.VMOpts) (*vm.VM, error) {
		nvm, err := vm.NewVM(ctx, vmopt)
		if err != nil {
			return nil, err
		}
		nvm.SetInvoker(inv)
		return nvm, nil
	})

	cg.SetStateManager(sm)

	enc, err := actors.SerializeParams(&account2.ContractParams{Code: []byte(tests.HelloWorldContractCode)})
	if err != nil {
		t.Fatal(err)
	}

	key, err := wallet.GenerateKey(types.KTSecp256k1)
	if err != nil {
		t.Fatal(err)
	}
	toAddress := key.Address

	m := &types.Message{
		From:       cg.Banker(),
		To:         toAddress,
		Method:     builtin.MethodsAccount.CreateContract,
		Params:     enc,
		GasLimit:   types.TestGasLimit,
		Value:      types.NewInt(0),
		GasPremium: types.NewInt(0),
		GasFeeCap:  types.NewInt(0),
	}

	for i := 0; i < 5; i++ {
		_, err := cg.NextTipSet()
		if err != nil {
			t.Fatal(err)
		}
	}

	fn := func(checker func(t *testing.T, e error)) {
		ts, err := cg.NextTipSet()
		if err != nil {
			t.Fatal(err)
		}

		ret, err := sm.CallWithGas(ctx, m, nil, ts.TipSet.TipSet())
		checker(t, err)

		var result account2.ContractResult
		err = result.UnmarshalCBOR(bytes.NewReader(ret.MsgRct.Return))
		checker(t, err)
	}

	fn(func(t *testing.T, e error) { require.NoError(t, e) })
	fn(func(t *testing.T, e error) { require.Error(t, e) })
}
