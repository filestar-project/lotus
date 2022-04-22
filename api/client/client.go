package client

import (
	"context"
	"net/http"
	"net/url"
	"path"
	"time"

	"github.com/filecoin-project/go-jsonrpc"

	"github.com/filecoin-project/lotus/api"
	"github.com/filecoin-project/lotus/api/apistruct"
	web3 "github.com/filecoin-project/lotus/api/web3_api"
	"github.com/filecoin-project/lotus/lib/rpcenc"
)

// NewCommonRPC creates a new http jsonrpc client.
func NewCommonRPC(ctx context.Context, addr string, requestHeader http.Header) (api.Common, jsonrpc.ClientCloser, error) {
	var res apistruct.CommonStruct
	closer, err := jsonrpc.NewMergeClient(ctx, addr, "Filecoin",
		[]interface{}{
			&res.Internal,
		},
		requestHeader,
	)

	return &res, closer, err
}

// NewFullNodeRPC creates a new http jsonrpc client.
func NewFullNodeRPC(ctx context.Context, addr string, requestHeader http.Header) (api.FullNode, jsonrpc.ClientCloser, error) {
	var res apistruct.FullNodeStruct
	closer, err := jsonrpc.NewMergeClient(ctx, addr, "Filecoin",
		[]interface{}{
			&res.CommonStruct.Internal,
			&res.Internal,
		}, requestHeader)

	return &res, closer, err
}

// NewStorageMinerRPC creates a new http jsonrpc client for miner
func NewStorageMinerRPC(ctx context.Context, addr string, requestHeader http.Header, opts ...jsonrpc.Option) (api.StorageMiner, jsonrpc.ClientCloser, error) {
	var res apistruct.StorageMinerStruct
	closer, err := jsonrpc.NewMergeClient(ctx, addr, "Filecoin",
		[]interface{}{
			&res.CommonStruct.Internal,
			&res.Internal,
		},
		requestHeader,
		opts...,
	)

	return &res, closer, err
}

func NewWorkerRPC(ctx context.Context, addr string, requestHeader http.Header) (api.WorkerAPI, jsonrpc.ClientCloser, error) {
	u, err := url.Parse(addr)
	if err != nil {
		return nil, nil, err
	}
	switch u.Scheme {
	case "ws":
		u.Scheme = "http"
	case "wss":
		u.Scheme = "https"
	}
	///rpc/v0 -> /rpc/streams/v0/push

	u.Path = path.Join(u.Path, "../streams/v0/push")

	var res apistruct.WorkerStruct
	closer, err := jsonrpc.NewMergeClient(ctx, addr, "Filecoin",
		[]interface{}{
			&res.Internal,
		},
		requestHeader,
		rpcenc.ReaderParamEncoder(u.String()),
		jsonrpc.WithNoReconnect(),
		jsonrpc.WithTimeout(30*time.Second),
	)

	return &res, closer, err
}

// NewGatewayRPC creates a new http jsonrpc client for a gateway node.
func NewGatewayRPC(ctx context.Context, addr string, requestHeader http.Header, opts ...jsonrpc.Option) (api.GatewayAPI, jsonrpc.ClientCloser, error) {
	var res apistruct.GatewayStruct
	closer, err := jsonrpc.NewMergeClient(ctx, addr, "Filecoin",
		[]interface{}{
			&res.Internal,
		},
		requestHeader,
		opts...,
	)

	return &res, closer, err
}

func NewWalletRPC(ctx context.Context, addr string, requestHeader http.Header) (api.WalletAPI, jsonrpc.ClientCloser, error) {
	var res apistruct.WalletStruct
	closer, err := jsonrpc.NewMergeClient(ctx, addr, "Filecoin",
		[]interface{}{
			&res.Internal,
		},
		requestHeader,
	)

	return &res, closer, err
}

// NewWeb3RPC creates a new http jsonrpc client.
func NewWeb3RPC(ctx context.Context, addr string, requestHeader http.Header) (web3.Web3Info, jsonrpc.ClientCloser, error) {
	var res apistruct.Web3InfoStruct
	closer, err := jsonrpc.NewMergeClient(ctx, addr, "web3",
		[]interface{}{
			&res.Internal,
		},
		requestHeader,
	)

	return &res, closer, err
}

// NewNetRPC creates a new http jsonrpc client.
func NewNetRPC(ctx context.Context, addr string, requestHeader http.Header) (web3.NetInfo, jsonrpc.ClientCloser, error) {
	var res apistruct.NetInfoStruct
	closer, err := jsonrpc.NewMergeClient(ctx, addr, "net",
		[]interface{}{
			&res.Internal,
		},
		requestHeader,
	)

	return &res, closer, err
}

// NewEthFuncRPC creates a new http jsonrpc client.
func NewEthRPC(ctx context.Context, addr string, requestHeader http.Header) (web3.EthFunctionality, jsonrpc.ClientCloser, error) {
	var res apistruct.EthFunctionalityStruct
	closer, err := jsonrpc.NewMergeClient(ctx, addr, "eth",
		[]interface{}{
			&res.EthInfoStruct.Internal,
			&res.Internal,
		},
		requestHeader,
	)

	return &res, closer, err
}

// NewEthFuncRPC creates a new http jsonrpc client.
func NewTraceRPC(ctx context.Context, addr string, requestHeader http.Header) (web3.TraceFunctionality, jsonrpc.ClientCloser, error) {
	var res apistruct.TraceFunctionalityStruct
	closer, err := jsonrpc.NewMergeClient(ctx, addr, "trace",
		[]interface{}{
			&res.Internal,
		},
		requestHeader,
	)

	return &res, closer, err
}
