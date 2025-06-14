package keeper

import (
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/celestiaorg/celestia-app/v4/x/lst/types"
)

// InitGenesis initializes the module's state from a provided genesis state.
func (k *Keeper) InitGenesis(ctx sdk.Context, genState types.GenesisState) error {
	// Initialize params
	if err := k.SetParams(ctx, genState.Params); err != nil {
		return err
	}

	return nil
}

// ExportGenesis returns the module's exported genesis
func (k *Keeper) ExportGenesis(ctx sdk.Context) *types.GenesisState {
	genesis := types.DefaultGenesis()
	genesis.Params = k.GetParams(ctx)

	return genesis
}

// GetParams get all parameters as types.Params
func (k Keeper) GetParams(ctx sdk.Context) types.Params {
	return types.NewParams()
}

// SetParams set the params
func (k Keeper) SetParams(ctx sdk.Context, params types.Params) error {
	return nil
}