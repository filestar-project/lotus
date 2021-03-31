package cli

import (
	"fmt"
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/lotus/chain/actors"
	"github.com/filecoin-project/lotus/chain/actors/builtin/stake"
	"github.com/filecoin-project/lotus/chain/types"
	builtin2 "github.com/filecoin-project/specs-actors/v2/actors/builtin"
	stake2 "github.com/filecoin-project/specs-actors/v2/actors/builtin/stake"
	"github.com/urfave/cli/v2"
)

var stakeCmd = &cli.Command{
	Name:  "stake",
	Usage: "Interact with filestar stake",
	Subcommands: []*cli.Command{
		stakeInfoCmd,
		stakeListLockedPrincipalCmd,
		stakeListVestingCmd,
		stakeDepositCmd,
		stakeWithdrawCmd,
	},
}

var stakeInfoCmd = &cli.Command{
	Name:      "info",
	Usage:     "Print stake information",
	ArgsUsage: "[<stakerAddress> (optional)]",
	Action: func(cctx *cli.Context) error {
		api, closer, err := GetFullNodeAPI(cctx)
		if err != nil {
			return err
		}
		defer closer()

		ctx := ReqContext(cctx)

		si, err := api.StateStakeInfo(ctx, types.EmptyTSK)
		if err != nil {
			return err
		}
		fmt.Printf("TotalStakePower: %s\n", types.FIL(si.TotalStakePower))
		fmt.Printf("MaturePeriod: %s\n", si.MaturePeriod)
		fmt.Printf("RoundPeriod: %s\n", si.RoundPeriod)
		fmt.Printf("PrincipalLockDuration: %s\n", si.PrincipalLockDuration)
		fmt.Printf("MinDepositAmount: %s\n", types.FIL(si.MinDepositAmount))
		fmt.Printf("MaxRewardPerRound: %s\n", types.FIL(si.MaxRewardPerRound))
		fmt.Printf("InflationFactor: %d\n", si.InflationFactor)
		fmt.Printf("LastRoundReward: %s\n", types.FIL(si.LastRoundReward))
		fmt.Printf("NextRoundEpoch: %s\n", si.NextRoundEpoch)

		var stakerAddr address.Address
		if cctx.Args().Present() {
			stakerAddr, err = address.NewFromString(cctx.Args().First())
			if err != nil {
				return err
			}
		} else {
			stakerAddr, err = api.WalletDefaultAddress(ctx)
			if err != nil {
				return err
			}
		}
		stakerID, err := api.StateLookupID(ctx, stakerAddr, types.EmptyTSK)
		if err != nil {
			return err
		}
		fmt.Printf("\nStaker: %s\n", stakerID)

		stakePower, err := api.StateStakerPower(ctx, stakerID, types.EmptyTSK)
		if err != nil {
			return err
		}
		if si.TotalStakePower.GreaterThan(big.Zero()) {
			fmt.Printf("Stake Power: %s (%0.4f%%)\n", types.FIL(stakePower), float64(types.BigDiv(types.BigMul(stakePower, types.NewInt(1000000)), si.TotalStakePower).Int64())/10000)
		} else {
			fmt.Printf("Stake Power: %s\n", types.FIL(stakePower))
		}

		lockedPrincipal, err := api.StateStakerLockedPrincipal(ctx, stakerID, types.EmptyTSK)
		if err != nil {
			return err
		}
		fmt.Printf("Locked Principal: %s\n", types.FIL(lockedPrincipal))

		availablePrincipal, err := api.StateStakerAvailablePrincipal(ctx, stakerID, types.EmptyTSK)
		if err != nil {
			return err
		}
		fmt.Printf("Available Principal: %s\n", types.FIL(availablePrincipal))

		vestingReward, err := api.StateStakerVestingReward(ctx, stakerID, types.EmptyTSK)
		if err != nil {
			return err
		}
		fmt.Printf("Vesting Reward: %s\n", types.FIL(vestingReward))

		availableReward, err := api.StateStakerAvailableReward(ctx, stakerID, types.EmptyTSK)
		if err != nil {
			return err
		}
		fmt.Printf("Available Reward: %s\n", types.FIL(availableReward))

		return nil
	},
}

var stakeListLockedPrincipalCmd = &cli.Command{
	Name:      "list-locked-principal",
	Usage:     "Print locked principals",
	ArgsUsage: "[<stakerAddress> (optional)]",
	Action: func(cctx *cli.Context) error {
		api, closer, err := GetFullNodeAPI(cctx)
		if err != nil {
			return err
		}
		defer closer()

		ctx := ReqContext(cctx)

		var stakerAddr address.Address
		if cctx.Args().Present() {
			stakerAddr, err = address.NewFromString(cctx.Args().First())
			if err != nil {
				return err
			}
		} else {
			stakerAddr, err = api.WalletDefaultAddress(ctx)
			if err != nil {
				return err
			}
		}
		stakerID, err := api.StateLookupID(ctx, stakerAddr, types.EmptyTSK)
		if err != nil {
			return err
		}
		fmt.Printf("Staker: %s\n", stakerID)

		list, err := api.StateStakerLockedPrincipalList(ctx, stakerID, types.EmptyTSK)
		if err != nil {
			return err
		}
		for _, item := range list {
			fmt.Printf("Epoch: %s\tAmount: %s\n", item.Epoch, types.FIL(item.Amount))
		}
		return nil
	},
}

var stakeListVestingCmd = &cli.Command{
	Name:      "list-vesting",
	Usage:     "Print vesting rewards",
	ArgsUsage: "[<stakerAddress> (optional)]",
	Action: func(cctx *cli.Context) error {
		api, closer, err := GetFullNodeAPI(cctx)
		if err != nil {
			return err
		}
		defer closer()

		ctx := ReqContext(cctx)

		var stakerAddr address.Address
		if cctx.Args().Present() {
			stakerAddr, err = address.NewFromString(cctx.Args().First())
			if err != nil {
				return err
			}
		} else {
			stakerAddr, err = api.WalletDefaultAddress(ctx)
			if err != nil {
				return err
			}
		}
		stakerID, err := api.StateLookupID(ctx, stakerAddr, types.EmptyTSK)
		if err != nil {
			return err
		}
		fmt.Printf("Staker: %s\n", stakerID)

		list, err := api.StateStakerVestingRewardList(ctx, stakerID, types.EmptyTSK)
		if err != nil {
			return err
		}
		for _, item := range list {
			fmt.Printf("Epoch: %s\tAmount: %s\n", item.Epoch, types.FIL(item.Amount))
		}
		return nil
	},
}

var stakeDepositCmd = &cli.Command{
	Name:      "deposit",
	Usage:     "Deposit stake principal",
	ArgsUsage: "[amount]",
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
		if cctx.Args().Len() != 1 {
			return ShowHelp(cctx, fmt.Errorf("'stake deposit' expects one argument: amount"))
		}

		api, closer, err := GetFullNodeAPI(cctx)
		if err != nil {
			return err
		}
		defer closer()

		ctx := ReqContext(cctx)

		val, err := types.ParseFIL(cctx.Args().Get(0))
		if err != nil {
			return ShowHelp(cctx, fmt.Errorf("failed to parse amount: %w", err))
		}

		var fromAddr address.Address
		if from := cctx.String("from"); from == "" {
			defaddr, err := api.WalletDefaultAddress(ctx)
			if err != nil {
				return err
			}

			fromAddr = defaddr
		} else {
			addr, err := address.NewFromString(from)
			if err != nil {
				return err
			}

			fromAddr = addr
		}

		gp, err := types.BigFromString(cctx.String("gas-premium"))
		if err != nil {
			return err
		}
		gfc, err := types.BigFromString(cctx.String("gas-feecap"))
		if err != nil {
			return err
		}

		msg := &types.Message{
			From:       fromAddr,
			To:         stake.Address,
			Value:      types.BigInt(val),
			GasPremium: gp,
			GasFeeCap:  gfc,
			GasLimit:   cctx.Int64("gas-limit"),
			Method:     builtin2.MethodsStake.Deposit,
		}

		if cctx.Int64("nonce") > 0 {
			msg.Nonce = uint64(cctx.Int64("nonce"))
			sm, err := api.WalletSignMessage(ctx, fromAddr, msg)
			if err != nil {
				return err
			}

			_, err = api.MpoolPush(ctx, sm)
			if err != nil {
				return err
			}
			fmt.Println(sm.Cid())
		} else {
			sm, err := api.MpoolPushMessage(ctx, msg, nil)
			if err != nil {
				return err
			}
			fmt.Println(sm.Cid())
		}

		return nil
	},
}

var stakeWithdrawCmd = &cli.Command{
	Name:      "withdraw",
	Usage:     "Withdraw stake principal or reward",
	ArgsUsage: "[source amount(STAR) optional, otherwise will withdraw max available]",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "staker",
			Usage: "optionally specify the staker to withdraw funds from",
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
		if cctx.Args().Len() < 1 {
			return ShowHelp(cctx, fmt.Errorf("'stake withdraw' expects at least one argument: source"))
		}

		api, closer, err := GetFullNodeAPI(cctx)
		if err != nil {
			return err
		}
		defer closer()

		ctx := ReqContext(cctx)

		var stakerAddr address.Address
		if from := cctx.String("staker"); from == "" {
			defaddr, err := api.WalletDefaultAddress(ctx)
			if err != nil {
				return err
			}

			stakerAddr = defaddr
		} else {
			addr, err := address.NewFromString(from)
			if err != nil {
				return err
			}

			stakerAddr = addr
		}
		stakerID, err := api.StateLookupID(ctx, stakerAddr, types.EmptyTSK)
		if err != nil {
			return err
		}

		var method abi.MethodNum
		var availableAmount abi.TokenAmount
		source := cctx.Args().First()
		switch source {
		case "principal":
			method = builtin2.MethodsStake.WithdrawPrincipal
			availableAmount, err = api.StateStakerAvailablePrincipal(ctx, stakerID, types.EmptyTSK)
			if err != nil {
				return err
			}
		case "reward":
			method = builtin2.MethodsStake.WithdrawReward
			availableAmount, err = api.StateStakerAvailableReward(ctx, stakerID, types.EmptyTSK)
			if err != nil {
				return err
			}
		default:
			return ShowHelp(cctx, fmt.Errorf("source must be one of: principal reward"))
		}

		var reqAmount abi.TokenAmount
		if cctx.Args().Len() == 2 {
			val, err := types.ParseFIL(cctx.Args().Get(1))
			if err != nil {
				return ShowHelp(cctx, fmt.Errorf("failed to parse amount: %w", err))
			}
			reqAmount = abi.TokenAmount(val)
			if reqAmount.GreaterThan(availableAmount) {
				return ShowHelp(cctx, fmt.Errorf("can't withdraw more funds than available; requested: %s; available: %s", types.FIL(reqAmount), types.FIL(availableAmount)))
			}
		} else {
			reqAmount = availableAmount
		}

		params, err := actors.SerializeParams(&stake2.WithdrawParams{
			AmountRequested: reqAmount, // Default to attempting to withdraw all the extra funds in the miner actor
		})
		if err != nil {
			return ShowHelp(cctx, fmt.Errorf("failed to serialize params: %w", err))
		}

		gp, err := types.BigFromString(cctx.String("gas-premium"))
		if err != nil {
			return err
		}
		gfc, err := types.BigFromString(cctx.String("gas-feecap"))
		if err != nil {
			return err
		}

		msg := &types.Message{
			From:       stakerAddr,
			To:         stake.Address,
			GasPremium: gp,
			GasFeeCap:  gfc,
			GasLimit:   cctx.Int64("gas-limit"),
			Method:     method,
			Params:     params,
		}

		if cctx.Int64("nonce") > 0 {
			msg.Nonce = uint64(cctx.Int64("nonce"))
			sm, err := api.WalletSignMessage(ctx, stakerAddr, msg)
			if err != nil {
				return err
			}

			_, err = api.MpoolPush(ctx, sm)
			if err != nil {
				return err
			}
			fmt.Println(sm.Cid())
		} else {
			sm, err := api.MpoolPushMessage(ctx, msg, nil)
			if err != nil {
				return err
			}
			fmt.Println(sm.Cid())
		}

		return nil
	},
}
