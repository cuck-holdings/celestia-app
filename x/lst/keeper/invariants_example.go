package keeper

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// Example functions demonstrating how to use the invariant system

// ExampleRunAllInvariants demonstrates how to check all invariants
func ExampleRunAllInvariants(ctx sdk.Context, k Keeper) {
	fmt.Println("Running all LST module invariants...")
	
	results := k.CheckAllInvariants(ctx)
	
	allHealthy := true
	for _, result := range results {
		if result.Broken {
			allHealthy = false
			fmt.Printf("❌ INVARIANT BROKEN: %s\n", result.Name)
			fmt.Printf("   Message: %s\n", result.Msg)
		} else {
			fmt.Printf("✅ INVARIANT OK: %s\n", result.Name)
		}
	}
	
	if allHealthy {
		fmt.Println("🎉 All invariants passed!")
	} else {
		fmt.Println("⚠️  Some invariants failed - investigation required")
	}
}

// ExampleTestInvariantBreaking demonstrates how to test invariant detection
func ExampleTestInvariantBreaking(ctx sdk.Context, k Keeper, basketID string) {
	fmt.Printf("Testing invariant detection for basket %s...\n", basketID)
	
	// Get all test scenarios
	scenarios := GetInvariantTestScenarios()
	
	for _, scenario := range scenarios {
		fmt.Printf("\nTesting scenario: %s\n", scenario.Name)
		fmt.Printf("Description: %s\n", scenario.Description)
		
		// Run the test scenario
		result := k.RunInvariantTestScenario(ctx, scenario, basketID)
		
		if result.CorruptionError != nil {
			fmt.Printf("❌ Failed to apply corruption: %s\n", result.CorruptionError)
			continue
		}
		
		if !result.HealthyBefore {
			fmt.Printf("⚠️  System was not healthy before test\n")
			continue
		}
		
		// Check if expected invariants were broken
		expectedBroken := make(map[string]bool)
		for _, name := range result.ExpectedBroken {
			expectedBroken[name] = true
		}
		
		actualBroken := make(map[string]bool)
		for _, name := range result.ActualBroken {
			actualBroken[name] = true
		}
		
		// Verify test worked as expected
		testPassed := true
		for expectedName := range expectedBroken {
			if !actualBroken[expectedName] {
				fmt.Printf("❌ Expected invariant %s to be broken, but it wasn't\n", expectedName)
				testPassed = false
			}
		}
		
		if testPassed && len(result.ActualBroken) > 0 {
			fmt.Printf("✅ Test passed - detected %d broken invariants as expected\n", len(result.ActualBroken))
		} else if len(result.ActualBroken) == 0 {
			fmt.Printf("❌ Test failed - no invariants were broken\n")
		}
	}
}

// ExampleMonitorBasketHealth demonstrates ongoing health monitoring
func ExampleMonitorBasketHealth(ctx sdk.Context, k Keeper, basketID string) {
	fmt.Printf("Monitoring health of basket %s...\n", basketID)
	
	// Check basket-specific metrics
	basket, found := k.GetBasket(ctx, basketID)
	if !found {
		fmt.Printf("❌ Basket %s not found\n", basketID)
		return
	}
	
	fmt.Printf("📊 Basket Stats:\n")
	fmt.Printf("   Total Shares: %s\n", basket.TotalShares)
	fmt.Printf("   Total Staked: %s\n", basket.TotalStakedTokens)
	fmt.Printf("   Validators: %d\n", len(basket.Validators))
	
	// Calculate exchange rate
	if basket.TotalShares.IsPositive() {
		exchangeRate := basket.TotalStakedTokens.ToLegacyDec().Quo(basket.TotalShares)
		fmt.Printf("   Exchange Rate: %s TIA per basket token\n", exchangeRate)
	}
	
	// Check unbonding capacity
	hasCapacity, entries, err := k.CheckUnbondingCapacity(ctx, basketID)
	if err != nil {
		fmt.Printf("❌ Failed to check unbonding capacity: %s\n", err)
	} else {
		fmt.Printf("📈 Unbonding Capacity: ")
		if hasCapacity {
			fmt.Printf("✅ Good\n")
		} else {
			fmt.Printf("⚠️  Near limit\n")
		}
		
		for validator, entryCount := range entries {
			if entryCount > 0 {
				fmt.Printf("   %s: %d entries\n", validator, entryCount)
			}
		}
	}
	
	// Check pending redemptions
	pendingRedemptions := k.GetPendingRedemptionsByBasket(ctx, basketID)
	fmt.Printf("⏳ Pending Redemptions: %d\n", len(pendingRedemptions))
	
	if len(pendingRedemptions) > 0 {
		currentTime := ctx.BlockTime()
		matureCount := 0
		for _, pending := range pendingRedemptions {
			if currentTime.After(pending.CompletionTime) || currentTime.Equal(pending.CompletionTime) {
				matureCount++
			}
		}
		fmt.Printf("   Mature (ready for payout): %d\n", matureCount)
	}
	
	// Run basket-specific invariants
	fmt.Printf("🔍 Running invariant checks...\n")
	results := k.CheckAllInvariants(ctx)
	
	brokenCount := 0
	for _, result := range results {
		if result.Broken {
			brokenCount++
			fmt.Printf("   ❌ %s: %s\n", result.Name, result.Msg)
		}
	}
	
	if brokenCount == 0 {
		fmt.Printf("   ✅ All invariants OK\n")
	} else {
		fmt.Printf("   ⚠️  %d invariants broken\n", brokenCount)
	}
}

// ExampleCrisisModuleIntegration shows how invariants work with crisis module
func ExampleCrisisModuleIntegration() {
	fmt.Println("LST Invariants Crisis Module Integration:")
	fmt.Println("")
	fmt.Println("The LST module registers 4 invariants with the crisis module:")
	fmt.Println("1. basket-accounting  - Verifies basket token backing")
	fmt.Println("2. module-accounts    - Checks module account balances")
	fmt.Println("3. pending-redemptions - Validates redemption records")
	fmt.Println("4. basket-state       - Ensures basket consistency")
	fmt.Println("")
	fmt.Println("Usage with CLI:")
	fmt.Println("  celestia-appd tx crisis invariant-check lst basket-accounting --from validator")
	fmt.Println("  celestia-appd tx crisis invariant-check lst module-accounts --from validator")
	fmt.Println("  celestia-appd tx crisis invariant-check lst pending-redemptions --from validator")
	fmt.Println("  celestia-appd tx crisis invariant-check lst basket-state --from validator")
	fmt.Println("")
	fmt.Println("Configuration:")
	fmt.Println("- Set ConstantFee in crisis module params (e.g., 1000000utia)")
	fmt.Println("- Configure InvariantCheckPeriod for automatic checking")
	fmt.Println("- Enable crisis module in app.go module manager")
	fmt.Println("")
	fmt.Println("Response to violations:")
	fmt.Println("- Crisis module can halt chain (if configured)")
	fmt.Println("- Operators receive immediate notification")
	fmt.Println("- Investigation and resolution procedures activate")
}