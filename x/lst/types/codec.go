package types

import (
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/msgservice"
)

var ModuleCdc = codec.NewProtoCodec(codectypes.NewInterfaceRegistry())

func RegisterLegacyAminoCodec(cdc *codec.LegacyAmino) {
	cdc.RegisterConcrete(&MsgCreateBasket{}, URLMsgCreateBasket, nil)
	cdc.RegisterConcrete(&MsgMintBasketToken{}, URLMsgMintBasketToken, nil)
	cdc.RegisterConcrete(&MsgRedeemBasketToken{}, URLMsgRedeemBasketToken, nil)
	cdc.RegisterConcrete(&MsgConvertDelegation{}, URLMsgConvertDelegation, nil)
	cdc.RegisterConcrete(&MsgConvertBasket{}, URLMsgConvertBasket, nil)
}

func RegisterInterfaces(registry codectypes.InterfaceRegistry) {
	registry.RegisterImplementations((*sdk.Msg)(nil),
		&MsgCreateBasket{},
		&MsgMintBasketToken{},
		&MsgRedeemBasketToken{},
		&MsgConvertDelegation{},
		&MsgConvertBasket{},
	)

	msgservice.RegisterMsgServiceDesc(registry, &_Msg_serviceDesc)
}
