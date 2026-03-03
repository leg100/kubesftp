package main

import (
	"context"
	"log"
	"log/slog"
	"os/signal"
	"syscall"

	"github.com/leg100/kubesftp/internal"
	"golang.org/x/sync/errgroup"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer cancel()
	go func() {
		<-ctx.Done()
		// Stop handling ^C; another ^C will exit the program.
		cancel()
	}()
	if err := run(ctx); err != nil {
		log.Fatal(err.Error())
	}
}

func run(ctx context.Context) error {
	cfg, err := internal.LoadConfig()
	if err != nil {
		return err
	}

	g, ctx := errgroup.WithContext(ctx)
	logger := slog.Default()

	// Start metrics server
	metrics, err := internal.StartMetricsServer(ctx, logger, g)
	if err != nil {
		return err
	}

	// Start syslog daemons for each chroot
	err = internal.StartSyslogDaemons(
		ctx,
		logger,
		g,
		cfg,
		&internal.Logger{Logger: logger},
		metrics,
	)
	if err != nil {
		return err
	}

	// Block until context canceled
	return g.Wait()
}
