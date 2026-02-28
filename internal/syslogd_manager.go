package internal

import (
	"context"
	"log"

	"golang.org/x/sync/errgroup"
)

func StartSyslogDaemons(ctx context.Context, g *errgroup.Group, cfg config, receivers ...syslogdReceiver) error {
	for _, user := range cfg.Users {
		d, err := newSyslogd(cfg.ChrootsDir, user, receivers...)
		if err != nil {
			return err
		}
		g.Go(func() error {
			return d.connect(ctx)
		})
	}
	log.Printf("monitoring %d users\n", len(cfg.Users))
	return nil
}
