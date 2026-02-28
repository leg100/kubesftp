package internal

import (
	"context"

	"golang.org/x/sync/errgroup"
)

func StartSyslogDaemons(ctx context.Context, g *errgroup.Group, cfg config, receivers ...syslogdReceiver) error {
	for _, user := range cfg.Users {
		d, err := newSyslogd(cfg.ChrootsDir, user, receivers...)
		if err != nil {
			return err
		}
		g.Go(func() error {
			return d.accept(ctx, g)
		})
		g.Go(func() error {
			<-ctx.Done()
			return d.listener.Close()
		})
	}
	return nil
}
