package types

import (
	"encoding/binary"
	"fmt"
)

const (
	// ModuleName defines the module name
	ModuleName = "lst"

	// StoreKey defines the primary module store key
	StoreKey = ModuleName

	// MemStoreKey defines the in-memory store key
	MemStoreKey = "mem_lst"

	// ParamsKey defines the key used for storing module parameters
	ParamsKey = "params"
)

// KVStore key prefixes
var (
	// Basket state
	BasketKey        = []byte{0x10} // basket/{id} -> Basket
	BasketByDenomKey = []byte{0x11} // basketByDenom/{denom} -> basketID

	// Pending redemptions only (conversions use instant redelegation)
	PendingRedemptionKey  = []byte{0x20} // pendingRedemption/{id} -> PendingRedemption
	RedemptionByUserKey   = []byte{0x21} // redemptionByUser/{userAddr}/{id} -> nil
	RedemptionByBasketKey = []byte{0x22} // redemptionByBasket/{basketId}/{id} -> nil

	// Auto-incrementing counters
	NextBasketIDKey  = []byte{0x30} // nextBasketID -> uint64
	NextPendingIDKey = []byte{0x31} // nextPendingID -> uint64 (for redemptions only)
)

func KeyPrefix(p string) []byte {
	return []byte(p)
}

// BasketStoreKey returns the key for a basket by ID
func BasketStoreKey(basketID string) []byte {
	return append(BasketKey, []byte(basketID)...)
}

// BasketByDenomStoreKey returns the key for looking up basket ID by denom
func BasketByDenomStoreKey(denom string) []byte {
	return append(BasketByDenomKey, []byte(denom)...)
}

// PendingRedemptionStoreKey returns the key for a pending redemption by ID
func PendingRedemptionStoreKey(id uint64) []byte {
	bz := make([]byte, 8)
	binary.BigEndian.PutUint64(bz, id)
	return append(PendingRedemptionKey, bz...)
}

// RedemptionByUserStoreKey returns the key for indexing redemptions by user
func RedemptionByUserStoreKey(userAddr string, id uint64) []byte {
	key := append(RedemptionByUserKey, []byte(userAddr)...)
	key = append(key, []byte("/")...)
	bz := make([]byte, 8)
	binary.BigEndian.PutUint64(bz, id)
	return append(key, bz...)
}

// RedemptionByBasketStoreKey returns the key for indexing redemptions by basket
func RedemptionByBasketStoreKey(basketID string, id uint64) []byte {
	key := append(RedemptionByBasketKey, []byte(basketID)...)
	key = append(key, []byte("/")...)
	bz := make([]byte, 8)
	binary.BigEndian.PutUint64(bz, id)
	return append(key, bz...)
}

// GetBasketModuleAccountName returns the module account name for a basket
func GetBasketModuleAccountName(basketID string) string {
	return fmt.Sprintf("%s-basket-%s", ModuleName, basketID)
}