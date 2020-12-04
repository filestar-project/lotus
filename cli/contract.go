package cli

import (
	"github.com/urfave/cli/v2"
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
}

func newContractCmdParams(cctx *cli.Context) (*contractCmdParams, error) {
	p := &contractCmdParams{}
	return p, nil
}

var contractCreate = &cli.Command{
	Name:      "create",
	Usage:     "create smart contract",
	ArgsUsage: "[amount] [contract-code]",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "from",
			Usage: "optionally specify the account to send funds from",
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
	},
	Action: func(cctx *cli.Context) error {
		/*
			api, closer, err := GetFullNodeAPI(cctx)
			if err != nil {
				return err
			}
			defer closer()
			ctx := ReqContext(cctx)
		*/

		return nil
	},
}

var contractCall = &cli.Command{
	Name:      "call",
	Usage:     "Call smart contract method",
	ArgsUsage: "[address] [amount] [call-code]",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "from",
			Usage: "optionally specify the account to send funds from",
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
	},
	Action: func(cctx *cli.Context) error {
		/*
			api, closer, err := GetFullNodeAPI(cctx)
			if err != nil {
				return err
			}
			defer closer()
			ctx := ReqContext(cctx)
		*/

		return nil
	},
}
