package tasks

import (
	"golang.org/x/sync/errgroup"
)

type Group struct {
	errGroup   errgroup.Group
	HandleCrit func(err error)
}

func (g *Group) Go(fn func() error) {
	g.errGroup.Go(func() error {
		return fn()
	})
}

func (g *Group) Wait() error {
	return g.errGroup.Wait()
}
