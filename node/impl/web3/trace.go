package web3impl

import (
	"context"

	"go.uber.org/fx"

	"github.com/filecoin-project/go-state-types/abi"
	web3 "github.com/filecoin-project/lotus/api/web3_api"
	"github.com/filecoin-project/lotus/chain/types"
	"github.com/filecoin-project/lotus/node/impl/full"
)

type TraceFunctionalityAPI struct {
	fx.In
	full.StateAPI
	full.ChainAPI
}

func (traceFunc *TraceFunctionalityAPI) Filter(ctx context.Context, filter web3.TraceFilter) ([]web3.TraceBlock, error) {
	result := make([]web3.TraceBlock, 0)
	execFilter, err := convertTraceFilter(filter)
	if err != nil {
		return []web3.TraceBlock{}, err
	}
	if filter.FromBlock == "" {
		filter.FromBlock = "0x01"
	}
	if filter.ToBlock == "" {
		filter.ToBlock = "latest"
	}
	start, err := getTipsetNumber(&traceFunc.ChainAPI, ctx, filter.FromBlock)
	if err != nil {
		return []web3.TraceBlock{}, err
	}
	end, err := getTipsetNumber(&traceFunc.ChainAPI, ctx, filter.ToBlock)
	if err != nil {
		return []web3.TraceBlock{}, err
	}
	after, err := web3.ConvertQuantity(filter.After).ToInt()
	if err != nil {
		return []web3.TraceBlock{}, err
	}
	count, err := web3.ConvertQuantity(filter.Count).ToInt()
	if err != nil {
		return []web3.TraceBlock{}, err
	}
	for i := start; i <= end; i++ {
		currentTipset, err := traceFunc.ChainGetTipSetByHeight(ctx, abi.ChainEpoch(i), types.EmptyTSK)
		if err != nil {
			return []web3.TraceBlock{}, err
		}
		_, trace, err := traceFunc.StateManager.ExecutionTraceWithFilter(ctx, execFilter, currentTipset)
		if err != nil {
			return []web3.TraceBlock{}, err
		}
		result = append(result, convertTraceResult(trace, currentTipset)...)
	}
	if int64(len(result)) < after+count {
		return []web3.TraceBlock{}, nil
	}
	if count == 0 {
		return result[after:], nil
	}
	return result[after : after+count], nil
}

var _ web3.TraceFunctionality = &TraceFunctionalityAPI{}
