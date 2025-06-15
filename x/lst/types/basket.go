package types

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/address"
)

// GetBasketTokenDenom returns the denom for a basket token
func GetBasketTokenDenom(basketID string) string {
	return fmt.Sprintf("bTIA-%s", basketID)
}

// GetBasketAccountAddress returns the module account address for a specific basket
func GetBasketAccountAddress(basketID string) sdk.AccAddress {
	// Create a unique module account address for each basket
	return address.Module(ModuleName, []byte(fmt.Sprintf("basket-%s", basketID)))
}
