package main

import (
	"encoding/hex"
	"fmt"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/lotus/chain/actors"
	stake2 "github.com/filecoin-project/specs-actors/v2/actors/builtin/stake"
)

func main() {
	msigParams := &stake2.ChangeMaturePeriodParams{
		MaturePeriod: abi.ChainEpoch(999),
	}

	encoded, err := actors.SerializeParams(msigParams)
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println(hex.EncodeToString(encoded))
}
