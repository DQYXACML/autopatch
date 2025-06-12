package main

import (
	auto_patch "github.com/DQYXACML/autopatch"
	"github.com/DQYXACML/autopatch/common/cliapp"
	"github.com/DQYXACML/autopatch/config"
	"github.com/DQYXACML/autopatch/flags"
	"github.com/ethereum/go-ethereum/log"
	"github.com/urfave/cli/v2"
)

func runAutoPatchNode(ctx *cli.Context) (cliapp.Lifecycle, error) {
	log.Info("test in runAutoPatchNode")
	cfg, err := config.LoadConfig(ctx)
	if err != nil {
		log.Error("failed to load config", "error", err)
		return nil, err
	}
	return auto_patch.NewAutoPatch(ctx.Context, &cfg)
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
				Action:      cliapp.LifecycleCmd(runAutoPatchNode),
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
