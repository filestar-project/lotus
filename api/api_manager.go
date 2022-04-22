package api

import (
	"fmt"

	"github.com/filecoin-project/lotus/api/web3_api"
)

// singleton manager for initialize global api
type APIManager struct {
	web3 web3_api.FullWeb3Interface
	api  FullNode
}

func (manager *APIManager) InitFullNode(init FullNode) error {
	if manager.api != nil {
		return fmt.Errorf("full node already initialized")
	}
	manager.api = init
	return nil
}

func (manager *APIManager) InitFullWeb3(init web3_api.FullWeb3Interface) error {
	if manager.web3.Web3 != nil || manager.web3.Net != nil ||
		manager.web3.Eth != nil || manager.web3.Trace != nil {
		return fmt.Errorf("web3 node already initialized")
	}
	manager.web3 = init
	return nil
}

func (manager *APIManager) GetFullNode() FullNode {
	return manager.api
}

func (manager *APIManager) GetFullWeb3() web3_api.FullWeb3Interface {
	return manager.web3
}

var GlobalManager APIManager
