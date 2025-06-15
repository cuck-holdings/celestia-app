package keeper

import (
	"context"
	"fmt"
	"strconv"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"

	"github.com/celestiaorg/celestia-app/v4/x/lst/types"
)

type msgServer struct {
	Keeper
}

// NewMsgServerImpl returns an implementation of the MsgServer interface
// for the provided Keeper.
func NewMsgServerImpl(keeper Keeper) types.MsgServer {
	return &msgServer{Keeper: keeper}
}

var _ types.MsgServer = msgServer{}

func (k msgServer) CreateBasket(goCtx context.Context, msg *types.MsgCreateBasket) (*types.MsgCreateBasketResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	// Validate creator address
	_, err := sdk.AccAddressFromBech32(msg.Creator)
	if err != nil {
		return nil, types.ErrInvalidCreator.Wrapf("invalid creator address: %s", err.Error())
	}

	// Validate that all validators exist and are bonded
	for i, val := range msg.Validators {
		valAddr, err := sdk.ValAddressFromBech32(val.ValidatorAddress)
		if err != nil {
			return nil, types.ErrInvalidValidatorAddr.Wrapf("validator %d: %s", i, err.Error())
		}

		validator, err := k.stakingKeeper.GetValidator(ctx, valAddr)
		if err != nil {
			return nil, types.ErrValidatorNotFound.Wrapf("validator %s not found: %s", val.ValidatorAddress, err.Error())
		}

		// Only allow bonded validators
		if validator.Status != stakingtypes.Bonded {
			return nil, types.ErrInvalidValidatorSet.Wrapf("validator %s is not bonded", val.ValidatorAddress)
		}
	}

	// Get next basket ID
	nextID := k.GetNextBasketID(ctx)
	basketID := strconv.FormatUint(nextID, 10)

	// Create basket
	basketDenom := types.GetBasketTokenDenom(basketID)
	basket := types.Basket{
		Id:                basketID,
		Denom:             basketDenom,
		Validators:        msg.Validators,
		TotalShares:       math.LegacyZeroDec(),
		TotalStakedTokens: math.ZeroInt(),
		Creator:           msg.Creator,
		CreationTime:      ctx.BlockTime().Unix(),
		Metadata:          msg.Metadata,
	}

	// Store the basket
	k.SetBasket(ctx, basket)

	// Update next basket ID
	k.SetNextBasketID(ctx, nextID+1)

	// Create module account for this basket (for holding delegations)
	basketAccountAddr := types.GetBasketAccountAddress(basketID)
	if !k.accountKeeper.HasAccount(ctx, basketAccountAddr) {
		basketAccount := k.accountKeeper.NewAccountWithAddress(ctx, basketAccountAddr)
		k.accountKeeper.SetAccount(ctx, basketAccount)
	}

	// Emit event
	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			types.EventTypeCreateBasket,
			sdk.NewAttribute(types.AttributeKeyBasketID, basketID),
			sdk.NewAttribute(types.AttributeKeyCreator, msg.Creator),
		),
	)

	return &types.MsgCreateBasketResponse{
		BasketId: basketID,
		Denom:    basketDenom,
	}, nil
}

func (k msgServer) MintBasketToken(goCtx context.Context, msg *types.MsgMintBasketToken) (*types.MsgMintBasketTokenResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	// Validate minter address
	minter, err := sdk.AccAddressFromBech32(msg.Minter)
	if err != nil {
		return nil, types.ErrInvalidMinter.Wrapf("invalid minter address: %s", err.Error())
	}

	// Get basket
	basket, found := k.GetBasket(ctx, msg.BasketId)
	if !found {
		return nil, types.ErrBasketNotFound.Wrapf("basket %s not found", msg.BasketId)
	}

	// Validate amount is in staking denom
	stakingDenom, err := k.stakingKeeper.BondDenom(ctx)
	if err != nil {
		return nil, err
	}
	if msg.Amount.Denom != stakingDenom {
		return nil, types.ErrInvalidStakingDenom.Wrapf("expected %s, got %s", stakingDenom, msg.Amount.Denom)
	}

	// Calculate basket token amount to mint based on current exchange rate
	basketTokenAmount, err := k.calculateBasketTokensToMint(ctx, basket, msg.Amount.Amount)
	if err != nil {
		return nil, err
	}

	// Transfer tokens from minter to module account
	moduleAddr := k.accountKeeper.GetModuleAddress(types.ModuleName)
	if err := k.bankKeeper.SendCoins(ctx, minter, moduleAddr, sdk.NewCoins(msg.Amount)); err != nil {
		return nil, err
	}

	// Delegate to validators according to basket weights
	basketAccountAddr := types.GetBasketAccountAddress(msg.BasketId)
	for _, val := range basket.Validators {

		// Calculate delegation amount for this validator
		delegationAmount := val.Weight.MulInt(msg.Amount.Amount).TruncateInt()
		if delegationAmount.IsZero() {
			continue
		}

		delegationCoin := sdk.NewCoin(stakingDenom, delegationAmount)

		// Send tokens to basket account
		if err := k.bankKeeper.SendCoins(ctx, moduleAddr, basketAccountAddr, sdk.NewCoins(delegationCoin)); err != nil {
			return nil, err
		}

		// Delegate from basket account to validator
		valAddr, _ := sdk.ValAddressFromBech32(val.ValidatorAddress)
		validator, err := k.stakingKeeper.GetValidator(ctx, valAddr)
		if err != nil {
			return nil, err
		}
		_, err = k.stakingKeeper.Delegate(ctx, basketAccountAddr, delegationAmount, stakingtypes.Unbonded, validator, true)
		if err != nil {
			return nil, err
		}
	}

	// Mint basket tokens to minter
	basketDenom := types.GetBasketTokenDenom(msg.BasketId)
	basketCoin := sdk.NewCoin(basketDenom, basketTokenAmount)
	if err := k.bankKeeper.MintCoins(ctx, types.ModuleName, sdk.NewCoins(basketCoin)); err != nil {
		return nil, err
	}

	if err := k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, minter, sdk.NewCoins(basketCoin)); err != nil {
		return nil, err
	}

	// Update basket total shares
	basket.TotalShares = basket.TotalShares.Add(math.LegacyNewDecFromInt(basketTokenAmount))
	basket.TotalStakedTokens = basket.TotalStakedTokens.Add(msg.Amount.Amount)
	k.SetBasket(ctx, basket)

	// Emit event
	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			types.EventTypeMintBasketToken,
			sdk.NewAttribute(types.AttributeKeyBasketID, msg.BasketId),
			sdk.NewAttribute(types.AttributeKeyMinter, msg.Minter),
			sdk.NewAttribute(types.AttributeKeyAmount, msg.Amount.String()),
			sdk.NewAttribute(types.AttributeKeyBasketTokens, basketCoin.String()),
		),
	)

	return &types.MsgMintBasketTokenResponse{
		SharesMinted: math.LegacyNewDecFromInt(basketTokenAmount),
	}, nil
}

func (k msgServer) RedeemBasketToken(goCtx context.Context, msg *types.MsgRedeemBasketToken) (*types.MsgRedeemBasketTokenResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	// Validate redeemer address
	redeemer, err := sdk.AccAddressFromBech32(msg.Redeemer)
	if err != nil {
		return nil, types.ErrInvalidRedeemer.Wrapf("invalid redeemer address: %s", err.Error())
	}

	// Get basket
	basket, found := k.GetBasket(ctx, msg.BasketId)
	if !found {
		return nil, types.ErrBasketNotFound.Wrapf("basket %s not found", msg.BasketId)
	}

	// Validate basket token denom
	expectedDenom := types.GetBasketTokenDenom(msg.BasketId)
	if msg.Amount.Denom != expectedDenom {
		return nil, types.ErrInvalidBasketDenom.Wrapf("expected %s, got %s", expectedDenom, msg.Amount.Denom)
	}

	// Calculate underlying tokens to redeem
	underlyingAmount, err := k.calculateUnderlyingTokensToRedeem(ctx, basket, msg.Amount.Amount)
	if err != nil {
		return nil, err
	}

	// Burn basket tokens from redeemer
	if err := k.bankKeeper.SendCoinsFromAccountToModule(ctx, redeemer, types.ModuleName, sdk.NewCoins(msg.Amount)); err != nil {
		return nil, err
	}

	if err := k.bankKeeper.BurnCoins(ctx, types.ModuleName, sdk.NewCoins(msg.Amount)); err != nil {
		return nil, err
	}

	// Start unbonding from validators proportionally
	basketAccountAddr := types.GetBasketAccountAddress(msg.BasketId)
	totalUnbondingAmount := math.ZeroInt()

	for _, val := range basket.Validators {
		valAddr, _ := sdk.ValAddressFromBech32(val.ValidatorAddress)

		// Calculate unbonding amount for this validator
		unbondingAmount := val.Weight.MulInt(underlyingAmount).TruncateInt()
		if unbondingAmount.IsZero() {
			continue
		}

		// Start unbonding
		_, _, err := k.stakingKeeper.Undelegate(ctx, basketAccountAddr, valAddr, math.LegacyNewDecFromInt(unbondingAmount))
		if err != nil {
			return nil, err
		}

		totalUnbondingAmount = totalUnbondingAmount.Add(unbondingAmount)
	}

	// Create pending redemption entry
	unbondingTime, err := k.stakingKeeper.UnbondingTime(ctx)
	if err != nil {
		return nil, err
	}
	completionTime := ctx.BlockTime().Add(unbondingTime)

	redemptionID, err := k.CreatePendingRedemption(
		ctx,
		msg.BasketId,
		redeemer,
		math.LegacyNewDecFromInt(msg.Amount.Amount),
		totalUnbondingAmount,
		completionTime,
	)
	if err != nil {
		return nil, err
	}

	// Update basket total shares
	basket.TotalShares = basket.TotalShares.Sub(math.LegacyNewDecFromInt(msg.Amount.Amount))
	basket.TotalStakedTokens = basket.TotalStakedTokens.Sub(totalUnbondingAmount)
	k.SetBasket(ctx, basket)

	// Emit event
	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			types.EventTypeRedeemBasketToken,
			sdk.NewAttribute(types.AttributeKeyBasketID, msg.BasketId),
			sdk.NewAttribute(types.AttributeKeyRedeemer, msg.Redeemer),
			sdk.NewAttribute(types.AttributeKeyAmount, msg.Amount.String()),
			sdk.NewAttribute(types.AttributeKeyCompletionTime, completionTime.String()),
			sdk.NewAttribute("redemption_id", fmt.Sprintf("%d", redemptionID)),
		),
	)

	return &types.MsgRedeemBasketTokenResponse{
		PendingRedemptionId: redemptionID,
		CompletionTime:      completionTime.String(),
	}, nil
}

func (k msgServer) ConvertDelegation(goCtx context.Context, msg *types.MsgConvertDelegation) (*types.MsgConvertDelegationResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	// Validate delegator address
	delegator, err := sdk.AccAddressFromBech32(msg.Delegator)
	if err != nil {
		return nil, types.ErrInvalidDelegator.Wrapf("invalid delegator address: %s", err.Error())
	}

	// Validate validator address
	valAddr, err := sdk.ValAddressFromBech32(msg.ValidatorAddress)
	if err != nil {
		return nil, types.ErrInvalidValidatorAddr.Wrapf("invalid validator address: %s", err.Error())
	}

	// Get basket
	basket, found := k.GetBasket(ctx, msg.BasketId)
	if !found {
		return nil, types.ErrBasketNotFound.Wrapf("basket %s not found", msg.BasketId)
	}

	// Validate amount is in staking denom
	stakingDenom, err := k.stakingKeeper.BondDenom(ctx)
	if err != nil {
		return nil, err
	}
	if msg.Amount.Denom != stakingDenom {
		return nil, types.ErrInvalidStakingDenom.Wrapf("expected %s, got %s", stakingDenom, msg.Amount.Denom)
	}

	// Use redelegation to convert delegation to basket
	basketAccountAddr := types.GetBasketAccountAddress(msg.BasketId)
	basketTokenAmount, err := k.ConvertDelegationToBasket(ctx, delegator, valAddr, basketAccountAddr, msg.Amount.Amount, basket.Validators)
	if err != nil {
		return nil, err
	}

	// Mint basket tokens to delegator
	basketDenom := types.GetBasketTokenDenom(msg.BasketId)
	basketCoin := sdk.NewCoin(basketDenom, basketTokenAmount)
	if err := k.bankKeeper.MintCoins(ctx, types.ModuleName, sdk.NewCoins(basketCoin)); err != nil {
		return nil, err
	}

	if err := k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, delegator, sdk.NewCoins(basketCoin)); err != nil {
		return nil, err
	}

	// Update basket total shares
	basket.TotalShares = basket.TotalShares.Add(math.LegacyNewDecFromInt(basketTokenAmount))
	basket.TotalStakedTokens = basket.TotalStakedTokens.Add(msg.Amount.Amount)
	k.SetBasket(ctx, basket)

	// Emit event
	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			types.EventTypeConvertDelegation,
			sdk.NewAttribute(types.AttributeKeyDelegator, msg.Delegator),
			sdk.NewAttribute(types.AttributeKeyValidatorAddress, msg.ValidatorAddress),
			sdk.NewAttribute(types.AttributeKeyBasketID, msg.BasketId),
			sdk.NewAttribute(types.AttributeKeyAmount, msg.Amount.String()),
			sdk.NewAttribute(types.AttributeKeyBasketTokens, basketCoin.String()),
		),
	)

	return &types.MsgConvertDelegationResponse{
		SharesMinted: math.LegacyNewDecFromInt(basketTokenAmount),
	}, nil
}

func (k msgServer) ConvertBasket(goCtx context.Context, msg *types.MsgConvertBasket) (*types.MsgConvertBasketResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	// Validate converter address
	converter, err := sdk.AccAddressFromBech32(msg.Converter)
	if err != nil {
		return nil, types.ErrInvalidConverter.Wrapf("invalid converter address: %s", err.Error())
	}

	// Get source and target baskets
	fromBasket, found := k.GetBasket(ctx, msg.FromBasketId)
	if !found {
		return nil, types.ErrBasketNotFound.Wrapf("source basket %s not found", msg.FromBasketId)
	}

	toBasket, found := k.GetBasket(ctx, msg.ToBasketId)
	if !found {
		return nil, types.ErrBasketNotFound.Wrapf("target basket %s not found", msg.ToBasketId)
	}

	// Validate source basket token denom
	expectedFromDenom := types.GetBasketTokenDenom(msg.FromBasketId)
	if msg.Amount.Denom != expectedFromDenom {
		return nil, types.ErrInvalidBasketDenom.Wrapf("expected %s, got %s", expectedFromDenom, msg.Amount.Denom)
	}

	// Burn source basket tokens
	if err := k.bankKeeper.SendCoinsFromAccountToModule(ctx, converter, types.ModuleName, sdk.NewCoins(msg.Amount)); err != nil {
		return nil, err
	}

	if err := k.bankKeeper.BurnCoins(ctx, types.ModuleName, sdk.NewCoins(msg.Amount)); err != nil {
		return nil, err
	}

	// Convert between baskets using redelegation
	fromBasketAccountAddr := types.GetBasketAccountAddress(msg.FromBasketId)
	toBasketAccountAddr := types.GetBasketAccountAddress(msg.ToBasketId)

	targetBasketTokenAmount, err := k.ConvertBasketToBasket(ctx, fromBasketAccountAddr, toBasketAccountAddr, msg.Amount.Amount, fromBasket.Validators, toBasket.Validators)
	if err != nil {
		return nil, err
	}

	// Mint target basket tokens to converter
	targetBasketDenom := types.GetBasketTokenDenom(msg.ToBasketId)
	targetBasketCoin := sdk.NewCoin(targetBasketDenom, targetBasketTokenAmount)
	if err := k.bankKeeper.MintCoins(ctx, types.ModuleName, sdk.NewCoins(targetBasketCoin)); err != nil {
		return nil, err
	}

	if err := k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, converter, sdk.NewCoins(targetBasketCoin)); err != nil {
		return nil, err
	}

	// Update basket total shares
	fromBasket.TotalShares = fromBasket.TotalShares.Sub(math.LegacyNewDecFromInt(msg.Amount.Amount))
	toBasket.TotalShares = toBasket.TotalShares.Add(math.LegacyNewDecFromInt(targetBasketTokenAmount))
	k.SetBasket(ctx, fromBasket)
	k.SetBasket(ctx, toBasket)

	// Emit event
	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			types.EventTypeConvertBasket,
			sdk.NewAttribute(types.AttributeKeyConverter, msg.Converter),
			sdk.NewAttribute(types.AttributeKeyFromBasketID, msg.FromBasketId),
			sdk.NewAttribute(types.AttributeKeyToBasketID, msg.ToBasketId),
			sdk.NewAttribute(types.AttributeKeyAmount, msg.Amount.String()),
			sdk.NewAttribute(types.AttributeKeyTargetBasketTokens, targetBasketCoin.String()),
		),
	)

	return &types.MsgConvertBasketResponse{
		SharesMinted: math.LegacyNewDecFromInt(targetBasketTokenAmount),
	}, nil
}

// Helper function to calculate basket tokens to mint based on exchange rate
func (k msgServer) calculateBasketTokensToMint(ctx sdk.Context, basket types.Basket, stakingAmount math.Int) (math.Int, error) {
	// If this is the first minting, use 1:1 ratio
	if basket.TotalShares.IsZero() {
		return stakingAmount, nil
	}

	// Calculate exchange rate based on current basket value
	exchangeRate, err := k.GetBasketExchangeRate(ctx, basket.Id)
	if err != nil {
		return math.ZeroInt(), err
	}

	// basket_tokens = staking_amount / exchange_rate
	basketTokens := math.LegacyNewDecFromInt(stakingAmount).Quo(exchangeRate).TruncateInt()
	return basketTokens, nil
}

// Helper function to calculate underlying tokens to redeem
func (k msgServer) calculateUnderlyingTokensToRedeem(ctx sdk.Context, basket types.Basket, basketTokenAmount math.Int) (math.Int, error) {
	// Calculate exchange rate
	exchangeRate, err := k.GetBasketExchangeRate(ctx, basket.Id)
	if err != nil {
		return math.ZeroInt(), err
	}

	// underlying_tokens = basket_tokens * exchange_rate
	underlyingTokens := math.LegacyNewDecFromInt(basketTokenAmount).Mul(exchangeRate).TruncateInt()
	return underlyingTokens, nil
}
