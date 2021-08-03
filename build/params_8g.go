// +build debug 8g

package build

import (
	"math"
	"os"

	"github.com/filecoin-project/go-state-types/abi"

	"github.com/filecoin-project/lotus/chain/actors/policy"
)

const UpgradeBreezeHeight = -1
const BreezeGasTampingDuration = 0

const UpgradeSmokeHeight = -1
const UpgradeIgnitionHeight = -2
const UpgradeRefuelHeight = -3
const UpgradeTapeHeight = -4

var UpgradeActorsV2Height = abi.ChainEpoch(10)
var UpgradeLiftoffHeight = abi.ChainEpoch(-5)

const UpgradeKumquatHeight = -6

const Upgrade8GiBSectorHeight = -7

const UpgradeStakeHeight = 12

const Upgrade8GiBPoStGasHeight = 13

var DrandSchedule = map[abi.ChainEpoch]DrandEnum{
	0: DrandMainnet,
}

func init() {
	policy.SetSupportedProofTypes(abi.RegisteredSealProof_StackedDrg8GiBV1)
	policy.SetConsensusMinerMinPower(abi.NewStoragePower(8 << 30))
	policy.SetMinVerifiedDealSize(abi.NewStoragePower(256))

	if os.Getenv("LOTUS_DISABLE_V2_ACTOR_MIGRATION") == "1" {
		UpgradeActorsV2Height = math.MaxInt64
		UpgradeLiftoffHeight = 11
	}

	BuildType |= Build8g
}

const BlockDelaySecs = uint64(4)

const PropagationDelaySecs = uint64(1)

// SlashablePowerDelay is the number of epochs after ElectionPeriodStart, after
// which the miner is slashed
//
// Epochs
const SlashablePowerDelay = 20

// Epochs
const InteractivePoRepConfidence = 6
