package types

import (
	"fmt"
	"strings"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

const (
	// Message URLs for amino codec registration
	URLMsgCreateBasket      = "/celestia.lst.v1.MsgCreateBasket"
	URLMsgMintBasketToken   = "/celestia.lst.v1.MsgMintBasketToken"
	URLMsgRedeemBasketToken = "/celestia.lst.v1.MsgRedeemBasketToken"
	URLMsgConvertDelegation = "/celestia.lst.v1.MsgConvertDelegation"
	URLMsgConvertBasket     = "/celestia.lst.v1.MsgConvertBasket"
)

// Verify that our message types implement sdk.Msg
var (
	_ sdk.Msg = &MsgCreateBasket{}
	_ sdk.Msg = &MsgMintBasketToken{}
	_ sdk.Msg = &MsgRedeemBasketToken{}
	_ sdk.Msg = &MsgConvertDelegation{}
	_ sdk.Msg = &MsgConvertBasket{}
)

// NewMsgCreateBasket creates a new MsgCreateBasket
func NewMsgCreateBasket(
	creator sdk.AccAddress,
	validators []ValidatorWeight,
	metadata BasketMetadata,
) *MsgCreateBasket {
	return &MsgCreateBasket{
		Creator:    creator.String(),
		Validators: validators,
		Metadata:   &metadata,
	}
}

// ValidateBasic performs basic validation for MsgCreateBasket
func (msg *MsgCreateBasket) ValidateBasic() error {
	// Validate creator address
	if _, err := sdk.AccAddressFromBech32(msg.Creator); err != nil {
		return fmt.Errorf("invalid creator address: %w", err)
	}

	// Must have at least one validator
	if len(msg.Validators) == 0 {
		return fmt.Errorf("must provide at least one validator")
	}

	// Check for duplicate validators and validate addresses
	validatorAddrs := make(map[string]bool)
	totalWeight := math.LegacyZeroDec()

	for i, val := range msg.Validators {
		// Validate validator address format
		if _, err := sdk.ValAddressFromBech32(val.ValidatorAddress); err != nil {
			return fmt.Errorf("invalid validator address at index %d: %w", i, err)
		}

		// Check for duplicates
		if validatorAddrs[val.ValidatorAddress] {
			return fmt.Errorf("duplicate validator address: %s", val.ValidatorAddress)
		}
		validatorAddrs[val.ValidatorAddress] = true

		// Validate weight
		if val.Weight.IsNil() || val.Weight.IsNegative() || val.Weight.IsZero() {
			return fmt.Errorf("validator weight at index %d must be positive", i)
		}

		totalWeight = totalWeight.Add(val.Weight)
	}

	// Weights should sum to 1.0 (allowing small tolerance for precision)
	tolerance := math.LegacyNewDecWithPrec(1, 10) // 0.0000000001
	diff := totalWeight.Sub(math.LegacyOneDec()).Abs()
	if diff.GT(tolerance) {
		return fmt.Errorf("validator weights must sum to 1.0, got %s", totalWeight.String())
	}

	// Validate metadata
	if err := ValidateBasketMetadata(*msg.Metadata); err != nil {
		return fmt.Errorf("invalid metadata: %w", err)
	}

	return nil
}

// NewMsgMintBasketToken creates a new MsgMintBasketToken
func NewMsgMintBasketToken(
	minter sdk.AccAddress,
	basketID string,
	amount sdk.Coin,
) *MsgMintBasketToken {
	return &MsgMintBasketToken{
		Minter:   minter.String(),
		BasketId: basketID,
		Amount:   amount,
	}
}

// ValidateBasic performs basic validation for MsgMintBasketToken
func (msg *MsgMintBasketToken) ValidateBasic() error {
	// Validate minter address
	if _, err := sdk.AccAddressFromBech32(msg.Minter); err != nil {
		return fmt.Errorf("invalid minter address: %w", err)
	}

	// Validate basket ID
	if strings.TrimSpace(msg.BasketId) == "" {
		return fmt.Errorf("basket ID cannot be empty")
	}

	// Validate amount
	if !msg.Amount.IsValid() {
		return fmt.Errorf("invalid amount: %s", msg.Amount.String())
	}

	if !msg.Amount.IsPositive() {
		return fmt.Errorf("amount must be positive: %s", msg.Amount.String())
	}

	// For TIA network, we expect the native staking denom to be "utia"
	// This could be configurable, but we'll use a reasonable default
	expectedDenom := "utia"
	if msg.Amount.Denom != expectedDenom {
		return fmt.Errorf("expected denom %s, got %s", expectedDenom, msg.Amount.Denom)
	}

	return nil
}

// NewMsgRedeemBasketToken creates a new MsgRedeemBasketToken
func NewMsgRedeemBasketToken(
	redeemer sdk.AccAddress,
	basketID string,
	amount sdk.Coin,
) *MsgRedeemBasketToken {
	return &MsgRedeemBasketToken{
		Redeemer: redeemer.String(),
		BasketId: basketID,
		Amount:   amount,
	}
}

// ValidateBasic performs basic validation for MsgRedeemBasketToken
func (msg *MsgRedeemBasketToken) ValidateBasic() error {
	// Validate redeemer address
	if _, err := sdk.AccAddressFromBech32(msg.Redeemer); err != nil {
		return fmt.Errorf("invalid redeemer address: %w", err)
	}

	// Validate basket ID
	if strings.TrimSpace(msg.BasketId) == "" {
		return fmt.Errorf("basket ID cannot be empty")
	}

	// Validate amount
	if !msg.Amount.IsValid() {
		return fmt.Errorf("invalid amount: %s", msg.Amount.String())
	}

	if !msg.Amount.IsPositive() {
		return fmt.Errorf("amount must be positive: %s", msg.Amount.String())
	}

	// Basic validation of basket token denom format (should be like "bTIA-1")
	if !strings.HasPrefix(msg.Amount.Denom, "bTIA-") {
		return fmt.Errorf("invalid basket token denom format: %s", msg.Amount.Denom)
	}

	return nil
}

// NewMsgConvertDelegation creates a new MsgConvertDelegation
func NewMsgConvertDelegation(
	delegator sdk.AccAddress,
	validatorAddr sdk.ValAddress,
	amount sdk.Coin,
	basketID string,
) *MsgConvertDelegation {
	return &MsgConvertDelegation{
		Delegator:        delegator.String(),
		ValidatorAddress: validatorAddr.String(),
		Amount:           amount,
		BasketId:         basketID,
	}
}

// ValidateBasic performs basic validation for MsgConvertDelegation
func (msg *MsgConvertDelegation) ValidateBasic() error {
	// Validate delegator address
	if _, err := sdk.AccAddressFromBech32(msg.Delegator); err != nil {
		return fmt.Errorf("invalid delegator address: %w", err)
	}

	// Validate validator address
	if _, err := sdk.ValAddressFromBech32(msg.ValidatorAddress); err != nil {
		return fmt.Errorf("invalid validator address: %w", err)
	}

	// Validate basket ID
	if strings.TrimSpace(msg.BasketId) == "" {
		return fmt.Errorf("basket ID cannot be empty")
	}

	// Validate amount
	if !msg.Amount.IsValid() {
		return fmt.Errorf("invalid amount: %s", msg.Amount.String())
	}

	if !msg.Amount.IsPositive() {
		return fmt.Errorf("amount must be positive: %s", msg.Amount.String())
	}

	// For TIA network, we expect the native staking denom to be "utia"
	expectedDenom := "utia"
	if msg.Amount.Denom != expectedDenom {
		return fmt.Errorf("expected denom %s, got %s", expectedDenom, msg.Amount.Denom)
	}

	return nil
}

// NewMsgConvertBasket creates a new MsgConvertBasket
func NewMsgConvertBasket(
	converter sdk.AccAddress,
	fromBasketID string,
	toBasketID string,
	amount sdk.Coin,
) *MsgConvertBasket {
	return &MsgConvertBasket{
		Converter:    converter.String(),
		FromBasketId: fromBasketID,
		ToBasketId:   toBasketID,
		Amount:       amount,
	}
}

// ValidateBasic performs basic validation for MsgConvertBasket
func (msg *MsgConvertBasket) ValidateBasic() error {
	// Validate converter address
	if _, err := sdk.AccAddressFromBech32(msg.Converter); err != nil {
		return fmt.Errorf("invalid converter address: %w", err)
	}

	// Validate basket IDs
	if strings.TrimSpace(msg.FromBasketId) == "" {
		return fmt.Errorf("from basket ID cannot be empty")
	}

	if strings.TrimSpace(msg.ToBasketId) == "" {
		return fmt.Errorf("to basket ID cannot be empty")
	}

	// Source and target baskets must be different
	if msg.FromBasketId == msg.ToBasketId {
		return fmt.Errorf("source and target baskets must be different")
	}

	// Validate amount
	if !msg.Amount.IsValid() {
		return fmt.Errorf("invalid amount: %s", msg.Amount.String())
	}

	if !msg.Amount.IsPositive() {
		return fmt.Errorf("amount must be positive: %s", msg.Amount.String())
	}

	// Basic validation of source basket token denom format (should be like "bTIA-1")
	if !strings.HasPrefix(msg.Amount.Denom, "bTIA-") {
		return fmt.Errorf("invalid source basket token denom format: %s", msg.Amount.Denom)
	}

	return nil
}
