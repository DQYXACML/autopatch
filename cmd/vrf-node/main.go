package main

import (
	"context"
	"github.com/ethereum/go-ethereum/log"
	"os"
)

func main() {
	log.SetDefault(log.NewLogger(log.NewTerminalHandlerWithLevel(os.Stderr, log.LevelInfo, true)))
	app := NewCli()
	err := app.RunContext(context.Background(), os.Args)
	if err != nil {
		log.Error("Application error", "error", err)
		os.Exit(1)
	}
}
