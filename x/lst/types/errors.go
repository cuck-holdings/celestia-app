package types

import (
	"cosmossdk.io/errors"
)

// x/lst module errors
var (
	ErrBasketNotFound       = errors.Register(ModuleName, 1100, "basket not found")
	ErrInvalidBasketID      = errors.Register(ModuleName, 1101, "invalid basket ID")
	ErrInvalidDenom         = errors.Register(ModuleName, 1102, "invalid denom")
	ErrInsufficientShares   = errors.Register(ModuleName, 1103, "insufficient basket shares")
	ErrInvalidValidatorSet  = errors.Register(ModuleName, 1104, "invalid validator set")
	ErrInvalidWeights       = errors.Register(ModuleName, 1105, "invalid validator weights")
	ErrValidatorNotFound    = errors.Register(ModuleName, 1106, "validator not found")
	ErrPendingNotFound      = errors.Register(ModuleName, 1107, "pending operation not found")
	ErrInvalidAmount        = errors.Register(ModuleName, 1108, "invalid amount")
	ErrRedelegationFailed   = errors.Register(ModuleName, 1109, "redelegation failed")
	ErrExchangeRateInvalid  = errors.Register(ModuleName, 1110, "invalid exchange rate")
)