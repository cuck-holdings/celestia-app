package types

import (
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
)

var ModuleCdc = codec.NewProtoCodec(codectypes.NewInterfaceRegistry())

func RegisterLegacyAminoCodec(cdc *codec.LegacyAmino) {
	// Register legacy amino codec messages here when needed
}

func RegisterInterfaces(registry codectypes.InterfaceRegistry) {
	// Register message interfaces here when messages are added
	// msgservice.RegisterMsgServiceDesc(registry, &_Msg_serviceDesc)
}
