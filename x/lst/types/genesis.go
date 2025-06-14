package types

import (
	"fmt"
)

// DefaultIndex is the default capability global index
const DefaultIndex uint64 = 1

// DefaultGenesis returns the default lst genesis state
func DefaultGenesis() *GenesisState {
	return &GenesisState{
		Params:             DefaultParams(),
		Baskets:            []Basket{},
		PendingRedemptions: []PendingRedemption{},
		NextBasketId:       1,
		NextPendingId:      1,
	}
}

// Validate performs basic genesis state validation returning an error upon any
// failure.
func (gs GenesisState) Validate() error {
	if err := gs.Params.Validate(); err != nil {
		return err
	}

	// Validate baskets
	basketIDs := make(map[string]bool)
	basketDenoms := make(map[string]bool)
	
	for _, basket := range gs.Baskets {
		if err := ValidateBasket(basket); err != nil {
			return fmt.Errorf("invalid basket %s: %w", basket.Id, err)
		}
		
		// Check for duplicate IDs
		if basketIDs[basket.Id] {
			return fmt.Errorf("duplicate basket ID: %s", basket.Id)
		}
		basketIDs[basket.Id] = true
		
		// Check for duplicate denoms
		if basketDenoms[basket.Denom] {
			return fmt.Errorf("duplicate basket denom: %s", basket.Denom)
		}
		basketDenoms[basket.Denom] = true
	}

	// Validate pending redemptions
	redemptionIDs := make(map[uint64]bool)
	for _, redemption := range gs.PendingRedemptions {
		if err := ValidatePendingRedemption(redemption); err != nil {
			return fmt.Errorf("invalid pending redemption %d: %w", redemption.Id, err)
		}
		
		if redemptionIDs[redemption.Id] {
			return fmt.Errorf("duplicate pending redemption ID: %d", redemption.Id)
		}
		redemptionIDs[redemption.Id] = true
		
		// Check that referenced basket exists
		if !basketIDs[redemption.BasketId] {
			return fmt.Errorf("pending redemption %d references non-existent basket %s", redemption.Id, redemption.BasketId)
		}
	}


	return nil
}
