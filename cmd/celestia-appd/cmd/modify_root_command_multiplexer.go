//go:build multiplexer

package cmd

import (
	"github.com/spf13/cobra"

	"github.com/celestiaorg/celestia-app/multiplexer/abci"
	"github.com/celestiaorg/celestia-app/multiplexer/appd"
	multiplexer "github.com/celestiaorg/celestia-app/multiplexer/cmd"
	"github.com/celestiaorg/celestia-app/v4/app"
	"github.com/cosmos/cosmos-sdk/server"

	embedding "github.com/celestiaorg/celestia-app/v4/internal/embedding"
)

// modifyRootCommand enhances the root command with the pass through and multiplexer.
func modifyRootCommand(rootCommand *cobra.Command) {
	v3AppBinary, err := embedding.CelestiaAppV3()
	if err != nil {
		panic(err)
	}

	v3, err := appd.New("v3", v3AppBinary)
	if err != nil {
		panic(err)
	}

	versions, err := abci.NewVersions(abci.Version{
		Appd:        v3,
		ABCIVersion: abci.ABCIClientVersion1,
		AppVersion:  3,
		StartArgs: []string{
			"--grpc.enable",
			// ensure the grpc address is accessible from other hosts, not just locally.
			"--grpc.address=0.0.0.0:9090",
			"--api.enable",
			"--api.swagger=false",
			// we want to run the in standalone mode, as the comet node will be running natively in the multiplexer.
			"--with-tendermint=false",
			"--transport=grpc",
			"--address=0.0.0.0:26658",
			// the grpc_laddr field is required when starting the grpc server when running the embedded binary in standalone mode
			// like this https://github.com/celestiaorg/celestia-app/blob/13020ab1d5861ce15f4caef5d06673a3dec8c78d/cmd/celestia-appd/cmd/start.go#L509
			//
			// this will then wire up the proxy server so requests to the BlockApi on 9090 are proxied to 9099 where there is a gRPC server running natively
			// serving the BlockApi.

			// The this happens here https://github.com/celestiaorg/cosmos-sdk/blob/49b16e81263be6ca1c493f79eb13832e246d3f2f/server/grpc/server.go#L41
			"--rpc.grpc_laddr=tcp://0.0.0.0:9099",
			// "--v2-upgrade-height=0",
		},
	})
	if err != nil {
		panic(err)
	}

	rootCommand.AddCommand(
		multiplexer.NewPassthroughCmd(versions),
	)

	// Add the following commands to the rootCommand: start, tendermint, export, version, and rollback and wire multiplexer.
	server.AddCommandsWithStartCmdOptions(
		rootCommand,
		app.DefaultNodeHome,
		NewAppServer,
		appExporter,
		server.StartCmdOptions{
			AddFlags:            addStartFlags,
			StartCommandHandler: multiplexer.New(versions),
		},
	)
}
