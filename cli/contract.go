package cli

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-jsonrpc"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/lotus/api"
	"github.com/filecoin-project/lotus/chain/actors"
	types "github.com/filecoin-project/lotus/chain/types"
	"github.com/filecoin-project/specs-actors/v2/actors/builtin"
	"github.com/filecoin-project/specs-actors/v2/actors/builtin/contract"
	init_ "github.com/filecoin-project/specs-actors/v2/actors/builtin/init"
	cid "github.com/ipfs/go-cid"
	"github.com/urfave/cli/v2"
	"golang.org/x/xerrors"
)

var contractCmd = &cli.Command{
	Name:  "contract",
	Usage: "Manage smart contracts",
	Subcommands: []*cli.Command{
		contractCreate,
		contractCall,
	},
}

type contractCmdParams struct {
	api        api.FullNode
	closer     jsonrpc.ClientCloser
	ctx        context.Context
	amount     types.BigInt
	from       address.Address
	to         address.Address
	code       []byte
	gasPremium types.BigInt
	gasFeeCap  types.BigInt
	gasLimit   int64
	nonce      uint64
	salt       []byte
}

const (
	contractCreateCommand = iota
	contractCallCommand
)

func newContractCmdParams(cctx *cli.Context, cmd int) (*contractCmdParams, error) {
	p := &contractCmdParams{}

	api, closer, err := GetFullNodeAPI(cctx)
	if err != nil {
		return nil, err
	}

	ctx := ReqContext(cctx)

	var fromAddr address.Address
	if from := cctx.String("from"); from == "" {
		defaddr, err := api.WalletDefaultAddress(ctx)
		if err != nil {
			return nil, err
		}

		fromAddr = defaddr
	} else {
		addr, err := address.NewFromString(from)
		if err != nil {
			return nil, err
		}

		fromAddr = addr
	}

	if fromAddr.Protocol() != address.SECP256K1 {
		return nil, fmt.Errorf("only secp256k1 addresses allowed with contracts")
	}

	gp, err := types.BigFromString(cctx.String("gas-premium"))
	if err != nil {
		return nil, err
	}

	gfc, err := types.BigFromString(cctx.String("gas-feecap"))
	if err != nil {
		return nil, err
	}

	amount, err := types.ParseFIL(cctx.Args().Get(0))
	if err != nil {
		return nil, ShowHelp(cctx, fmt.Errorf("failed to parse amount: %w", err))
	}

	indexAddr := -1
	indexCode := 1
	if cmd == contractCallCommand {
		indexAddr = 1
		indexCode = 2
	}

	code, err := hex.DecodeString(cctx.Args().Get(indexCode))
	if err != nil {
		return nil, fmt.Errorf("failed to decode contract code as hex param: %w", err)
	}

	var toAddr address.Address
	if indexAddr != -1 {
		a, err := address.NewFromString(cctx.Args().Get(indexAddr))
		if a == address.Undef {
			if err != nil {
				return nil, err
			}
			return nil, ShowHelp(cctx, fmt.Errorf("contract address must be specified"))
		}
		toAddr = a
	}

	p.api = api
	p.closer = closer
	p.ctx = ctx
	p.amount = types.BigInt(amount)
	p.from = fromAddr
	p.to = toAddr
	p.code = code
	p.gasPremium = gp
	p.gasFeeCap = gfc
	p.gasLimit = cctx.Int64("gas-limit")
	p.nonce = 0

	nonce := cctx.Int64("nonce")
	if nonce > 0 {
		p.nonce = uint64(nonce)
	}

	return p, nil
}

var contractDefaultFlags = []cli.Flag{
	&cli.StringFlag{
		Name:  "from",
		Usage: "optionally specify the account to send funds from (only secp256k1 allowed)",
	},
	&cli.StringFlag{
		Name:  "gas-premium",
		Usage: "specify gas price to use in AttoFIL",
		Value: "0",
	},
	&cli.StringFlag{
		Name:  "gas-feecap",
		Usage: "specify gas fee cap to use in AttoFIL",
		Value: "0",
	},
	&cli.Int64Flag{
		Name:  "gas-limit",
		Usage: "specify gas limit",
		Value: 0,
	},
	&cli.Int64Flag{
		Name:  "nonce",
		Usage: "specify the nonce to use",
		Value: -1,
	},
}

type contractCreateInfo struct {
	Cid string
}

var contractCreate = &cli.Command{
	Name:      "create",
	Usage:     "create smart contract",
	ArgsUsage: "[amount] [contract-code]",
	Flags:     contractDefaultFlags,
	Action: func(cctx *cli.Context) error {

		if cctx.Args().Len() != 2 {
			return ShowHelp(cctx, fmt.Errorf("'create' expects three arguments: amount, address and contract code"))
		}

		p, err := newContractCmdParams(cctx, contractCreateCommand)
		p.to = builtin.InitActorAddr
		if err != nil {
			return err
		}
		defer p.closer()

		contractParams, err := actors.SerializeParams(&contract.ContractParams{Code: p.code, Value: p.amount, CommitStatus: true})
		if err != nil {
			return xerrors.Errorf("failed to serialize contract create params: %w", err)
		}

		params, err := actors.SerializeParams(&init_.ExecParams{CodeCID: builtin.ContractActorCodeID, ConstructorParams: contractParams})
		if err != nil {
			return xerrors.Errorf("failed to serialize exec contract create params: %w", err)
		}

		msg := &types.Message{
			From:       p.from,
			To:         p.to,
			Value:      big.NewInt(0),
			GasPremium: p.gasPremium,
			GasFeeCap:  p.gasFeeCap,
			GasLimit:   p.gasLimit,
			Method:     builtin.MethodsInit.Exec,
			Params:     params,
			Nonce:      0,
		}

		var cid cid.Cid
		if p.nonce > 0 {
			msg.Nonce = p.nonce
			sm, err := p.api.WalletSignMessage(p.ctx, p.from, msg)
			if err != nil {
				return err
			}

			_, err = p.api.MpoolPush(p.ctx, sm)
			if err != nil {
				return err
			}
			cid = sm.Cid()
		} else {
			sm, err := p.api.MpoolPushMessage(p.ctx, msg, nil)
			if err != nil {
				return err
			}
			cid = sm.Cid()
		}

		createInfo := &contractCreateInfo{Cid: cid.String()}
		j, err := json.MarshalIndent(createInfo, "", "    ")
		if err != nil {
			return err
		}

		fmt.Println(string(j))

		return nil
	},
}

type contractCallInfo struct {
	Cid string
}

var contractCall = &cli.Command{
	Name:      "call",
	Usage:     "Call smart contract method",
	ArgsUsage: "[amount] [address] [call-code]",
	Flags:     contractDefaultFlags,
	Action: func(cctx *cli.Context) error {
		if cctx.Args().Len() != 3 {
			return ShowHelp(cctx, fmt.Errorf("'call' expects three arguments: amount, address and contract code"))
		}

		p, err := newContractCmdParams(cctx, contractCallCommand)
		if err != nil {
			return err
		}
		defer p.closer()

		params, err := actors.SerializeParams(&contract.ContractParams{Code: p.code, Value: p.amount, CommitStatus: true})
		if err != nil {
			return xerrors.Errorf("failed to serialize contract call params: %w", err)
		}

		msg := &types.Message{
			From:       p.from,
			To:         p.to,
			Value:      big.NewInt(0),
			GasPremium: p.gasPremium,
			GasFeeCap:  p.gasFeeCap,
			GasLimit:   p.gasLimit,
			Method:     builtin.MethodsContract.CallContract,
			Params:     params,
			Nonce:      0,
		}

		var cid cid.Cid
		if p.nonce > 0 {
			msg.Nonce = p.nonce
			sm, err := p.api.WalletSignMessage(p.ctx, p.from, msg)
			if err != nil {
				return err
			}

			_, err = p.api.MpoolPush(p.ctx, sm)
			if err != nil {
				return err
			}
			cid = sm.Cid()
		} else {
			sm, err := p.api.MpoolPushMessage(p.ctx, msg, nil)
			if err != nil {
				return err
			}
			cid = sm.Cid()
		}

		callInfo := &contractCallInfo{Cid: cid.String()}
		j, err := json.MarshalIndent(callInfo, "", "    ")
		if err != nil {
			return err
		}

		fmt.Println(string(j))

		return nil
	},
}
