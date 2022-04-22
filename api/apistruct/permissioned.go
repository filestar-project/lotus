package apistruct

import (
	"github.com/filecoin-project/go-jsonrpc/auth"
	"github.com/filecoin-project/lotus/api"
	web3 "github.com/filecoin-project/lotus/api/web3_api"
)

const (
	// When changing these, update docs/API.md too

	PermRead  auth.Permission = "read" // default
	PermWrite auth.Permission = "write"
	PermSign  auth.Permission = "sign"  // Use wallet keys for signing
	PermAdmin auth.Permission = "admin" // Manage permissions
)

var AllPermissions = []auth.Permission{PermRead, PermWrite, PermSign, PermAdmin}
var DefaultPerms = []auth.Permission{PermRead}

func PermissionedStorMinerAPI(a api.StorageMiner) api.StorageMiner {
	var out StorageMinerStruct
	auth.PermissionedProxy(AllPermissions, DefaultPerms, a, &out.Internal)
	auth.PermissionedProxy(AllPermissions, DefaultPerms, a, &out.CommonStruct.Internal)
	return &out
}

func PermissionedFullAPI(a api.FullNode) api.FullNode {
	var out FullNodeStruct
	auth.PermissionedProxy(AllPermissions, DefaultPerms, a, &out.Internal)
	auth.PermissionedProxy(AllPermissions, DefaultPerms, a, &out.CommonStruct.Internal)
	return &out
}

func PermissionedWorkerAPI(a api.WorkerAPI) api.WorkerAPI {
	var out WorkerStruct
	auth.PermissionedProxy(AllPermissions, DefaultPerms, a, &out.Internal)
	return &out
}

func PermissionedWalletAPI(a api.WalletAPI) api.WalletAPI {
	var out WalletStruct
	auth.PermissionedProxy(AllPermissions, DefaultPerms, a, &out.Internal)
	return &out
}

func PermissionedWeb3Info(a web3.Web3Info) web3.Web3Info {
	var out Web3InfoStruct
	auth.PermissionedProxy(AllPermissions, DefaultPerms, a, &out.Internal)
	return &out
}

func PermissionedNetInfo(a web3.NetInfo) web3.NetInfo {
	var out NetInfoStruct
	auth.PermissionedProxy(AllPermissions, DefaultPerms, a, &out.Internal)
	return &out
}

func PermissionedEthFunc(a web3.EthFunctionality) web3.EthFunctionality {
	var out EthFunctionalityStruct
	auth.PermissionedProxy(AllPermissions, DefaultPerms, a, &out.EthInfoStruct.Internal)
	auth.PermissionedProxy(AllPermissions, DefaultPerms, a, &out.Internal)
	return &out
}

func PermissionedTraceFunc(a web3.TraceFunctionality) web3.TraceFunctionality {
	var out TraceFunctionalityStruct
	auth.PermissionedProxy(AllPermissions, DefaultPerms, a, &out.Internal)
	return &out
}
