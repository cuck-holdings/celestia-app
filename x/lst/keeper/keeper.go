package keeper

import (
	"context"
	"fmt"

	"cosmossdk.io/log"
	storetypes "cosmossdk.io/store/types"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	paramtypes "github.com/cosmos/cosmos-sdk/x/params/types"

	"github.com/celestiaorg/celestia-app/v4/x/lst/types"
)

// Keeper handles all the state changes for the lst module.
type Keeper struct {
	cdc            codec.Codec
	storeKey       storetypes.StoreKey
	legacySubspace paramtypes.Subspace
	authority      string
}

func NewKeeper(
	cdc codec.Codec,
	storeKey storetypes.StoreKey,
	legacySubspace paramtypes.Subspace,
	authority string,
) *Keeper {
	if !legacySubspace.HasKeyTable() {
		legacySubspace = legacySubspace.WithKeyTable(types.ParamKeyTable())
	}

	return &Keeper{
		cdc:            cdc,
		storeKey:       storeKey,
		legacySubspace: legacySubspace,
		authority:      authority,
	}
}

// GetAuthority returns the module's authority.
func (k Keeper) GetAuthority() string {
	return k.authority
}

// Logger returns a module-specific logger.
func (k Keeper) Logger(ctx context.Context) log.Logger {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	return sdkCtx.Logger().With("module", fmt.Sprintf("x/%s", types.ModuleName))
}