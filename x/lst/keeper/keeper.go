package keeper

import (
	"context"
	"encoding/binary"
	"fmt"
	"strconv"

	"cosmossdk.io/log"
	"cosmossdk.io/math"
	storetypes "cosmossdk.io/store/types"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authkeeper "github.com/cosmos/cosmos-sdk/x/auth/keeper"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	paramtypes "github.com/cosmos/cosmos-sdk/x/params/types"
	stakingkeeper "github.com/cosmos/cosmos-sdk/x/staking/keeper"

	"github.com/celestiaorg/celestia-app/v4/x/lst/types"
)

// Keeper handles all the state changes for the lst module.
type Keeper struct {
	cdc            codec.Codec
	storeKey       storetypes.StoreKey
	legacySubspace paramtypes.Subspace
	authority      string

	// Keepers for other modules
	accountKeeper authkeeper.AccountKeeper
	bankKeeper    bankkeeper.Keeper
	stakingKeeper *stakingkeeper.Keeper
}

func NewKeeper(
	cdc codec.Codec,
	storeKey storetypes.StoreKey,
	legacySubspace paramtypes.Subspace,
	authority string,
	accountKeeper authkeeper.AccountKeeper,
	bankKeeper bankkeeper.Keeper,
	stakingKeeper *stakingkeeper.Keeper,
) *Keeper {
	if !legacySubspace.HasKeyTable() {
		legacySubspace = legacySubspace.WithKeyTable(types.ParamKeyTable())
	}

	return &Keeper{
		cdc:            cdc,
		storeKey:       storeKey,
		legacySubspace: legacySubspace,
		authority:      authority,
		accountKeeper:  accountKeeper,
		bankKeeper:     bankKeeper,
		stakingKeeper:  stakingKeeper,
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

// Store returns the module's KVStore
func (k Keeper) Store(ctx context.Context) storetypes.KVStore {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	return sdkCtx.KVStore(k.storeKey)
}

// BASKET MANAGEMENT METHODS

// GetNextBasketID returns the next basket ID and increments the counter
func (k Keeper) GetNextBasketID(ctx context.Context) uint64 {
	store := k.Store(ctx)

	bz := store.Get(types.NextBasketIDKey)
	if bz == nil {
		// Start from 1 if not set
		nextID := uint64(1)
		k.SetNextBasketID(ctx, nextID+1)
		return nextID
	}

	nextID := binary.BigEndian.Uint64(bz)
	k.SetNextBasketID(ctx, nextID+1)
	return nextID
}

// SetNextBasketID sets the next basket ID
func (k Keeper) SetNextBasketID(ctx context.Context, id uint64) {
	store := k.Store(ctx)
	bz := make([]byte, 8)
	binary.BigEndian.PutUint64(bz, id)
	store.Set(types.NextBasketIDKey, bz)
}

// CreateBasket creates a new basket with the given parameters
func (k Keeper) CreateBasket(
	ctx context.Context,
	creator sdk.AccAddress,
	validators []types.ValidatorWeight,
	metadata types.BasketMetadata,
) (string, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	// Generate unique basket ID
	basketID := strconv.FormatUint(k.GetNextBasketID(ctx), 10)

	// Generate denom for this basket
	denom := fmt.Sprintf("bTIA-%s", basketID)

	// Validate that all validators exist and weights sum correctly
	totalWeight := math.LegacyZeroDec()
	for _, val := range validators {
		if _, err := k.stakingKeeper.GetValidator(sdkCtx, sdk.ValAddress(val.ValidatorAddress)); err != nil {
			return "", fmt.Errorf("validator %s not found: %w", val.ValidatorAddress, err)
		}
		totalWeight = totalWeight.Add(val.Weight)
	}

	// Weights should sum to 1.0
	if !totalWeight.Equal(math.LegacyOneDec()) {
		return "", fmt.Errorf("validator weights must sum to 1.0, got %s", totalWeight.String())
	}

	// Create basket
	basket := types.Basket{
		Id:                basketID,
		Denom:             denom,
		Validators:        validators,
		TotalShares:       math.LegacyZeroDec(),
		TotalStakedTokens: math.ZeroInt(),
		Creator:           creator.String(),
		CreationTime:      sdkCtx.BlockTime().Unix(),
		Metadata:          &metadata,
	}

	// Store basket
	k.SetBasket(ctx, basket)

	// Create reverse lookup by denom
	k.SetBasketByDenom(ctx, denom, basketID)

	return basketID, nil
}

// GetBasket retrieves a basket by ID
func (k Keeper) GetBasket(ctx context.Context, basketID string) (types.Basket, bool) {
	store := k.Store(ctx)
	bz := store.Get(types.BasketStoreKey(basketID))
	if bz == nil {
		return types.Basket{}, false
	}

	var basket types.Basket
	k.cdc.MustUnmarshal(bz, &basket)
	return basket, true
}

// SetBasket stores a basket
func (k Keeper) SetBasket(ctx context.Context, basket types.Basket) {
	store := k.Store(ctx)
	bz := k.cdc.MustMarshal(&basket)
	store.Set(types.BasketStoreKey(basket.Id), bz)
}

// GetBasketByDenom retrieves a basket by its denom
func (k Keeper) GetBasketByDenom(ctx context.Context, denom string) (types.Basket, bool) {
	basketID := k.GetBasketIDByDenom(ctx, denom)
	if basketID == "" {
		return types.Basket{}, false
	}
	return k.GetBasket(ctx, basketID)
}

// GetBasketIDByDenom retrieves a basket ID by denom
func (k Keeper) GetBasketIDByDenom(ctx context.Context, denom string) string {
	store := k.Store(ctx)
	bz := store.Get(types.BasketByDenomStoreKey(denom))
	if bz == nil {
		return ""
	}
	return string(bz)
}

// SetBasketByDenom stores the basket ID for a given denom
func (k Keeper) SetBasketByDenom(ctx context.Context, denom, basketID string) {
	store := k.Store(ctx)
	store.Set(types.BasketByDenomStoreKey(denom), []byte(basketID))
}

// GetAllBaskets returns all baskets
func (k Keeper) GetAllBaskets(ctx context.Context) []types.Basket {
	store := k.Store(ctx)
	iterator := storetypes.KVStorePrefixIterator(store, types.BasketKey)
	defer iterator.Close()

	var baskets []types.Basket
	for ; iterator.Valid(); iterator.Next() {
		var basket types.Basket
		k.cdc.MustUnmarshal(iterator.Value(), &basket)
		baskets = append(baskets, basket)
	}

	return baskets
}

// PENDING OPERATIONS MANAGEMENT

// GetNextPendingID returns the next pending operation ID and increments the counter
func (k Keeper) GetNextPendingID(ctx context.Context) uint64 {
	store := k.Store(ctx)

	bz := store.Get(types.NextPendingIDKey)
	if bz == nil {
		// Start from 1 if not set
		nextID := uint64(1)
		k.SetNextPendingID(ctx, nextID+1)
		return nextID
	}

	nextID := binary.BigEndian.Uint64(bz)
	k.SetNextPendingID(ctx, nextID+1)
	return nextID
}

// SetNextPendingID sets the next pending operation ID
func (k Keeper) SetNextPendingID(ctx context.Context, id uint64) {
	store := k.Store(ctx)
	bz := make([]byte, 8)
	binary.BigEndian.PutUint64(bz, id)
	store.Set(types.NextPendingIDKey, bz)
}

// GetBasketModuleAccount returns the module account for a specific basket
func (k Keeper) GetBasketModuleAccount(ctx context.Context, basketID string) sdk.AccAddress {
	accountName := types.GetBasketModuleAccountName(basketID)
	return k.accountKeeper.GetModuleAddress(accountName)
}
