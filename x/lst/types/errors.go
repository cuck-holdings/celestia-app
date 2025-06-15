package types

import (
	"cosmossdk.io/errors"
)

// x/lst module errors
var (
	ErrBasketNotFound         = errors.Register(ModuleName, 1100, "basket not found")
	ErrInvalidBasketID        = errors.Register(ModuleName, 1101, "invalid basket ID")
	ErrInvalidDenom           = errors.Register(ModuleName, 1102, "invalid denom")
	ErrInsufficientShares     = errors.Register(ModuleName, 1103, "insufficient basket shares")
	ErrInvalidValidatorSet    = errors.Register(ModuleName, 1104, "invalid validator set")
	ErrInvalidWeights         = errors.Register(ModuleName, 1105, "invalid validator weights")
	ErrValidatorNotFound      = errors.Register(ModuleName, 1106, "validator not found")
	ErrPendingNotFound        = errors.Register(ModuleName, 1107, "pending operation not found")
	ErrInvalidAmount          = errors.Register(ModuleName, 1108, "invalid amount")
	ErrRedelegationFailed     = errors.Register(ModuleName, 1109, "redelegation failed")
	ErrExchangeRateInvalid    = errors.Register(ModuleName, 1110, "invalid exchange rate")
	ErrInvalidCreator         = errors.Register(ModuleName, 1111, "invalid creator address")
	ErrNoValidators           = errors.Register(ModuleName, 1112, "no validators provided")
	ErrDuplicateValidator     = errors.Register(ModuleName, 1113, "duplicate validator address")
	ErrInvalidValidatorAddr   = errors.Register(ModuleName, 1114, "invalid validator address")
	ErrInvalidMinter          = errors.Register(ModuleName, 1115, "invalid minter address")
	ErrInvalidRedeemer        = errors.Register(ModuleName, 1116, "invalid redeemer address")
	ErrInvalidDelegator       = errors.Register(ModuleName, 1117, "invalid delegator address")
	ErrInvalidConverter       = errors.Register(ModuleName, 1118, "invalid converter address")
	ErrSameBaskets            = errors.Register(ModuleName, 1119, "source and target baskets cannot be the same")
	ErrInvalidBasketDenom     = errors.Register(ModuleName, 1120, "invalid basket token denom")
	ErrInvalidStakingDenom    = errors.Register(ModuleName, 1121, "invalid staking denom")
	ErrWeightsSumIncorrect    = errors.Register(ModuleName, 1122, "validator weights must sum to 1.0")
	ErrZeroWeight             = errors.Register(ModuleName, 1123, "validator weight must be positive")
)