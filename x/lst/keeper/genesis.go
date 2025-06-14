package keeper

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/celestiaorg/celestia-app/v4/x/lst/types"
)

// InitGenesis initializes the module's state from a provided genesis state.
func (k *Keeper) InitGenesis(ctx sdk.Context, genState types.GenesisState) error {
	// Initialize params
	if err := k.SetParams(ctx, genState.Params); err != nil {
		return err
	}

	// Initialize baskets
	for _, basket := range genState.Baskets {
		k.SetBasket(ctx, basket)
		k.SetBasketByDenom(ctx, basket.Denom, basket.Id)
	}

	// Initialize pending redemptions
	for _, redemption := range genState.PendingRedemptions {
		k.SetPendingRedemption(ctx, redemption)
	}

	// Set next IDs
	if genState.NextBasketId > 0 {
		k.SetNextBasketID(ctx, genState.NextBasketId)
	}
	if genState.NextPendingId > 0 {
		k.SetNextPendingID(ctx, genState.NextPendingId)
	}

	return nil
}

// ExportGenesis returns the module's exported genesis
func (k *Keeper) ExportGenesis(ctx sdk.Context) *types.GenesisState {
	genesis := types.DefaultGenesis()
	genesis.Params = k.GetParams(ctx)

	// Export all baskets
	genesis.Baskets = k.GetAllBaskets(ctx)

	// Export all pending redemptions
	genesis.PendingRedemptions = k.GetAllPendingRedemptions(ctx)

	// Export next IDs
	store := k.Store(ctx)
	
	// Get next basket ID
	if bz := store.Get(types.NextBasketIDKey); bz != nil {
		genesis.NextBasketId = sdk.BigEndianToUint64(bz)
	}
	
	// Get next pending ID
	if bz := store.Get(types.NextPendingIDKey); bz != nil {
		genesis.NextPendingId = sdk.BigEndianToUint64(bz)
	}

	return genesis
}

// GetParams get all parameters as types.Params
func (k Keeper) GetParams(ctx context.Context) types.Params {
	return types.NewParams()
}

// SetParams set the params
func (k Keeper) SetParams(ctx context.Context, params types.Params) error {
	return nil
}