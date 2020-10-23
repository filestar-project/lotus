package web3impl

import (
	"context"
	"fmt"

	"go.uber.org/fx"

	web3 "github.com/filecoin-project/lotus/api/web3_api"
	"github.com/filecoin-project/lotus/chain/types"
	"github.com/filecoin-project/lotus/node/impl/common"
	"github.com/filecoin-project/lotus/node/impl/full"
)

type NetInfoAPI struct {
	fx.In
	full.StateAPI
	common.CommonAPI
}

func (netInfo *NetInfoAPI) PeerCount(ctx context.Context) (web3.HexString, error) {
	peersInfo, err := netInfo.NetPeers(ctx)
	if err != nil {
		return "0x", err
	}
	return web3.GetHexString(int64(len(peersInfo))), nil
}
func (netInfo *NetInfoAPI) Listening(ctx context.Context) (bool, error) {
	return true, nil
}
func (netInfo *NetInfoAPI) Version(ctx context.Context) (string, error) {
	nver, err := netInfo.StateNetworkVersion(ctx, types.EmptyTSK)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%v", nver), nil
}

var _ web3.NetInfo = &NetInfoAPI{}
