package keeper

import (
	"fmt"
	"time"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/celestiaorg/celestia-app/v4/x/lst/types"
)

// RegisterInvariants registers all LST module invariants with the crisis module
func RegisterInvariants(ir sdk.InvariantRegistry, k Keeper) {
	ir.RegisterRoute(types.ModuleName, "basket-accounting", BasketAccountingInvariant(k))
	ir.RegisterRoute(types.ModuleName, "module-accounts", ModuleAccountsInvariant(k))
	ir.RegisterRoute(types.ModuleName, "pending-redemptions", PendingRedemptionsInvariant(k))
	ir.RegisterRoute(types.ModuleName, "basket-state", BasketStateInvariant(k))
}

// BasketAccountingInvariant checks that basket accounting is consistent:
// For each basket: actual_staked + pending_unbonding + module_balance = total_shares * exchange_rate
func BasketAccountingInvariant(k Keeper) sdk.Invariant {
	return func(ctx sdk.Context) (string, bool) {
		baskets := k.GetAllBaskets(ctx)
		tolerance := math.NewInt(1000) // 1000 utia tolerance for rounding
		
		for _, basket := range baskets {
			// Get basket account address
			basketAccountAddr := types.GetBasketAccountAddress(basket.Id)
			
			// 1. Calculate actual staked amount from delegations
			actualStaked := math.ZeroInt()
			for _, val := range basket.Validators {
				valAddr, err := sdk.ValAddressFromBech32(val.ValidatorAddress)
				if err != nil {
					return sdk.FormatInvariant(
						types.ModuleName, "basket-accounting",
						fmt.Sprintf("invalid validator address in basket %s: %s", basket.Id, val.ValidatorAddress),
					), true
				}
				
				delegation, err := k.stakingKeeper.GetDelegation(ctx, basketAccountAddr, valAddr)
				if err == nil {
					// Get validator to calculate token value from shares
					validator, err := k.stakingKeeper.GetValidator(ctx, valAddr)
					if err == nil {
						tokens := validator.TokensFromShares(delegation.Shares).TruncateInt()
						actualStaked = actualStaked.Add(tokens)
					}
				}
			}
			
			// 2. Calculate pending unbonding amounts
			pendingUnbonding := math.ZeroInt()
			pendingRedemptions := k.GetPendingRedemptionsByBasket(ctx, basket.Id)
			for _, pending := range pendingRedemptions {
				pendingUnbonding = pendingUnbonding.Add(pending.TokensToReceive)
			}
			
			// 3. Get module account liquid balance
			stakingDenom, err := k.stakingKeeper.BondDenom(ctx)
			if err != nil {
				return sdk.FormatInvariant(
					types.ModuleName, "basket-accounting",
					fmt.Sprintf("failed to get bond denom: %s", err.Error()),
				), true
			}
			
			moduleBalance := k.bankKeeper.GetBalance(ctx, basketAccountAddr, stakingDenom).Amount
			
			// 4. Calculate total accounted value
			totalAccountedValue := actualStaked.Add(pendingUnbonding).Add(moduleBalance)
			
			// 5. Calculate expected value from basket state
			expectedValue := basket.TotalStakedTokens
			
			// 6. Check if values match within tolerance
			diff := totalAccountedValue.Sub(expectedValue).Abs()
			if diff.GT(tolerance) {
				return sdk.FormatInvariant(
					types.ModuleName, "basket-accounting",
					fmt.Sprintf(
						"basket %s accounting mismatch: actual_staked=%s + pending_unbonding=%s + module_balance=%s = %s, expected=%s, diff=%s",
						basket.Id, actualStaked, pendingUnbonding, moduleBalance, totalAccountedValue, expectedValue, diff,
					),
				), true
			}
			
			// 7. Additional sanity checks
			if basket.TotalShares.IsNegative() {
				return sdk.FormatInvariant(
					types.ModuleName, "basket-accounting",
					fmt.Sprintf("basket %s has negative total shares: %s", basket.Id, basket.TotalShares),
				), true
			}
			
			if basket.TotalStakedTokens.IsNegative() {
				return sdk.FormatInvariant(
					types.ModuleName, "basket-accounting",
					fmt.Sprintf("basket %s has negative total staked tokens: %s", basket.Id, basket.TotalStakedTokens),
				), true
			}
			
			// 8. Check exchange rate reasonableness (if shares exist)
			if basket.TotalShares.IsPositive() {
				exchangeRate := math.LegacyNewDecFromInt(expectedValue).Quo(basket.TotalShares)
				// Exchange rate should be positive and reasonable (between 0.1 and 10.0)
				minRate := math.LegacyNewDecWithPrec(1, 1) // 0.1
				maxRate := math.LegacyNewDec(10)           // 10.0
				if exchangeRate.LT(minRate) || exchangeRate.GT(maxRate) {
					return sdk.FormatInvariant(
						types.ModuleName, "basket-accounting",
						fmt.Sprintf("basket %s has unreasonable exchange rate: %s", basket.Id, exchangeRate),
					), true
				}
			}
		}
		
		return sdk.FormatInvariant(types.ModuleName, "basket-accounting", "all baskets accounting verified"), false
	}
}

// ModuleAccountsInvariant checks that module account balances are consistent with pending operations
func ModuleAccountsInvariant(k Keeper) sdk.Invariant {
	return func(ctx sdk.Context) (string, bool) {
		baskets := k.GetAllBaskets(ctx)
		stakingDenom, err := k.stakingKeeper.BondDenom(ctx)
		if err != nil {
			return sdk.FormatInvariant(
				types.ModuleName, "module-accounts",
				fmt.Sprintf("failed to get bond denom: %s", err.Error()),
			), true
		}
		
		for _, basket := range baskets {
			basketAccountAddr := types.GetBasketAccountAddress(basket.Id)
			
			// Get current module account balance
			moduleBalance := k.bankKeeper.GetBalance(ctx, basketAccountAddr, stakingDenom).Amount
			
			// Calculate expected balance from pending redemptions that should have completed
			expectedBalance := math.ZeroInt()
			pendingRedemptions := k.GetPendingRedemptionsByBasket(ctx, basket.Id)
			currentTime := ctx.BlockTime()
			
			for _, pending := range pendingRedemptions {
				// If redemption has completed, tokens should be in module account
				if currentTime.After(pending.CompletionTime) || currentTime.Equal(pending.CompletionTime) {
					expectedBalance = expectedBalance.Add(pending.TokensToReceive)
				}
			}
			
			// Module account balance should not exceed what's expected from completed redemptions
			// (it could be less if payouts have been processed but records not yet cleaned up)
			if moduleBalance.GT(expectedBalance) {
				// Allow some tolerance for rewards or dust
				tolerance := math.NewInt(10000) // 0.01 TIA tolerance
				excess := moduleBalance.Sub(expectedBalance)
				if excess.GT(tolerance) {
					return sdk.FormatInvariant(
						types.ModuleName, "module-accounts",
						fmt.Sprintf(
							"basket %s module account has excessive balance: actual=%s, expected_from_completed_redemptions=%s, excess=%s",
							basket.Id, moduleBalance, expectedBalance, excess,
						),
					), true
				}
			}
		}
		
		// Check main module account (should generally be empty except for operational dust)
		mainModuleAddr := k.accountKeeper.GetModuleAddress(types.ModuleName)
		mainModuleBalance := k.bankKeeper.GetBalance(ctx, mainModuleAddr, stakingDenom).Amount
		
		// Main module account should not accumulate large balances
		maxMainBalance := math.NewInt(1000000) // 1 TIA maximum
		if mainModuleBalance.GT(maxMainBalance) {
			return sdk.FormatInvariant(
				types.ModuleName, "module-accounts",
				fmt.Sprintf("main module account has excessive balance: %s", mainModuleBalance),
			), true
		}
		
		return sdk.FormatInvariant(types.ModuleName, "module-accounts", "all module accounts verified"), false
	}
}

// PendingRedemptionsInvariant checks the consistency of pending redemption records
func PendingRedemptionsInvariant(k Keeper) sdk.Invariant {
	return func(ctx sdk.Context) (string, bool) {
		allRedemptions := k.GetAllPendingRedemptions(ctx)
		baskets := k.GetAllBaskets(ctx)
		basketMap := make(map[string]types.Basket)
		
		// Create basket lookup map
		for _, basket := range baskets {
			basketMap[basket.Id] = basket
		}
		
		for _, redemption := range allRedemptions {
			// 1. Check that referenced basket exists
			basket, exists := basketMap[redemption.BasketId]
			if !exists {
				return sdk.FormatInvariant(
					types.ModuleName, "pending-redemptions",
					fmt.Sprintf("pending redemption %d references non-existent basket %s", redemption.Id, redemption.BasketId),
				), true
			}
			
			// 2. Check that redemption amounts are reasonable
			if redemption.SharesBurned.IsNegative() || redemption.SharesBurned.IsZero() {
				return sdk.FormatInvariant(
					types.ModuleName, "pending-redemptions",
					fmt.Sprintf("pending redemption %d has invalid shares burned: %s", redemption.Id, redemption.SharesBurned),
				), true
			}
			
			if redemption.TokensToReceive.IsNegative() || redemption.TokensToReceive.IsZero() {
				return sdk.FormatInvariant(
					types.ModuleName, "pending-redemptions",
					fmt.Sprintf("pending redemption %d has invalid tokens to receive: %s", redemption.Id, redemption.TokensToReceive),
				), true
			}
			
			// 3. Check that shares burned is not more than basket total
			if redemption.SharesBurned.GT(basket.TotalShares) {
				return sdk.FormatInvariant(
					types.ModuleName, "pending-redemptions",
					fmt.Sprintf(
						"pending redemption %d shares burned (%s) exceeds basket %s total shares (%s)",
						redemption.Id, redemption.SharesBurned, redemption.BasketId, basket.TotalShares,
					),
				), true
			}
			
			// 4. Check that delegator address is valid
			if _, err := sdk.AccAddressFromBech32(redemption.Delegator); err != nil {
				return sdk.FormatInvariant(
					types.ModuleName, "pending-redemptions",
					fmt.Sprintf("pending redemption %d has invalid delegator address: %s", redemption.Id, redemption.Delegator),
				), true
			}
			
			// 5. Check that completion time is not in the past by too much
			currentTime := ctx.BlockTime()
			if redemption.CompletionTime.Before(currentTime.Add(-24 * 7 * time.Hour)) { // 1 week tolerance
				return sdk.FormatInvariant(
					types.ModuleName, "pending-redemptions",
					fmt.Sprintf("pending redemption %d has very old completion time: %s", redemption.Id, redemption.CompletionTime),
				), true
			}
		}
		
		return sdk.FormatInvariant(types.ModuleName, "pending-redemptions", "all pending redemptions verified"), false
	}
}

// BasketStateInvariant checks the internal consistency of basket state
func BasketStateInvariant(k Keeper) sdk.Invariant {
	return func(ctx sdk.Context) (string, bool) {
		baskets := k.GetAllBaskets(ctx)
		
		// Track used basket IDs and denoms to check for duplicates
		usedIDs := make(map[string]bool)
		usedDenoms := make(map[string]bool)
		
		for _, basket := range baskets {
			// 1. Check for duplicate basket IDs
			if usedIDs[basket.Id] {
				return sdk.FormatInvariant(
					types.ModuleName, "basket-state",
					fmt.Sprintf("duplicate basket ID found: %s", basket.Id),
				), true
			}
			usedIDs[basket.Id] = true
			
			// 2. Check for duplicate denoms
			if usedDenoms[basket.Denom] {
				return sdk.FormatInvariant(
					types.ModuleName, "basket-state",
					fmt.Sprintf("duplicate basket denom found: %s", basket.Denom),
				), true
			}
			usedDenoms[basket.Denom] = true
			
			// 3. Check basket ID and denom consistency
			expectedDenom := types.GetBasketTokenDenom(basket.Id)
			if basket.Denom != expectedDenom {
				return sdk.FormatInvariant(
					types.ModuleName, "basket-state",
					fmt.Sprintf("basket %s denom mismatch: expected %s, got %s", basket.Id, expectedDenom, basket.Denom),
				), true
			}
			
			// 4. Check validator weights
			totalWeight := math.LegacyZeroDec()
			validatorAddrs := make(map[string]bool)
			
			for i, val := range basket.Validators {
				// Check for duplicate validators in same basket
				if validatorAddrs[val.ValidatorAddress] {
					return sdk.FormatInvariant(
						types.ModuleName, "basket-state",
						fmt.Sprintf("basket %s has duplicate validator: %s", basket.Id, val.ValidatorAddress),
					), true
				}
				validatorAddrs[val.ValidatorAddress] = true
				
				// Check validator address format
				if _, err := sdk.ValAddressFromBech32(val.ValidatorAddress); err != nil {
					return sdk.FormatInvariant(
						types.ModuleName, "basket-state",
						fmt.Sprintf("basket %s validator %d has invalid address: %s", basket.Id, i, val.ValidatorAddress),
					), true
				}
				
				// Check weight is positive
				if val.Weight.IsNegative() || val.Weight.IsZero() {
					return sdk.FormatInvariant(
						types.ModuleName, "basket-state",
						fmt.Sprintf("basket %s validator %d has invalid weight: %s", basket.Id, i, val.Weight),
					), true
				}
				
				totalWeight = totalWeight.Add(val.Weight)
			}
			
			// 5. Check that weights sum to 1.0 (with small tolerance)
			tolerance := math.LegacyNewDecWithPrec(1, 10) // 0.0000000001
			diff := totalWeight.Sub(math.LegacyOneDec()).Abs()
			if diff.GT(tolerance) {
				return sdk.FormatInvariant(
					types.ModuleName, "basket-state",
					fmt.Sprintf("basket %s validator weights don't sum to 1.0: got %s", basket.Id, totalWeight),
				), true
			}
			
			// 6. Check that basket has at least one validator
			if len(basket.Validators) == 0 {
				return sdk.FormatInvariant(
					types.ModuleName, "basket-state",
					fmt.Sprintf("basket %s has no validators", basket.Id),
				), true
			}
			
			// 7. Check creator address format
			if _, err := sdk.AccAddressFromBech32(basket.Creator); err != nil {
				return sdk.FormatInvariant(
					types.ModuleName, "basket-state",
					fmt.Sprintf("basket %s has invalid creator address: %s", basket.Id, basket.Creator),
				), true
			}
			
			// 8. Check metadata if present
			if basket.Metadata != nil {
				if err := types.ValidateBasketMetadata(*basket.Metadata); err != nil {
					return sdk.FormatInvariant(
						types.ModuleName, "basket-state",
						fmt.Sprintf("basket %s has invalid metadata: %s", basket.Id, err.Error()),
					), true
				}
			}
		}
		
		return sdk.FormatInvariant(types.ModuleName, "basket-state", "all basket states verified"), false
	}
}

// Helper function to run all invariants and return detailed results
func (k Keeper) CheckAllInvariants(ctx sdk.Context) []InvariantResult {
	results := []InvariantResult{}
	
	// List of all invariants
	invariants := []struct {
		name string
		fn   sdk.Invariant
	}{
		{"basket-accounting", BasketAccountingInvariant(k)},
		{"module-accounts", ModuleAccountsInvariant(k)},
		{"pending-redemptions", PendingRedemptionsInvariant(k)},
		{"basket-state", BasketStateInvariant(k)},
	}
	
	// Run each invariant
	for _, inv := range invariants {
		msg, broken := inv.fn(ctx)
		results = append(results, InvariantResult{
			Name:   inv.name,
			Broken: broken,
			Msg:    msg,
		})
	}
	
	return results
}

// InvariantResult represents the result of an invariant check
type InvariantResult struct {
	Name   string `json:"name"`
	Broken bool   `json:"broken"`
	Msg    string `json:"message"`
}