package config

import (
	"github.com/DQYXACML/vrf-node/flags"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"github.com/urfave/cli/v2"
	"time"
)

type Config struct {
	Chain ChainConfig
}

type ChainConfig struct {
	ChainRpcUrl               string
	ChainId                   uint
	StartingHeight            uint64
	Confirmations             uint64
	BlockStep                 uint64
	Contracts                 []common.Address
	MainLoopInterval          time.Duration
	EventInterval             time.Duration
	CallInterval              time.Duration
	PrivateKey                string
	VrfContractAddress        string
	VrfFactoryContractAddress string
	CallerAddress             string
	NumConfirmations          uint64
	SafeAbortNonceTooLowCount uint64
	Mnemonic                  string
	CallerHDPath              string
	Passphrase                string
}

func LoadConfig(cliCtx *cli.Context) (Config, error) {
	cfg := NewConfig(cliCtx)
	log.Info("loaded chain config")
	return cfg, nil
}

func LoadContracts() []common.Address {
	var Contracts []common.Address
	Contracts = append(Contracts, VrfAddr)
	return Contracts
}

func NewConfig(cliCtx *cli.Context) Config {
	return Config{
		Chain: ChainConfig{
			ChainId:          cliCtx.Uint(flags.ChainIdFlag.Name),
			ChainRpcUrl:      cliCtx.String(flags.ChainRpcFlag.Name),
			MainLoopInterval: cliCtx.Duration(flags.MainIntervalFlag.Name),
			BlockStep:        cliCtx.Uint64(flags.BlocksStepFlag.Name),
			StartingHeight:   cliCtx.Uint64(flags.StartingHeightFlag.Name),
		},
	}
}
