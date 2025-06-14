package keeper

import (
	"context"
	"time"

	"cosmossdk.io/math"
	storetypes "cosmossdk.io/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/celestiaorg/celestia-app/v4/x/lst/types"
)

// PENDING REDEMPTION OPERATIONS

// SetPendingRedemption stores a pending redemption operation
func (k Keeper) SetPendingRedemption(ctx context.Context, redemption types.PendingRedemption) {
	store := k.Store(ctx)
	bz := k.cdc.MustMarshal(&redemption)
	store.Set(types.PendingRedemptionStoreKey(redemption.Id), bz)

	// Create indexes
	store.Set(types.RedemptionByUserStoreKey(redemption.Delegator, redemption.Id), []byte{})
	store.Set(types.RedemptionByBasketStoreKey(redemption.BasketId, redemption.Id), []byte{})
}

// GetPendingRedemption retrieves a pending redemption by ID
func (k Keeper) GetPendingRedemption(ctx context.Context, id uint64) (types.PendingRedemption, bool) {
	store := k.Store(ctx)
	bz := store.Get(types.PendingRedemptionStoreKey(id))
	if bz == nil {
		return types.PendingRedemption{}, false
	}

	var redemption types.PendingRedemption
	k.cdc.MustUnmarshal(bz, &redemption)
	return redemption, true
}

// DeletePendingRedemption removes a pending redemption and its indexes
func (k Keeper) DeletePendingRedemption(ctx context.Context, redemption types.PendingRedemption) {
	store := k.Store(ctx)

	// Remove main record
	store.Delete(types.PendingRedemptionStoreKey(redemption.Id))

	// Remove indexes
	store.Delete(types.RedemptionByUserStoreKey(redemption.Delegator, redemption.Id))
	store.Delete(types.RedemptionByBasketStoreKey(redemption.BasketId, redemption.Id))
}

// GetAllPendingRedemptions returns all pending redemptions
func (k Keeper) GetAllPendingRedemptions(ctx context.Context) []types.PendingRedemption {
	store := k.Store(ctx)
	iterator := storetypes.KVStorePrefixIterator(store, types.PendingRedemptionKey)
	defer iterator.Close()

	var redemptions []types.PendingRedemption
	for ; iterator.Valid(); iterator.Next() {
		var redemption types.PendingRedemption
		k.cdc.MustUnmarshal(iterator.Value(), &redemption)
		redemptions = append(redemptions, redemption)
	}

	return redemptions
}

// GetPendingRedemptionsByUser returns all pending redemptions for a user
func (k Keeper) GetPendingRedemptionsByUser(ctx context.Context, userAddr string) []types.PendingRedemption {
	store := k.Store(ctx)
	prefix := append(types.RedemptionByUserKey, []byte(userAddr)...)
	prefix = append(prefix, []byte("/")...)
	iterator := storetypes.KVStorePrefixIterator(store, prefix)
	defer iterator.Close()

	var redemptions []types.PendingRedemption
	for ; iterator.Valid(); iterator.Next() {
		// Extract ID from key and get the actual redemption
		key := iterator.Key()
		idBytes := key[len(prefix):]
		if len(idBytes) == 8 {
			id := k.extractIDFromBytes(idBytes)
			if redemption, found := k.GetPendingRedemption(ctx, id); found {
				redemptions = append(redemptions, redemption)
			}
		}
	}

	return redemptions
}

// GetPendingRedemptionsByBasket returns all pending redemptions for a basket
func (k Keeper) GetPendingRedemptionsByBasket(ctx context.Context, basketID string) []types.PendingRedemption {
	store := k.Store(ctx)
	prefix := append(types.RedemptionByBasketKey, []byte(basketID)...)
	prefix = append(prefix, []byte("/")...)
	iterator := storetypes.KVStorePrefixIterator(store, prefix)
	defer iterator.Close()

	var redemptions []types.PendingRedemption
	for ; iterator.Valid(); iterator.Next() {
		// Extract ID from key and get the actual redemption
		key := iterator.Key()
		idBytes := key[len(prefix):]
		if len(idBytes) == 8 {
			id := k.extractIDFromBytes(idBytes)
			if redemption, found := k.GetPendingRedemption(ctx, id); found {
				redemptions = append(redemptions, redemption)
			}
		}
	}

	return redemptions
}

// BASKET CONVERSION OPERATIONS (using instant redelegation)

// ConvertBasketToBasket converts shares from one basket to another using redelegation
func (k Keeper) ConvertBasketToBasket(
	ctx context.Context,
	delegator sdk.AccAddress,
	fromBasketID string,
	toBasketID string,
	sharesToConvert math.LegacyDec,
) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	// Get both baskets
	fromBasket, found := k.GetBasket(ctx, fromBasketID)
	if !found {
		return types.ErrBasketNotFound
	}

	toBasket, found := k.GetBasket(ctx, toBasketID)
	if !found {
		return types.ErrBasketNotFound
	}

	// Get basket module accounts
	fromBasketAcc := k.GetBasketModuleAccount(ctx, fromBasketID)
	toBasketAcc := k.GetBasketModuleAccount(ctx, toBasketID)

	// Calculate the amount of TIA tokens represented by the shares
	exchangeRate := k.GetBasketExchangeRate(ctx, fromBasket)
	tokensToTransfer := sharesToConvert.Mul(exchangeRate).TruncateInt()

	// Burn the user's basket shares from the source basket
	err := k.bankKeeper.BurnCoins(sdkCtx, types.ModuleName, sdk.NewCoins(sdk.NewCoin(fromBasket.Denom, sharesToConvert.TruncateInt())))
	if err != nil {
		return err
	}

	// Use redelegation to move stake from source basket validators to destination basket validators
	err = k.redelegateProportionally(ctx, fromBasketAcc, toBasketAcc, fromBasket, toBasket, tokensToTransfer)
	if err != nil {
		return err
	}

	// Update basket states
	fromBasket.TotalShares = fromBasket.TotalShares.Sub(sharesToConvert)
	fromBasket.TotalStakedTokens = fromBasket.TotalStakedTokens.Sub(tokensToTransfer)
	k.SetBasket(ctx, fromBasket)

	// Calculate new shares to mint for destination basket
	toExchangeRate := k.GetBasketExchangeRate(ctx, toBasket)
	newShares := math.LegacyNewDecFromInt(tokensToTransfer).Quo(toExchangeRate)

	// Mint new shares in destination basket
	newCoins := sdk.NewCoins(sdk.NewCoin(toBasket.Denom, newShares.TruncateInt()))
	err = k.bankKeeper.MintCoins(sdkCtx, types.ModuleName, newCoins)
	if err != nil {
		return err
	}

	err = k.bankKeeper.SendCoinsFromModuleToAccount(sdkCtx, types.ModuleName, delegator, newCoins)
	if err != nil {
		return err
	}

	// Update destination basket state
	toBasket.TotalShares = toBasket.TotalShares.Add(newShares)
	toBasket.TotalStakedTokens = toBasket.TotalStakedTokens.Add(tokensToTransfer)
	k.SetBasket(ctx, toBasket)

	return nil
}

// ConvertDelegationToBasket converts a user's direct delegation to a basket using redelegation
func (k Keeper) ConvertDelegationToBasket(
	ctx context.Context,
	delegator sdk.AccAddress,
	validatorAddr sdk.ValAddress,
	amount math.Int,
	basketID string,
) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	// Get the target basket
	basket, found := k.GetBasket(ctx, basketID)
	if !found {
		return types.ErrBasketNotFound
	}

	basketAcc := k.GetBasketModuleAccount(ctx, basketID)

	// Redelegate from user's validator to basket validators proportionally
	err := k.redelegateToBasket(ctx, delegator, validatorAddr, basketAcc, basket, amount)
	if err != nil {
		return err
	}

	// Calculate shares to mint based on current exchange rate
	exchangeRate := k.GetBasketExchangeRate(ctx, basket)
	sharesToMint := math.LegacyNewDecFromInt(amount).Quo(exchangeRate)

	// Mint basket shares for the user
	newCoins := sdk.NewCoins(sdk.NewCoin(basket.Denom, sharesToMint.TruncateInt()))
	err = k.bankKeeper.MintCoins(sdkCtx, types.ModuleName, newCoins)
	if err != nil {
		return err
	}

	err = k.bankKeeper.SendCoinsFromModuleToAccount(sdkCtx, types.ModuleName, delegator, newCoins)
	if err != nil {
		return err
	}

	// Update basket state
	basket.TotalShares = basket.TotalShares.Add(sharesToMint)
	basket.TotalStakedTokens = basket.TotalStakedTokens.Add(amount)
	k.SetBasket(ctx, basket)

	return nil
}

// UTILITY METHODS

// CreatePendingRedemption creates a new pending redemption with auto-generated ID
func (k Keeper) CreatePendingRedemption(
	ctx context.Context,
	basketID string,
	delegator sdk.AccAddress,
	sharesBurned math.LegacyDec,
	tokensToReceive math.Int,
	completionTime time.Time,
) (uint64, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	redemptionID := k.GetNextPendingID(ctx)

	redemption := types.PendingRedemption{
		Id:              redemptionID,
		BasketId:        basketID,
		Delegator:       delegator.String(),
		SharesBurned:    sharesBurned,
		TokensToReceive: tokensToReceive,
		CompletionTime:  completionTime,
		CreationTime:    sdkCtx.BlockTime(),
	}

	k.SetPendingRedemption(ctx, redemption)
	return redemptionID, nil
}

// GetMaturePendingRedemptions returns redemptions that are ready to be completed
func (k Keeper) GetMaturePendingRedemptions(ctx context.Context) []types.PendingRedemption {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	currentTime := sdkCtx.BlockTime()

	var matureRedemptions []types.PendingRedemption
	for _, redemption := range k.GetAllPendingRedemptions(ctx) {
		if redemption.CompletionTime.Before(currentTime) || redemption.CompletionTime.Equal(currentTime) {
			matureRedemptions = append(matureRedemptions, redemption)
		}
	}

	return matureRedemptions
}

// REDELEGATION HELPER METHODS

// redelegateProportionally redelegates tokens from source basket validators to destination basket validators
func (k Keeper) redelegateProportionally(
	ctx context.Context,
	fromBasketAcc sdk.AccAddress,
	toBasketAcc sdk.AccAddress,
	fromBasket types.Basket,
	toBasket types.Basket,
	totalAmount math.Int,
) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	// Calculate proportional amounts to redelegate from each source validator
	for _, fromVal := range fromBasket.Validators {
		fromValAddr, err := sdk.ValAddressFromBech32(fromVal.ValidatorAddress)
		if err != nil {
			return err
		}

		// Amount to redelegate from this validator (proportional to weight)
		amountFromThis := fromVal.Weight.MulInt(totalAmount).TruncateInt()
		if amountFromThis.IsZero() {
			continue
		}

		// Redelegate proportionally to destination validators
		for _, toVal := range toBasket.Validators {
			toValAddr, err := sdk.ValAddressFromBech32(toVal.ValidatorAddress)
			if err != nil {
				return err
			}

			// Amount to redelegate to this destination validator
			amountToThis := toVal.Weight.MulInt(amountFromThis).TruncateInt()
			if amountToThis.IsZero() {
				continue
			}

			// Execute the redelegation using staking keeper
			_, err = k.stakingKeeper.BeginRedelegation(sdkCtx, fromBasketAcc, fromValAddr, toValAddr, math.LegacyNewDecFromInt(amountToThis))
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// redelegateToBasket redelegates from a single validator to basket validators proportionally
func (k Keeper) redelegateToBasket(
	ctx context.Context,
	delegator sdk.AccAddress,
	fromValidator sdk.ValAddress,
	toBasketAcc sdk.AccAddress,
	basket types.Basket,
	amount math.Int,
) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	// Redelegate proportionally to each validator in the basket
	for _, val := range basket.Validators {
		toValAddr, err := sdk.ValAddressFromBech32(val.ValidatorAddress)
		if err != nil {
			return err
		}

		// Calculate proportional amount for this validator
		amountToThis := val.Weight.MulInt(amount).TruncateInt()
		if amountToThis.IsZero() {
			continue
		}

		// Execute redelegation from user to basket account
		_, err = k.stakingKeeper.BeginRedelegation(sdkCtx, delegator, fromValidator, toValAddr, math.LegacyNewDecFromInt(amountToThis))
		if err != nil {
			return err
		}
	}

	return nil
}

// GetBasketExchangeRate calculates the current exchange rate for a basket (TIA per basket token)
func (k Keeper) GetBasketExchangeRate(ctx context.Context, basket types.Basket) math.LegacyDec {
	if basket.TotalShares.IsZero() {
		// If no shares exist, use 1:1 exchange rate
		return math.LegacyOneDec()
	}

	// Exchange rate = TotalStakedTokens / TotalShares
	return math.LegacyNewDecFromInt(basket.TotalStakedTokens).Quo(basket.TotalShares)
}

// extractIDFromBytes extracts uint64 ID from bytes
func (k Keeper) extractIDFromBytes(bz []byte) uint64 {
	if len(bz) != 8 {
		return 0
	}
	return sdk.BigEndianToUint64(bz)
}
