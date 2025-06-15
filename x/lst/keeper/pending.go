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
	fromBasketAccountAddr sdk.AccAddress,
	toBasketAccountAddr sdk.AccAddress,
	sharesToConvert math.Int,
	fromBasketValidators []types.ValidatorWeight,
	toBasketValidators []types.ValidatorWeight,
) (math.Int, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	// Calculate proportional amounts to redelegate from each source validator
	for _, fromVal := range fromBasketValidators {
		fromValAddr, err := sdk.ValAddressFromBech32(fromVal.ValidatorAddress)
		if err != nil {
			return math.ZeroInt(), err
		}

		// Amount to redelegate from this validator (proportional to weight)
		amountFromThis := fromVal.Weight.MulInt(sharesToConvert).TruncateInt()
		if amountFromThis.IsZero() {
			continue
		}

		// Redelegate proportionally to destination validators
		for _, toVal := range toBasketValidators {
			toValAddr, err := sdk.ValAddressFromBech32(toVal.ValidatorAddress)
			if err != nil {
				return math.ZeroInt(), err
			}

			// Amount to redelegate to this destination validator
			amountToThis := toVal.Weight.MulInt(amountFromThis).TruncateInt()
			if amountToThis.IsZero() {
				continue
			}

			// Execute the redelegation using staking keeper
			_, err = k.stakingKeeper.BeginRedelegation(sdkCtx, fromBasketAccountAddr, fromValAddr, toValAddr, math.LegacyNewDecFromInt(amountToThis))
			if err != nil {
				return math.ZeroInt(), err
			}
		}
	}

	// Return the amount converted (for target basket token calculation)
	return sharesToConvert, nil
}

// ConvertDelegationToBasket converts a user's direct delegation to a basket using redelegation
func (k Keeper) ConvertDelegationToBasket(
	ctx context.Context,
	delegator sdk.AccAddress,
	validatorAddr sdk.ValAddress,
	basketAccountAddr sdk.AccAddress,
	amount math.Int,
	basketValidators []types.ValidatorWeight,
) (math.Int, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	// Redelegate from user's validator to basket validators proportionally
	for _, val := range basketValidators {
		toValAddr, err := sdk.ValAddressFromBech32(val.ValidatorAddress)
		if err != nil {
			return math.ZeroInt(), err
		}

		// Calculate proportional amount for this validator
		amountToThis := val.Weight.MulInt(amount).TruncateInt()
		if amountToThis.IsZero() {
			continue
		}

		// Execute redelegation from user directly to basket account for this validator
		_, err = k.stakingKeeper.BeginRedelegation(sdkCtx, delegator, validatorAddr, toValAddr, math.LegacyNewDecFromInt(amountToThis))
		if err != nil {
			return math.ZeroInt(), err
		}
	}

	// Return the amount converted (for basket token calculation)
	return amount, nil
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

// GetBasketExchangeRate calculates the current exchange rate for a basket (TIA per basket token)
func (k Keeper) GetBasketExchangeRate(ctx context.Context, basketID string) (math.LegacyDec, error) {
	basket, found := k.GetBasket(ctx, basketID)
	if !found {
		return math.LegacyZeroDec(), types.ErrBasketNotFound
	}

	if basket.TotalShares.IsZero() {
		// If no shares exist, use 1:1 exchange rate
		return math.LegacyOneDec(), nil
	}

	// Calculate total value of delegations for this basket
	totalValue, err := k.calculateBasketTotalValue(ctx, basket)
	if err != nil {
		return math.LegacyZeroDec(), err
	}

	// Exchange rate = TotalValue / TotalShares
	return math.LegacyNewDecFromInt(totalValue).Quo(basket.TotalShares), nil
}

// calculateBasketTotalValue calculates the total value of all delegations for a basket
func (k Keeper) calculateBasketTotalValue(ctx context.Context, basket types.Basket) (math.Int, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	basketAccountAddr := types.GetBasketAccountAddress(basket.Id)
	totalValue := math.ZeroInt()

	for _, val := range basket.Validators {
		valAddr, err := sdk.ValAddressFromBech32(val.ValidatorAddress)
		if err != nil {
			return math.ZeroInt(), err
		}

		// Get delegation from basket account to this validator
		delegation, err := k.stakingKeeper.GetDelegation(sdkCtx, basketAccountAddr, valAddr)
		if err != nil {
			return totalValue, err
		}

		// Get validator to calculate token value
		validator, err := k.stakingKeeper.GetValidator(sdkCtx, valAddr)
		if err != nil {
			return totalValue, err
		}

		// Calculate token value of the delegation
		tokens := validator.TokensFromShares(delegation.Shares).TruncateInt()
		totalValue = totalValue.Add(tokens)
	}

	return totalValue, nil
}

// extractIDFromBytes extracts uint64 ID from bytes
func (k Keeper) extractIDFromBytes(bz []byte) uint64 {
	if len(bz) != 8 {
		return 0
	}
	return sdk.BigEndianToUint64(bz)
}
