package flags

import (
	"github.com/urfave/cli/v2"
	"time"
)

var Flags []cli.Flag

const envVarPrefix = "VRF_NODE"

func prefixEnvVars(name string) []string {
	return []string{envVarPrefix + "_" + name}
}

var RequiredFlags []cli.Flag = []cli.Flag{
	//MigrationFlag,
	ChainIdFlag,
	ChainRpcFlag,
	//MasterDbHostFlag,
	//MasterDbPortFlag,
	//MasterDbUserFlag,
	//MasterDbPasswordFlag,
	//MasterDbNameFlag,
	MainIntervalFlag,
	BlocksStepFlag,
	//EventIntervalFlag,
	//CallIntervalFlag,
	//PrivateKeyFlag,
	//VrfContractAddressFlag,
	//VrfFactoryContractAddressFlag,
	//CallerAddressFlag,
	//NumConfirmationsFlag,
	//SafeAbortNonceTooLowCountFlag,
	//SlaveDbEnableFlag,
}
var OptionalFlags []cli.Flag = []cli.Flag{
	//MnemonicFlag,
	//CallerHDPathFlag,
	//PassphraseFlag,
	StartingHeightFlag,
	//ConfirmationsFlag,
	//SlaveDbHostFlag,
	//SlaveDbPortFlag,
	//SlaveDbUserFlag,
	//SlaveDbPasswordFlag,
	//SlaveDbNameFlag,
}

var (
	MigrationFlag = &cli.StringFlag{
		Name:    "migrations-dir",
		Value:   "./migrations",
		Usage:   "path to migrations folder",
		EnvVars: prefixEnvVars("MIGRATIONS_DIR"),
	}
	ChainIdFlag = &cli.UintFlag{
		Name:     "chain-id",
		Usage:    "The port of the api",
		EnvVars:  prefixEnvVars("CHAIN_ID"),
		Value:    1,
		Required: true,
	}
	ChainRpcFlag = &cli.StringFlag{
		Name:     "chain-rpc",
		Usage:    "HTTP provider URL for L1",
		EnvVars:  prefixEnvVars("CHAIN_RPC"),
		Required: true,
	}
	StartingHeightFlag = &cli.Uint64Flag{
		Name:    "starting-height",
		Usage:   "The starting height of chain",
		EnvVars: prefixEnvVars("STARTING_HEIGHT"),
		Value:   0,
	}
	ConfirmationsFlag = &cli.Uint64Flag{
		Name:    "confirmations",
		Usage:   "The confirmation depth of l1",
		EnvVars: prefixEnvVars("CONFIRMATIONS"),
		Value:   64,
	}
	MainIntervalFlag = &cli.DurationFlag{
		Name:    "main-loop-interval",
		Usage:   "The interval of synchronization",
		EnvVars: prefixEnvVars("MAIN_LOOP_INTERVAL"),
		Value:   time.Second * 5,
	}
	BlocksStepFlag = &cli.UintFlag{
		Name:    "blocks-step",
		Usage:   "Scanner blocks step",
		EnvVars: prefixEnvVars("BLOCKS_STEP"),
		Value:   5,
	}
	EventIntervalFlag = &cli.DurationFlag{
		Name:    "event-loop-interval",
		Usage:   "The interval of event parse",
		EnvVars: prefixEnvVars("EVENT_LOOP_INTERVAL"),
		Value:   time.Second * 5,
	}
	CallIntervalFlag = &cli.DurationFlag{
		Name:    "call-loop-interval",
		Usage:   "The interval of contract caller",
		EnvVars: prefixEnvVars("CALL_LOOP_INTERVAL"),
		Value:   time.Second * 5,
	}
	PrivateKeyFlag = &cli.StringFlag{
		Name:     "private-key",
		Usage:    "Ethereum private key for caller contacts",
		EnvVars:  prefixEnvVars("PRIVATE_KEY"),
		Required: true,
	}
	VrfContractAddressFlag = &cli.StringFlag{
		Name:     "vrf-address",
		Usage:    "Address of the treasure manager contract",
		EnvVars:  prefixEnvVars("VRF_ADDRESS"),
		Required: true,
	}
	VrfFactoryContractAddressFlag = &cli.StringFlag{
		Name:     "vrf-factory-address",
		Usage:    "Address of the vrf factory",
		EnvVars:  prefixEnvVars("VRF_FACTORY_ADDRESS"),
		Required: true,
	}
	CallerAddressFlag = &cli.StringFlag{
		Name:     "caller_address",
		Usage:    "contract caller for vrf",
		EnvVars:  prefixEnvVars("CALLER_ADDRESS"),
		Required: true,
	}
	NumConfirmationsFlag = &cli.Uint64Flag{
		Name: "num-confirmations",
		Usage: "Number of confirmations which we will wait after " +
			"appending a new batch",
		EnvVars: prefixEnvVars("NUM_CONFIRMATIONS"),
		Value:   1,
	}
	SafeAbortNonceTooLowCountFlag = &cli.Uint64Flag{
		Name: "safe-abort-nonce-too-low-count",
		Usage: "Number of ErrNonceTooLow observations required to " +
			"give up on a tx at a particular nonce without receiving " +
			"confirmation",
		EnvVars: prefixEnvVars("SAFE_ABORT_NONCE_TOO_LOW_COUNT"),
		Value:   3,
	}

	MnemonicFlag = &cli.StringFlag{
		Name: "mnemonic",
		Usage: "The mnemonic used to derive the wallets for either the " +
			"sequencer or the proposer",
		EnvVars: prefixEnvVars("MNEMONIC"),
	}
	CallerHDPathFlag = &cli.StringFlag{
		Name: "sequencer-hd-path",
		Usage: "The HD path used to derive the sequencer wallet from the " +
			"mnemonic. The mnemonic flag must also be set.",
		EnvVars: prefixEnvVars("CALLER_HD_PATH"),
	}
	PassphraseFlag = &cli.StringFlag{
		Name:    "passphrase",
		Usage:   "passphrase for the seed generation process to increase the seed's security",
		EnvVars: prefixEnvVars("PASSPHRASE"),
		Value:   "",
	}

	// MasterDbHostFlag MasterDb Flags
	MasterDbHostFlag = &cli.StringFlag{
		Name:     "master-db-host",
		Usage:    "The host of the master database",
		EnvVars:  prefixEnvVars("MASTER_DB_HOST"),
		Required: true,
	}
	MasterDbPortFlag = &cli.IntFlag{
		Name:     "master-db-port",
		Usage:    "The port of the master database",
		EnvVars:  prefixEnvVars("MASTER_DB_PORT"),
		Required: true,
	}
	MasterDbUserFlag = &cli.StringFlag{
		Name:     "master-db-user",
		Usage:    "The user of the master database",
		EnvVars:  prefixEnvVars("MASTER_DB_USER"),
		Required: true,
	}
	MasterDbPasswordFlag = &cli.StringFlag{
		Name:     "master-db-password",
		Usage:    "The host of the master database",
		EnvVars:  prefixEnvVars("MASTER_DB_PASSWORD"),
		Required: true,
	}
	MasterDbNameFlag = &cli.StringFlag{
		Name:     "master-db-name",
		Usage:    "The db name of the master database",
		EnvVars:  prefixEnvVars("MASTER_DB_NAME"),
		Required: true,
	}

	SlaveDbEnableFlag = &cli.BoolFlag{
		Name:     "slave-db-enable",
		Usage:    "Whether to use slave db",
		EnvVars:  prefixEnvVars("SLAVE_DB_ENABLE"),
		Required: true,
	}

	// SlaveDbHostFlag Slave DB  flags
	SlaveDbHostFlag = &cli.StringFlag{
		Name:    "slave-db-host",
		Usage:   "The host of the slave database",
		EnvVars: prefixEnvVars("SLAVE_DB_HOST"),
	}
	SlaveDbPortFlag = &cli.IntFlag{
		Name:    "slave-db-port",
		Usage:   "The port of the slave database",
		EnvVars: prefixEnvVars("SLAVE_DB_PORT"),
	}
	SlaveDbUserFlag = &cli.StringFlag{
		Name:    "slave-db-user",
		Usage:   "The user of the slave database",
		EnvVars: prefixEnvVars("SLAVE_DB_USER"),
	}
	SlaveDbPasswordFlag = &cli.StringFlag{
		Name:    "slave-db-password",
		Usage:   "The host of the slave database",
		EnvVars: prefixEnvVars("SLAVE_DB_PASSWORD"),
	}
	SlaveDbNameFlag = &cli.StringFlag{
		Name:    "slave-db-name",
		Usage:   "The db name of the slave database",
		EnvVars: prefixEnvVars("SLAVE_DB_NAME"),
	}
)

func init() {
	Flags = append(RequiredFlags, OptionalFlags...)
}
