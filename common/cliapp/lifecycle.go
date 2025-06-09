package cliapp

import (
	"context"
	"errors"
	"fmt"
	"github.com/urfave/cli/v2"
	"os"
)

var interruptErr = errors.New("interrupted signal")

type Lifecycle interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	Stopped() bool
}

type LifecycleAction func(ctx *cli.Context) (Lifecycle, error)

type waitSignalFn func(ctx context.Context, signals ...os.Signal)

func LifecycleCmd(fn LifecycleAction) cli.ActionFunc {
	return lifecycleCmd(fn)
}

func lifecycleCmd(fn LifecycleAction) cli.ActionFunc {
	return func(ctx *cli.Context) error {
		hostCtx := ctx.Context
		appCtx, _ := context.WithCancelCause(ctx.Context)
		ctx.Context = appCtx

		appLifecycle, err := fn(ctx)
		if err != nil {
			return errors.Join(
				fmt.Errorf("failed to setup: %w", err),
				context.Cause(appCtx),
			)
		}
		if err := appLifecycle.Start(appCtx); err != nil {
			return errors.Join(
				fmt.Errorf("failed to start: %w", err),
				context.Cause(appCtx),
			)
		}
		<-appCtx.Done()

		stopCtx, _ := context.WithCancelCause(hostCtx)

		if err := appLifecycle.Stop(stopCtx); err != nil {
			return errors.Join(
				fmt.Errorf("failed to stop: %w", err),
				context.Cause(stopCtx),
			)
		}
		return nil
	}
}
