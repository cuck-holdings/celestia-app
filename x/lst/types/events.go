package types

// Event types for the lst module
const (
	EventTypeCreateBasket      = "create_basket"
	EventTypeMintBasketToken   = "mint_basket_token"
	EventTypeRedeemBasketToken = "redeem_basket_token"
	EventTypeConvertDelegation = "convert_delegation"
	EventTypeConvertBasket     = "convert_basket"
)

// Event attribute keys
const (
	AttributeKeyBasketID           = "basket_id"
	AttributeKeyCreator            = "creator"
	AttributeKeyMinter             = "minter"
	AttributeKeyRedeemer           = "redeemer"
	AttributeKeyDelegator          = "delegator"
	AttributeKeyConverter          = "converter"
	AttributeKeyValidatorAddress   = "validator_address"
	AttributeKeyAmount             = "amount"
	AttributeKeyBasketTokens       = "basket_tokens"
	AttributeKeyCompletionTime     = "completion_time"
	AttributeKeyFromBasketID       = "from_basket_id"
	AttributeKeyToBasketID         = "to_basket_id"
	AttributeKeyTargetBasketTokens = "target_basket_tokens"
)