package types

import (
	"fmt"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// ValidateBasket validates a basket
func ValidateBasket(basket Basket) error {
	if basket.Id == "" {
		return fmt.Errorf("basket ID cannot be empty")
	}
	
	if basket.Denom == "" {
		return fmt.Errorf("basket denom cannot be empty")
	}
	
	if len(basket.Validators) == 0 {
		return fmt.Errorf("basket must have at least one validator")
	}
	
	// Validate creator address
	if _, err := sdk.AccAddressFromBech32(basket.Creator); err != nil {
		return fmt.Errorf("invalid creator address: %w", err)
	}
	
	// Validate validators and weights
	totalWeight := math.LegacyZeroDec()
	validatorAddrs := make(map[string]bool)
	
	for i, val := range basket.Validators {
		if val.ValidatorAddress == "" {
			return fmt.Errorf("validator %d address cannot be empty", i)
		}
		
		// Check for duplicate validators
		if validatorAddrs[val.ValidatorAddress] {
			return fmt.Errorf("duplicate validator address: %s", val.ValidatorAddress)
		}
		validatorAddrs[val.ValidatorAddress] = true
		
		// Validate validator address format
		if _, err := sdk.ValAddressFromBech32(val.ValidatorAddress); err != nil {
			return fmt.Errorf("invalid validator address %s: %w", val.ValidatorAddress, err)
		}
		
		// Validate weight
		if val.Weight.IsNil() || val.Weight.IsNegative() {
			return fmt.Errorf("validator %s weight must be positive", val.ValidatorAddress)
		}
		
		totalWeight = totalWeight.Add(val.Weight)
	}
	
	// Weights should sum to 1.0
	if !totalWeight.Equal(math.LegacyOneDec()) {
		return fmt.Errorf("validator weights must sum to 1.0, got %s", totalWeight.String())
	}
	
	// Validate total shares (should be non-negative)
	if basket.TotalShares.IsNil() || basket.TotalShares.IsNegative() {
		return fmt.Errorf("total shares must be non-negative")
	}
	
	// Validate total staked tokens (should be non-negative)
	if basket.TotalStakedTokens.IsNil() || basket.TotalStakedTokens.IsNegative() {
		return fmt.Errorf("total staked tokens must be non-negative")
	}
	
	return nil
}

// ValidatePendingRedemption validates a pending redemption
func ValidatePendingRedemption(redemption PendingRedemption) error {
	if redemption.Id == 0 {
		return fmt.Errorf("redemption ID cannot be zero")
	}
	
	if redemption.BasketId == "" {
		return fmt.Errorf("basket ID cannot be empty")
	}
	
	if redemption.Delegator == "" {
		return fmt.Errorf("delegator address cannot be empty")
	}
	
	// Validate delegator address
	if _, err := sdk.AccAddressFromBech32(redemption.Delegator); err != nil {
		return fmt.Errorf("invalid delegator address: %w", err)
	}
	
	// Validate shares burned
	if redemption.SharesBurned.IsNil() || redemption.SharesBurned.IsNegative() || redemption.SharesBurned.IsZero() {
		return fmt.Errorf("shares burned must be positive")
	}
	
	// Validate tokens to receive
	if redemption.TokensToReceive.IsNil() || redemption.TokensToReceive.IsNegative() || redemption.TokensToReceive.IsZero() {
		return fmt.Errorf("tokens to receive must be positive")
	}
	
	// Validate completion time is after creation time
	if redemption.CompletionTime.Before(redemption.CreationTime) {
		return fmt.Errorf("completion time must be after creation time")
	}
	
	return nil
}


// ValidateBasketMetadata validates basket metadata
func ValidateBasketMetadata(metadata BasketMetadata) error {
	// Metadata is optional, so empty values are allowed
	// We just check for reasonable length limits if provided
	
	if len(metadata.Name) > 128 {
		return fmt.Errorf("basket name too long (max 128 characters)")
	}
	
	if len(metadata.Description) > 512 {
		return fmt.Errorf("basket description too long (max 512 characters)")
	}
	
	if len(metadata.Symbol) > 32 {
		return fmt.Errorf("basket symbol too long (max 32 characters)")
	}
	
	return nil
}