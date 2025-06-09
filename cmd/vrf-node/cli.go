package main

import (
	"github.com/DQYXACML/autopath/common/cliapp"
	"github.com/DQYXACML/autopath/config"
	"github.com/DQYXACML/autopath/flags"
	vrf_node "github.com/DQYXACML/vrf-node"
	"github.com/ethereum/go-ethereum/log"
	"github.com/urfave/cli/v2"
)

func runVRFNode(ctx *cli.Context) (cliapp.Lifecycle, error) {
	log.Info("test in runVRFNode")
	cfg, err := config.LoadConfig(ctx)
	if err != nil {
		log.Error("failed to load config", "error", err)
		return nil, err
	}
	return vrf_node.NewVrfNode(ctx.Context, &cfg)
}

func NewCli() *cli.App {
	myFlags := flags.Flags
	return &cli.App{
		Version:              "v0.0.1",
		Description:          "An indexer of all optimism events with a serving api layer",
		EnableBashCompletion: true,
		Commands: []*cli.Command{
			{
				Name:        "index",
				Description: "Runs the indexing service",
				Flags:       myFlags,
				Action:      cliapp.LifecycleCmd(runVRFNode),
			},
			{
				Name:        "version",
				Description: "print version",
				Action: func(ctx *cli.Context) error {
					cli.ShowVersion(ctx)
					return nil
				},
			},
		},
	}
}
