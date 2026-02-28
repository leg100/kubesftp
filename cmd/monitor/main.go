package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/leg100/kubesftp/internal"
	"golang.org/x/sync/errgroup"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	// TODO: do we need to defer a cancel?
	go func() {
		<-ctx.Done()
		// Stop handling ^C; another ^C will exit the program.
		cancel()
	}()
	if err := run(ctx); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context) error {
	cfg, err := internal.LoadConfig()
	if err != nil {
		return err
	}

	// Register metrics and handler
	metrics := internal.NewMetricsServer()
	metrics.RegisterHandler()

	g, ctx := errgroup.WithContext(ctx)

	// Start syslog daemons for each chroot
	err = internal.StartSyslogDaemons(
		ctx,
		g,
		cfg,
		&internal.Logger{Out: os.Stdout},
		metrics,
	)
	if err != nil {
		return err
	}

	// Start metrics http server
	server := http.Server{
		Addr: ":8080",
	}
	g.Go(func() error {
		go server.ListenAndServe()
		<-ctx.Done()
		return server.Close()
	})

	// Block until context canceled
	return g.Wait()
}
