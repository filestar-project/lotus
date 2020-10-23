package web3_api

import (
	"context"
)

type Web3Info interface {
	ClientVersion(ctx context.Context) (string, error)
	Sha3(ctx context.Context, data HexString) (string, error)
}
