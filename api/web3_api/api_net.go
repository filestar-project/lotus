package web3_api

import (
	"context"
)

type NetInfo interface {
	PeerCount(ctx context.Context) (HexString, error)
	Listening(ctx context.Context) (bool, error)
	Version(ctx context.Context) (string, error)
}
