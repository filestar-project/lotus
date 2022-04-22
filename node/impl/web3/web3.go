package web3impl

import (
	"context"
	"encoding/hex"

	"go.uber.org/fx"

	"github.com/ethereum/go-ethereum/crypto"
	web3 "github.com/filecoin-project/lotus/api/web3_api"
	"github.com/filecoin-project/lotus/node/impl/common"
)

type Web3InfoAPI struct {
	fx.In

	common.CommonAPI
}

func (web3Info *Web3InfoAPI) ClientVersion(ctx context.Context) (string, error) {
	version, err := web3Info.Version(ctx)
	if err != nil {
		return "", err
	}
	return version.Version, nil
}

func (web3Info *Web3InfoAPI) Sha3(ctx context.Context, data web3.HexString) (string, error) {
	input, err := data.Bytes()
	if err != nil {
		return "0x", err
	}
	result := crypto.Keccak256(input)
	return "0x" + hex.EncodeToString(result), nil
}

var _ web3.Web3Info = &Web3InfoAPI{}
