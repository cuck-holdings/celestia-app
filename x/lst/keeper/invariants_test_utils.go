package keeper

import (
	"time"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/celestiaorg/celestia-app/v4/x/lst/types"
)

// InvariantTestUtils provides utilities for testing invariants by deliberately
// breaking state to verify that invariants catch the violations.

// BreakBasketAccounting deliberately corrupts basket accounting to test invariants
func (k Keeper) BreakBasketAccounting(ctx sdk.Context, basketID string, corruptionType string) error {
	basket, found := k.GetBasket(ctx, basketID)
	if !found {
		return types.ErrBasketNotFound
	}

	switch corruptionType {
	case "inflate_total_staked":
		// Artificially inflate the total staked amount
		basket.TotalStakedTokens = basket.TotalStakedTokens.Add(math.NewInt(1000000))
		k.SetBasket(ctx, basket)

	case "deflate_total_staked":
		// Artificially deflate the total staked amount
		if basket.TotalStakedTokens.GT(math.NewInt(1000000)) {
			basket.TotalStakedTokens = basket.TotalStakedTokens.Sub(math.NewInt(1000000))
		} else {
			basket.TotalStakedTokens = math.ZeroInt()
		}
		k.SetBasket(ctx, basket)

	case "negative_shares":
		// Set negative total shares
		basket.TotalShares = math.LegacyNewDec(-1000)
		k.SetBasket(ctx, basket)

	case "negative_staked":
		// Set negative total staked tokens
		basket.TotalStakedTokens = math.NewInt(-1000000)
		k.SetBasket(ctx, basket)

	case "unreasonable_exchange_rate":
		// Create an unreasonable exchange rate
		basket.TotalShares = math.LegacyNewDec(1000000)      // 1M shares
		basket.TotalStakedTokens = math.NewInt(100)           // 100 utia
		k.SetBasket(ctx, basket)

	default:
		return types.ErrInvalidAmount.Wrapf("unknown corruption type: %s", corruptionType)
	}

	return nil
}

// BreakModuleAccounts corrupts module account state for testing
func (k Keeper) BreakModuleAccounts(ctx sdk.Context, basketID string, corruptionType string) error {
	basketAccountAddr := types.GetBasketAccountAddress(basketID)
	stakingDenom, err := k.stakingKeeper.BondDenom(ctx)
	if err != nil {
		return err
	}

	switch corruptionType {
	case "excess_balance":
		// Add excessive balance to module account
		excessCoins := sdk.NewCoins(sdk.NewCoin(stakingDenom, math.NewInt(10000000))) // 10 TIA
		err := k.bankKeeper.MintCoins(ctx, types.ModuleName, excessCoins)
		if err != nil {
			return err
		}
		err = k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, basketAccountAddr, excessCoins)
		if err != nil {
			return err
		}

	case "main_module_excess":
		// Add excessive balance to main module account
		excessCoins := sdk.NewCoins(sdk.NewCoin(stakingDenom, math.NewInt(2000000))) // 2 TIA
		err := k.bankKeeper.MintCoins(ctx, types.ModuleName, excessCoins)
		if err != nil {
			return err
		}
		// Coins are already in main module account after minting

	default:
		return types.ErrInvalidAmount.Wrapf("unknown corruption type: %s", corruptionType)
	}

	return nil
}

// BreakPendingRedemptions corrupts pending redemption state for testing
func (k Keeper) BreakPendingRedemptions(ctx sdk.Context, basketID string, corruptionType string) error {
	switch corruptionType {
	case "invalid_basket_ref":
		// Create a pending redemption referencing a non-existent basket
		invalidRedemption := types.PendingRedemption{
			Id:              k.GetNextPendingID(ctx),
			BasketId:        "non-existent-basket",
			Delegator:       "celestia1example",
			SharesBurned:    math.LegacyNewDec(1000),
			TokensToReceive: math.NewInt(1000000),
			CompletionTime:  ctx.BlockTime().Add(24 * time.Hour),
			CreationTime:    ctx.BlockTime(),
		}
		k.SetPendingRedemption(ctx, invalidRedemption)

	case "negative_shares":
		// Create a pending redemption with negative shares
		invalidRedemption := types.PendingRedemption{
			Id:              k.GetNextPendingID(ctx),
			BasketId:        basketID,
			Delegator:       "celestia1example",
			SharesBurned:    math.LegacyNewDec(-1000),
			TokensToReceive: math.NewInt(1000000),
			CompletionTime:  ctx.BlockTime().Add(24 * time.Hour),
			CreationTime:    ctx.BlockTime(),
		}
		k.SetPendingRedemption(ctx, invalidRedemption)

	case "excessive_shares":
		// Create a pending redemption with more shares than basket total
		basket, found := k.GetBasket(ctx, basketID)
		if !found {
			return types.ErrBasketNotFound
		}
		
		invalidRedemption := types.PendingRedemption{
			Id:              k.GetNextPendingID(ctx),
			BasketId:        basketID,
			Delegator:       "celestia1example",
			SharesBurned:    basket.TotalShares.Add(math.LegacyNewDec(1000000)), // More than basket total
			TokensToReceive: math.NewInt(1000000),
			CompletionTime:  ctx.BlockTime().Add(24 * time.Hour),
			CreationTime:    ctx.BlockTime(),
		}
		k.SetPendingRedemption(ctx, invalidRedemption)

	case "invalid_delegator":
		// Create a pending redemption with invalid delegator address
		invalidRedemption := types.PendingRedemption{
			Id:              k.GetNextPendingID(ctx),
			BasketId:        basketID,
			Delegator:       "invalid-address",
			SharesBurned:    math.LegacyNewDec(1000),
			TokensToReceive: math.NewInt(1000000),
			CompletionTime:  ctx.BlockTime().Add(24 * time.Hour),
			CreationTime:    ctx.BlockTime(),
		}
		k.SetPendingRedemption(ctx, invalidRedemption)

	case "very_old_completion":
		// Create a pending redemption with very old completion time
		invalidRedemption := types.PendingRedemption{
			Id:              k.GetNextPendingID(ctx),
			BasketId:        basketID,
			Delegator:       "celestia1example",
			SharesBurned:    math.LegacyNewDec(1000),
			TokensToReceive: math.NewInt(1000000),
			CompletionTime:  ctx.BlockTime().Add(-24 * 7 * 2 * time.Hour), // 2 weeks ago
			CreationTime:    ctx.BlockTime().Add(-24 * 7 * 2 * time.Hour),
		}
		k.SetPendingRedemption(ctx, invalidRedemption)

	default:
		return types.ErrInvalidAmount.Wrapf("unknown corruption type: %s", corruptionType)
	}

	return nil
}

// BreakBasketState corrupts basket state for testing
func (k Keeper) BreakBasketState(ctx sdk.Context, basketID string, corruptionType string) error {
	basket, found := k.GetBasket(ctx, basketID)
	if !found {
		return types.ErrBasketNotFound
	}

	switch corruptionType {
	case "duplicate_validators":
		// Add duplicate validator to basket
		if len(basket.Validators) > 0 {
			duplicateValidator := basket.Validators[0]
			basket.Validators = append(basket.Validators, duplicateValidator)
			k.SetBasket(ctx, basket)
		}

	case "invalid_weights_sum":
		// Make weights not sum to 1.0
		if len(basket.Validators) > 0 {
			basket.Validators[0].Weight = basket.Validators[0].Weight.Add(math.LegacyNewDecWithPrec(1, 1)) // Add 0.1
			k.SetBasket(ctx, basket)
		}

	case "negative_weight":
		// Set negative weight for a validator
		if len(basket.Validators) > 0 {
			basket.Validators[0].Weight = math.LegacyNewDec(-1)
			k.SetBasket(ctx, basket)
		}

	case "invalid_validator_address":
		// Set invalid validator address
		if len(basket.Validators) > 0 {
			basket.Validators[0].ValidatorAddress = "invalid-validator-address"
			k.SetBasket(ctx, basket)
		}

	case "wrong_denom":
		// Set wrong denom for basket
		basket.Denom = "wrong-denom"
		k.SetBasket(ctx, basket)

	case "no_validators":
		// Remove all validators from basket
		basket.Validators = []types.ValidatorWeight{}
		k.SetBasket(ctx, basket)

	case "invalid_creator":
		// Set invalid creator address
		basket.Creator = "invalid-creator-address"
		k.SetBasket(ctx, basket)

	default:
		return types.ErrInvalidAmount.Wrapf("unknown corruption type: %s", corruptionType)
	}

	return nil
}

// CreateDuplicateBasket creates a basket with duplicate ID or denom for testing
func (k Keeper) CreateDuplicateBasket(ctx sdk.Context, existingBasketID string, duplicationType string) error {
	existingBasket, found := k.GetBasket(ctx, existingBasketID)
	if !found {
		return types.ErrBasketNotFound
	}

	switch duplicationType {
	case "duplicate_id":
		// Create another basket with the same ID
		duplicateBasket := existingBasket
		duplicateBasket.Denom = "different-denom" // Change denom to avoid denom conflict
		k.SetBasket(ctx, duplicateBasket)

	case "duplicate_denom":
		// Create another basket with the same denom
		duplicateBasket := existingBasket
		duplicateBasket.Id = "different-id" // Change ID to avoid ID conflict
		k.SetBasket(ctx, duplicateBasket)

	default:
		return types.ErrInvalidAmount.Wrapf("unknown duplication type: %s", duplicationType)
	}

	return nil
}

// InvariantTestScenario represents a test scenario for invariant checking
type InvariantTestScenario struct {
	Name           string
	Description    string
	CorruptionFunc func(ctx sdk.Context, k Keeper, basketID string) error
	ExpectedBroken []string // List of invariant names that should be broken
}

// GetInvariantTestScenarios returns a list of test scenarios for invariant checking
func GetInvariantTestScenarios() []InvariantTestScenario {
	return []InvariantTestScenario{
		{
			Name:        "basket-accounting-inflation",
			Description: "Artificially inflate basket total staked tokens",
			CorruptionFunc: func(ctx sdk.Context, k Keeper, basketID string) error {
				return k.BreakBasketAccounting(ctx, basketID, "inflate_total_staked")
			},
			ExpectedBroken: []string{"basket-accounting"},
		},
		{
			Name:        "basket-accounting-deflation",
			Description: "Artificially deflate basket total staked tokens",
			CorruptionFunc: func(ctx sdk.Context, k Keeper, basketID string) error {
				return k.BreakBasketAccounting(ctx, basketID, "deflate_total_staked")
			},
			ExpectedBroken: []string{"basket-accounting"},
		},
		{
			Name:        "negative-shares",
			Description: "Set negative total shares for basket",
			CorruptionFunc: func(ctx sdk.Context, k Keeper, basketID string) error {
				return k.BreakBasketAccounting(ctx, basketID, "negative_shares")
			},
			ExpectedBroken: []string{"basket-accounting"},
		},
		{
			Name:        "unreasonable-exchange-rate",
			Description: "Create unreasonable exchange rate",
			CorruptionFunc: func(ctx sdk.Context, k Keeper, basketID string) error {
				return k.BreakBasketAccounting(ctx, basketID, "unreasonable_exchange_rate")
			},
			ExpectedBroken: []string{"basket-accounting"},
		},
		{
			Name:        "module-account-excess",
			Description: "Add excessive balance to basket module account",
			CorruptionFunc: func(ctx sdk.Context, k Keeper, basketID string) error {
				return k.BreakModuleAccounts(ctx, basketID, "excess_balance")
			},
			ExpectedBroken: []string{"module-accounts"},
		},
		{
			Name:        "invalid-basket-reference",
			Description: "Create pending redemption with invalid basket reference",
			CorruptionFunc: func(ctx sdk.Context, k Keeper, basketID string) error {
				return k.BreakPendingRedemptions(ctx, basketID, "invalid_basket_ref")
			},
			ExpectedBroken: []string{"pending-redemptions"},
		},
		{
			Name:        "excessive-shares-redemption",
			Description: "Create pending redemption with more shares than basket total",
			CorruptionFunc: func(ctx sdk.Context, k Keeper, basketID string) error {
				return k.BreakPendingRedemptions(ctx, basketID, "excessive_shares")
			},
			ExpectedBroken: []string{"pending-redemptions"},
		},
		{
			Name:        "duplicate-validators",
			Description: "Add duplicate validator to basket",
			CorruptionFunc: func(ctx sdk.Context, k Keeper, basketID string) error {
				return k.BreakBasketState(ctx, basketID, "duplicate_validators")
			},
			ExpectedBroken: []string{"basket-state"},
		},
		{
			Name:        "invalid-weights-sum",
			Description: "Make validator weights not sum to 1.0",
			CorruptionFunc: func(ctx sdk.Context, k Keeper, basketID string) error {
				return k.BreakBasketState(ctx, basketID, "invalid_weights_sum")
			},
			ExpectedBroken: []string{"basket-state"},
		},
		{
			Name:        "duplicate-basket-id",
			Description: "Create basket with duplicate ID",
			CorruptionFunc: func(ctx sdk.Context, k Keeper, basketID string) error {
				return k.CreateDuplicateBasket(ctx, basketID, "duplicate_id")
			},
			ExpectedBroken: []string{"basket-state"},
		},
	}
}

// RunInvariantTestScenario runs a specific test scenario and returns results
func (k Keeper) RunInvariantTestScenario(ctx sdk.Context, scenario InvariantTestScenario, basketID string) InvariantTestResult {
	// Take snapshot of current invariant status
	beforeResults := k.CheckAllInvariants(ctx)
	allHealthyBefore := true
	for _, result := range beforeResults {
		if result.Broken {
			allHealthyBefore = false
			break
		}
	}

	// Apply corruption
	corruptionErr := scenario.CorruptionFunc(ctx, k, basketID)

	// Check invariants after corruption
	afterResults := k.CheckAllInvariants(ctx)

	// Analyze results
	actualBroken := []string{}
	for _, result := range afterResults {
		if result.Broken {
			actualBroken = append(actualBroken, result.Name)
		}
	}

	return InvariantTestResult{
		ScenarioName:       scenario.Name,
		HealthyBefore:      allHealthyBefore,
		CorruptionError:    corruptionErr,
		ExpectedBroken:     scenario.ExpectedBroken,
		ActualBroken:       actualBroken,
		InvariantResults:   afterResults,
	}
}

// InvariantTestResult represents the result of running an invariant test scenario
type InvariantTestResult struct {
	ScenarioName     string            `json:"scenario_name"`
	HealthyBefore    bool              `json:"healthy_before"`
	CorruptionError  error             `json:"corruption_error,omitempty"`
	ExpectedBroken   []string          `json:"expected_broken"`
	ActualBroken     []string          `json:"actual_broken"`
	InvariantResults []InvariantResult `json:"invariant_results"`
}