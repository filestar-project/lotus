package build

import (
	"github.com/filecoin-project/go-address"

	"github.com/libp2p/go-libp2p-core/protocol"

	"github.com/filecoin-project/lotus/node/modules/dtypes"
)

// Core network constants

func BlocksTopic(netName dtypes.NetworkName) string   { return "/star/blocks/" + string(netName) }
func MessagesTopic(netName dtypes.NetworkName) string { return "/star/msgs/" + string(netName) }
func DhtProtocolName(netName dtypes.NetworkName) protocol.ID {
	return protocol.ID("/star/kad/" + string(netName))
}

func SetAddressNetwork(n address.Network) {
	address.CurrentNetwork = n
}
