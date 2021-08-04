package cli

import (
	"fmt"
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/lotus/chain/actors"
	"github.com/filecoin-project/lotus/chain/actors/builtin/token"
	"github.com/filecoin-project/lotus/chain/types"
	builtin2 "github.com/filecoin-project/specs-actors/v2/actors/builtin"
	token2 "github.com/filecoin-project/specs-actors/v2/actors/builtin/token"
	"github.com/urfave/cli/v2"
	"golang.org/x/xerrors"
)

var tokenCmd = &cli.Command{
	Name:"token",
	Usage:"Interact with filestar token",
	Subcommands: []*cli.Command{
		tokenInfoCmd,
		tokenCreateCmd,
		tokenURICmd,
		tokenCreatorsCmd,
		tokenMintBatchCmd,
		tokenBalancesCmd,
		tokenSafeTransferCmd,
		tokenApprovesCmd,
	},
}

var tokenURICmd = &cli.Command{
	Name: "uri",
	Usage: "URI get or set",
	Subcommands: []*cli.Command{
		tokenGetURICmd,
		tokenSetURICmd,
	},
}

var tokenApprovesCmd = &cli.Command{
	Name: "approves",
	Usage: "approve get or set",
	Subcommands: []*cli.Command{
		tokenApprovesGetCmd,
		tokenApprovesSetCmd,
	},
}

var tokenInfoCmd = &cli.Command{
	Name: "info",
	Usage: "Print token infomation",
	ArgsUsage: "[tokenAddress (optional]",
	Action: func(cctx *cli.Context) error {
		api, closer, err := GetFullNodeAPI(cctx)
		if err != nil {
			return err
		}
		defer closer()

		ctx := ReqContext(cctx)
		si, err := api.StateTokenInfo(ctx, types.EmptyTSK)
		if err != nil {
			return err
		}
		fmt.Printf("Total token Nonce: %v.\n", si.Nonce)

		var tokenAddr address.Address
		if cctx.Args().Present() {
			tokenAddr, err = address.NewFromString(cctx.Args().First())
			if err != nil {
				return err
			}
		} else {
			tokenAddr, err = api.WalletDefaultAddress(ctx)
			if err != nil {
				return err
			}
		}

		tokenAddrID, err := api.StateLookupID(ctx, tokenAddr, types.EmptyTSK)
		if err != nil {
			return err
		}

		fmt.Printf("\nTokenAddrID: %s.\n", tokenAddrID)

		createTokenIDs, err := api.StateTokenIDsByCreator(ctx, tokenAddrID, types.EmptyTSK)
		if err != nil {
			return err
		}
		if len(createTokenIDs) == 0 {
			fmt.Printf("\nTotal 0 tokens created by %s.\n", tokenAddrID)
		} else {
			fmt.Printf("\nTotal %d tokens created by %s:\n", len(createTokenIDs), tokenAddrID)
			for _, tokenID := range createTokenIDs {
				fmt.Printf("%v ", tokenID)
			}
		}

		balancesWithTokenID, err := api.StateTokenBalanceByAddr(ctx, tokenAddrID, types.EmptyTSK)
		if err != nil {
			return err
		}
		if balancesWithTokenID == nil || len(balancesWithTokenID) == 0{
			fmt.Printf("\nTotal 0 tokens in %s\n.", tokenAddrID)
			return nil
		}

		cnt := 0
		fmt.Printf("\nTokens in %s and their amount infos :\n", tokenAddrID)
		for _, balanceWithTokenID := range balancesWithTokenID {
			if balanceWithTokenID == nil {
				cnt++
				continue
			}
			fmt.Printf("TokenID : %v, Amount : %v.\n", balanceWithTokenID.TokenID, balanceWithTokenID.Balance)
		}
		if cnt == len(balancesWithTokenID) {
			fmt.Printf("<nil>.\n")
		}
		return nil
	},
}

var tokenCreatorsCmd = &cli.Command{
	Name: "creators",
	Usage: "Print token creators",
	ArgsUsage: "[tokenID (optional)]",
	Action: func(cctx *cli.Context) error {
		api, closer, err := GetFullNodeAPI(cctx)
		if err != nil {
			return err
		}
		defer closer()

		ctx := ReqContext(cctx)

		if cctx.Args().Present() {
			tokenID, err := big.FromString(cctx.Args().First())
			if err != nil {
				return err
			}
			creator, err := api.StateTokenCreatorByTokenID(ctx, tokenID, types.EmptyTSK)
			if err != nil {
				return err
			}
			fmt.Printf("Token %v is created by %s.\n", tokenID, creator)
		} else {
			creators, err := api.StateTokenCreators(ctx, types.EmptyTSK)
			if err != nil {
				return err
			}
			if creators == nil || len(creators) == 0 {
				fmt.Printf("There are 0 tokens now.\n")
			} else {
				fmt.Printf("There are %d tokens and their creators: \n", len(creators))
				for idx, creator := range creators {
					fmt.Printf("TokenID : %v, creator : %s.\n", big.NewInt(int64(idx + 1)), creator)
				}
			}
		}

		return nil
	},
}

var tokenGetURICmd = &cli.Command{
	Name: "get",
	Usage: "Print token uri by tokenID",
	ArgsUsage: "[tokenID]",
	Action: func(cctx *cli.Context) error {
		if cctx.Args().Len() < 1 {
			return ShowHelp(cctx, fmt.Errorf("'token uri get' expects at least one argument: tokenID"))
		}

		api, closer, err := GetFullNodeAPI(cctx)
		if err != nil {
			return err
		}
		defer closer()

		ctx := ReqContext(cctx)

		tokenID, err := big.FromString(cctx.Args().First())
		if err != nil {
			return err
		}

		uri, err := api.StateTokenURIByTokenID(ctx, tokenID, types.EmptyTSK)
		if err != nil {
			return err
		}
		if uri == "" {
			fmt.Printf("URI with tokenID %v is null.\n", tokenID)
			return nil
		}

		fmt.Printf("URI with tokenID %v is %s.\n", tokenID, uri)
		return nil
	},
}

var tokenBalancesCmd = &cli.Command{
	Name: "balances",
	Usage: "Print token balance details",
	ArgsUsage: "[tokenID(s)] [owners(s)], At least one of the two parameters(tokenID and owner) must be given, If you " +
		"want to get the corresponding balance amount between a set of token ID and owner, both parameters(tokenIDs and" +
		" owners) must be given",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name: "tokenID",
			Usage: "specify tokenID to use in balances",
		},
		&cli.StringFlag{
			Name: "owner",
			Usage: "specify ownerAddress to use in balances",
		},
		&cli.StringFlag{
			Name: "tokenIDs",
			Usage: "specify tokenIDs to use in balances",
		},
		&cli.StringFlag{
			Name: "owners",
			Usage: "specify ownersAddress to use in balances",
		},
	},
	Action: func(cctx *cli.Context) error {
		api, closer, err := GetFullNodeAPI(cctx)
		if err != nil {
			return err
		}
		defer closer()

		ctx := ReqContext(cctx)

		tokenID, _ := big.FromString(cctx.String("tokenID"))

		ownerAddress, _ := address.NewFromString(cctx.String("owner"))

		if !tokenID.Nil() && !tokenID.LessThan(big.Zero()) && !ownerAddress.Empty() {
			ownerAddrID, err := api.StateLookupID(ctx, ownerAddress, types.EmptyTSK)
			if err != nil {
				return err
			}
			fmt.Printf("OwnerAddressID: %v.\n", ownerAddrID)
			tokenAmount, err := api.StateTokenBalanceByTokenIDAndAddr(ctx, tokenID, ownerAddrID, types.EmptyTSK)
			if err != nil {
				return err
			}
			fmt.Printf("owner %s has %v token with tokenID %v.\n", ownerAddrID, tokenAmount, tokenID)
		} else if (tokenID.Nil() || tokenID.LessThan(big.Zero())) && !ownerAddress.Empty() {
			ownerAddrID, err := api.StateLookupID(ctx, ownerAddress, types.EmptyTSK)
			if err != nil {
				return err
			}
			fmt.Printf("OwnerAddressID: %v.\n", ownerAddrID)
			tokenAmountsByAddr, err := api.StateTokenBalanceByAddr(ctx, ownerAddrID, types.EmptyTSK)
			if err != nil {
				return err
			}
			if tokenAmountsByAddr == nil || len(tokenAmountsByAddr) == 0 {
				fmt.Printf("owner %s has 0 token now.\n", ownerAddrID)
			} else {
				cnt := 0
				fmt.Printf("Tokens in owner %s and their amounts: \n", ownerAddress)
				for _, tokenAmount := range tokenAmountsByAddr {
					if tokenAmount == nil {
						cnt++
						continue
					}
					fmt.Printf("TokenID : %v, Amount : %v.\n", tokenAmount.TokenID, tokenAmount.Balance)
				}
				if cnt == len(tokenAmountsByAddr) {
					fmt.Printf("<nil>.\n")
				}
			}
		} else if !tokenID.Nil() && !tokenID.LessThan(big.Zero()) && ownerAddress.Empty() {
			tokenAmountsByTokenID, err := api.StateTokenBalanceByTokenID(ctx, tokenID, types.EmptyTSK)
			if err != nil {
				return err
			}
			if tokenAmountsByTokenID == nil || len(tokenAmountsByTokenID) == 0 {
				fmt.Printf("Token with tokenID %s is only a type and no address own it.\n", tokenID)
			} else {
				fmt.Printf("Token with tokenID %s is shared by %d owners and their amounts: \n", tokenID, len(tokenAmountsByTokenID))
				for _, tokenAmount := range tokenAmountsByTokenID {
					if tokenAmount == nil {
						continue
					}
					fmt.Printf("Owner : %s, Amount : %v.\n", tokenAmount.Owner, tokenAmount.Balance)
				}
			}
		} else {
			tokenIDsString := cctx.String("tokenIDs")
			ownersAddressString := cctx.String("owners")
			if tokenIDsString == "" || ownersAddressString == "" {
				return xerrors.Errorf("It seems that you want to get the balance amount corresponding to a set of token" +
					" ID and address, please give the standard parameter format, refer to: tokenIDs [tokenID1," +
					"tokenID2,...] owners [owner1,owner2,...]")
			}
			tokenIDs := ParseMultiBigNumParams(tokenIDsString)
			owners := ParseMultiOwnerAddrParams(ownersAddressString)
			for idx, _ := range owners {
				tokenAddrID, err := api.StateLookupID(ctx, owners[idx], types.EmptyTSK)
				if err != nil {
					return err
				}
				fmt.Printf("OwnerAddressID: %v\n", tokenAddrID)
				owners[idx] = tokenAddrID
			}

			tokenAmounts, err := api.StateTokenBalanceByTokenIDsAndAddrs(ctx, tokenIDs, owners, types.EmptyTSK)
			if err != nil {
				return err
			}
			for idx, v := range tokenAmounts {
				fmt.Printf("owner %s has %v token with tokenID %v.\n", owners[idx], v, tokenIDs[idx])
			}
		}
		return nil
	},
}

var tokenApprovesGetCmd = &cli.Command{
	Name: "get",
	Usage: "Print token isAllApproved results",
	ArgsUsage: "[addrFrom] [addrTo]",
	Action: func(cctx *cli.Context) error {
		if cctx.Args().Len() != 2 {
			return ShowHelp(cctx, fmt.Errorf("'token approves get' expects two argument: addrFrom and addrTo"))
		}
		api, closer, err := GetFullNodeAPI(cctx)
		if err != nil {
			return err
		}
		defer closer()

		ctx := ReqContext(cctx)

		addrFrom, _ := address.NewFromString(cctx.Args().First())
		addrFromID, err := api.StateLookupID(ctx, addrFrom, types.EmptyTSK)
		if err != nil {
			return err
		}

		addrTo, _ := address.NewFromString(cctx.Args().Get(1))
		addrToID, err := api.StateLookupID(ctx, addrTo, types.EmptyTSK)
		if err != nil {
			return err
		}
		fmt.Printf("\nAddrFromID: %v, AddrToID: %v\n", addrFromID, addrToID)

		if addrFromID == addrToID {
			return xerrors.Errorf("two address can not be the same")
		}

		isAllApproved, err := api.StateTokenIsAllApproved(ctx, addrFromID, addrToID, types.EmptyTSK)
		if err != nil {
			return err
		}
		fmt.Printf("address from %s to %s isAllApproved result is %v.\n", addrFromID, addrToID, isAllApproved)
		return nil
	},
}

var tokenCreateCmd = &cli.Command{
	Name: "create",
	Usage: "create a new toke type",
	ArgsUsage: "[uri(optional)] [amount(optional)]",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name: "uri",
			Usage: "specify uri to set for new token",
		},
		&cli.StringFlag{
			Name:  "amount",
			Usage: "specify init amount to set for new token",
			Value: "0",
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
		api, closer, err := GetFullNodeAPI(cctx)
		if err != nil {
			return err
		}
		defer closer()

		ctx := ReqContext(cctx)

		defaddr, err := api.WalletDefaultAddress(ctx)
		if err != nil {
			return err
		}

		uri := cctx.String("uri")
		var amount big.Int
		if amountString := cctx.String("amount"); amountString == "" {
			amount = big.Zero()
		} else {
			amount, err = types.BigFromString(amountString)
			if err != nil {
				return err
			}
		}

		gp, err := types.BigFromString(cctx.String("gas-premium"))
		if err != nil {
			return err
		}
		gfc, err := types.BigFromString(cctx.String("gas-feecap"))
		if err != nil {
			return err
		}

		params, err := actors.SerializeParams(&token2.CreateTokenParams{
			TokenURI: uri,
			ValueInit: amount,
		})
		if err != nil {
			return ShowHelp(cctx, fmt.Errorf("failed to serialize params: %w", err))
		}
		msg := &types.Message{
			From:       defaddr,
			To:         token.Address,
			GasPremium: gp,
			GasFeeCap:  gfc,
			GasLimit:   cctx.Int64("gas-limit"),
			Method:     builtin2.MethodsToken.Create,
			Params:     params,
		}

		if cctx.Int64("nonce") > 0 {
			msg.Nonce = uint64(cctx.Int64("nonce"))
			sm, err := api.WalletSignMessage(ctx, defaddr, msg)
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

var tokenMintBatchCmd = &cli.Command{
	Name: "mintBatch",
	Usage: "Mint tokens to multi address, only the token creator has the right to operate",
	ArgsUsage: "[tokenID], [addrTos] [amounts]",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name: "tokenID",
			Usage: "specify tokenID to mint",
		},
		&cli.StringFlag{
			Name:  "addrTos",
			Usage: "specify address set for mint to",
		},
		&cli.StringFlag{
			Name:  "amounts",
			Usage: "specify amount set to mint",
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
		api, closer, err := GetFullNodeAPI(cctx)
		if err != nil {
			return err
		}
		defer closer()

		ctx := ReqContext(cctx)

		defaddr, err := api.WalletDefaultAddress(ctx)
		if err != nil {
			return err
		}

		tokenID, err := types.BigFromString(cctx.String("tokenID"))
		if err != nil {
			return err
		}

		addrTosString := cctx.String("addrTos")
		tokenAmountsString := cctx.String("amounts")

		if addrTosString == "" || tokenAmountsString == "" {
			return ShowHelp(cctx, fmt.Errorf("'token mintBatch' expects addrTos and amounts can not be " +
				"empty, please refer to the format:addrTos [addrTo1,addrTo2,...] amounts [amount1,amount2,...]"))
		}

		addrTos := ParseMultiOwnerAddrParams(addrTosString)
		amounts := ParseMultiBigNumParams(tokenAmountsString)

		if len(addrTos) != len(amounts) {
			return ShowHelp(cctx, fmt.Errorf("'token mintBatch' expects length of addrTos is equal with amounts"))
		}

		for idx, _ := range addrTos {
			tokenAddrID, err := api.StateLookupID(ctx, addrTos[idx], types.EmptyTSK)
			if err != nil {
				return err
			}
			addrTos[idx] = tokenAddrID
		}

		gp, err := types.BigFromString(cctx.String("gas-premium"))
		if err != nil {
			return err
		}
		gfc, err := types.BigFromString(cctx.String("gas-feecap"))
		if err != nil {
			return err
		}

		params, err := actors.SerializeParams(&token2.MintBatchTokenParams{
			TokenID: tokenID,
			AddrTos: addrTos,
			Values: amounts,
		})
		if err != nil {
			return ShowHelp(cctx, fmt.Errorf("failed to serialize params: %w", err))
		}
		msg := &types.Message{
			From:       defaddr,
			To:         token.Address,
			GasPremium: gp,
			GasFeeCap:  gfc,
			GasLimit:   cctx.Int64("gas-limit"),
			Method:     builtin2.MethodsToken.MintBatch,
			Params:     params,
		}

		if cctx.Int64("nonce") > 0 {
			msg.Nonce = uint64(cctx.Int64("nonce"))
			sm, err := api.WalletSignMessage(ctx, defaddr, msg)
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

var tokenSetURICmd = &cli.Command{
	Name: "set",
	Usage: "Execute setURI operation",
	ArgsUsage: "[tokenID] [uri]",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name: "tokenID",
			Usage: "specify uri to set for new token",
		},
		&cli.StringFlag{
			Name:  "uri",
			Usage: "specify init amount to set for new token",
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

		api, closer, err := GetFullNodeAPI(cctx)
		if err != nil {
			return err
		}
		defer closer()

		ctx := ReqContext(cctx)

		defaddr, err := api.WalletDefaultAddress(ctx)
		if err != nil {
			return err
		}

		tokenID, err := big.FromString(cctx.String("tokenID"))
		if err != nil {
			return err
		}

		newURI := cctx.String("uri")
		if newURI == "" {
			return ShowHelp(cctx, fmt.Errorf("'token uri set' expects the new uri argument can not be empty"))
		}

		gp, err := types.BigFromString(cctx.String("gas-premium"))
		if err != nil {
			return err
		}
		gfc, err := types.BigFromString(cctx.String("gas-feecap"))
		if err != nil {
			return err
		}

		params, err := actors.SerializeParams(&token2.ChangeURIParams{
			TokenID: tokenID,
			NewURI: newURI,
		})
		if err != nil {
			return ShowHelp(cctx, fmt.Errorf("failed to serialize params: %w", err))
		}

		msg := &types.Message{
			From:       defaddr,
			To:         token.Address,
			GasPremium: gp,
			GasFeeCap:  gfc,
			GasLimit:   cctx.Int64("gas-limit"),
			Method:     builtin2.MethodsToken.ChangeURI,
			Params:     params,
		}

		if cctx.Int64("nonce") > 0 {
			msg.Nonce = uint64(cctx.Int64("nonce"))
			sm, err := api.WalletSignMessage(ctx, defaddr, msg)
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

var tokenSafeTransferCmd = &cli.Command{
	Name: "safeTransfer",
	Usage: "Execute transfer operation",
	ArgsUsage: "[addrFrom(optional)] [addrTo] [tokenIDs] [<tokenID1,tokenID2,...] [amounts] [<amount1,amount2,...]",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name: "from",
			Usage: "specify address transfer from",
		},
		&cli.StringFlag{
			Name: "to",
			Usage: "specify address transfer to",
		},
		&cli.StringFlag{
			Name: "tokenIDs",
			Usage: "specify tokenIDs to transfer",
		},
		&cli.StringFlag{
			Name: "amounts",
			Usage: "specify amounts to transfer",
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
		api, closer, err := GetFullNodeAPI(cctx)
		if err != nil {
			return err
		}
		defer closer()

		ctx := ReqContext(cctx)

		defaddr, err := api.WalletDefaultAddress(ctx)
		if err != nil {
			return err
		}

		var addrFrom address.Address
		if from := cctx.String("from"); from == "" {
			addrFrom = defaddr
		} else {
			addr, err := address.NewFromString(from)
			if err != nil {
				return err
			}
			addrFrom = addr
		}
		addrFromID, err := api.StateLookupID(ctx, addrFrom, types.EmptyTSK)
		if err != nil {
			return err
		}

		var addrTo address.Address
		if to := cctx.String("to"); to == "" {
			return ShowHelp(cctx, fmt.Errorf("'token safeTransfer' expects address transfer to can not be empty"))
		} else {
			addrTo , _ = address.NewFromString(to)
		}
		addrToID, err := api.StateLookupID(ctx, addrTo, types.EmptyTSK)
		if err != nil {
			return err
		}

		tokenIDsString := cctx.String("tokenIDs")
		tokenAmountsString := cctx.String("amounts")

		if tokenIDsString == "" || tokenAmountsString == "" {
			return ShowHelp(cctx, fmt.Errorf("'token safeTransfer' expects tokenIDs and amounts can not be " +
				"empty, please refer to the format:tokenIDs [tokenID1,tokenID2,...] amounts [amount1,amount2,...]"))
		}

		tokenIDs := ParseMultiBigNumParams(tokenIDsString)
		amounts := ParseMultiBigNumParams(tokenAmountsString)

		if len(tokenIDs) != len(amounts) {
			return ShowHelp(cctx, fmt.Errorf("'token safeTransfer' expects length of tokenIDs is equal with amounts"))
		}

		gp, err := types.BigFromString(cctx.String("gas-premium"))
		if err != nil {
			return err
		}
		gfc, err := types.BigFromString(cctx.String("gas-feecap"))
		if err != nil {
			return err
		}

		var msg *types.Message
		if len(tokenIDs) == 1 {
			params, err := actors.SerializeParams(&token2.SafeTransferFromParams{
				AddrFrom: addrFromID,
				AddrTo: addrToID,
				TokenID: tokenIDs[0],
				Value: amounts[0],
			})
			if err != nil {
				return ShowHelp(cctx, fmt.Errorf("failed to serialize params: %w", err))
			}
			msg = &types.Message{
				From:       defaddr,
				To:         token.Address,
				GasPremium: gp,
				GasFeeCap:  gfc,
				GasLimit:   cctx.Int64("gas-limit"),
				Method:     builtin2.MethodsToken.SafeTransferFrom,
				Params:     params,
			}
		} else {
			params, err := actors.SerializeParams(&token2.SafeBatchTransferFromParams{
				AddrFrom: addrFromID,
				AddrTo: addrToID,
				TokenIDs: tokenIDs,
				Values: amounts,
			})
			if err != nil {
				return ShowHelp(cctx, fmt.Errorf("failed to serialize params: %w", err))
			}
			msg = &types.Message{
				From:       defaddr,
				To:         token.Address,
				Value:      big.Zero(),
				GasPremium: gp,
				GasFeeCap:  gfc,
				GasLimit:   cctx.Int64("gas-limit"),
				Method:     builtin2.MethodsToken.SafeBatchTransferFrom,
				Params: 	params,
			}
		}

		if cctx.Int64("nonce") > 0 {
			msg.Nonce = uint64(cctx.Int64("nonce"))
			sm, err := api.WalletSignMessage(ctx, addrFrom, msg)
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

var tokenApprovesSetCmd = &cli.Command{
	Name: "set",
	Usage: "Set token isAllApproved",
	ArgsUsage: "[addrTo] [approved]",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "to",
			Usage: "specify address set all approved to",
		},
		&cli.BoolFlag{
			Name:    "approved",
			Usage:   "specify approve status to set",
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

		api, closer, err := GetFullNodeAPI(cctx)
		if err != nil {
			return err
		}
		defer closer()

		ctx := ReqContext(cctx)

		defaddr, err := api.WalletDefaultAddress(ctx)
		if err != nil {
			return err
		}

		var addrTo address.Address
		if to := cctx.String("to"); to == "" {
			return ShowHelp(cctx, fmt.Errorf("'token approves set' expects address approve to can not be empty"))
		} else {
			addrTo , _ = address.NewFromString(to)
		}
		addrToID, err := api.StateLookupID(ctx, addrTo, types.EmptyTSK)
		if err != nil {
			return err
		}
		approved := cctx.Bool("approved")

		gp, err := types.BigFromString(cctx.String("gas-premium"))
		if err != nil {
			return err
		}
		gfc, err := types.BigFromString(cctx.String("gas-feecap"))
		if err != nil {
			return err
		}

		params, err := actors.SerializeParams(&token2.SetApproveForAllParams{
			AddrTo: addrToID,
			Approved: approved,
		})
		if err != nil {
			return ShowHelp(cctx, fmt.Errorf("failed to serialize params: %w", err))
		}
		msg := &types.Message{
			From:       defaddr,
			To:         token.Address,
			GasPremium: gp,
			GasFeeCap:  gfc,
			GasLimit:   cctx.Int64("gas-limit"),
			Method:     builtin2.MethodsToken.SetApproveForAll,
			Params:     params,
		}

		if cctx.Int64("nonce") > 0 {
			msg.Nonce = uint64(cctx.Int64("nonce"))
			sm, err := api.WalletSignMessage(ctx, defaddr, msg)
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


func ParseMultiBigNumParams(params string) []big.Int {
	var bigNums []big.Int
	var idx = 0
	for idx < len(params) {
		if params[idx] != ' ' && params[idx] != '[' && params[idx] != ']' && params[idx] != ',' {
			var strTmp string
			for idx < len(params) && params[idx] != ' ' && params[idx] != '[' && params[idx] != ']' && params[idx] != ',' {
				strTmp = strTmp + string(params[idx])
				idx++
			}
			tokenID, _ := types.BigFromString(strTmp)
			bigNums = append(bigNums, tokenID)
		} else {
			idx++
		}
	}
	return bigNums
}

func ParseMultiOwnerAddrParams(params string) []address.Address {
	var owners []address.Address
	var idx = 0
	for idx < len(params) {
		if params[idx] != ' ' && params[idx] != '[' && params[idx] != ']' && params[idx] != ',' {
			var strTmp string
			for idx < len(params) && params[idx] != ' ' && params[idx] != '[' && params[idx] != ']' && params[idx] != ',' {
				strTmp = strTmp + string(params[idx])
				idx++
			}
			owner, _ := address.NewFromString(strTmp)
			owners = append(owners, owner)
		} else {
			idx++
		}
	}
	return owners
}