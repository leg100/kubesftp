package internal

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"regexp"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"golang.org/x/sync/errgroup"
)

const metricsPort = 8080

// metrics exposes various metrics about the sftp server to the administrator.
type metrics struct {
	currentSessions prometheus.Gauge
	totalSessions   prometheus.Counter
}

func StartMetricsServer(ctx context.Context, logger *slog.Logger, g *errgroup.Group) (*metrics, error) {
	registry := prometheus.NewRegistry()
	currentSessions := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "kubesftp",
		Subsystem: "sessions",
		Name:      "current",
		Help:      "Current number of sessions",
	})
	registry.MustRegister(currentSessions)
	totalSessions := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "kubesftp",
		Subsystem: "sessions",
		Name:      "total",
		Help:      "Total number of sessions",
	})
	registry.MustRegister(totalSessions)

	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", metricsPort))
	if err != nil {
		return nil, err
	}
	logger.Info("metrics server now listening", "address", listener.Addr())

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{Registry: registry}))

	server := http.Server{Handler: mux}

	g.Go(func() error {
		if err := server.Serve(listener); err != http.ErrServerClosed {
			return err
		}
		return nil
	})
	g.Go(func() error {
		<-ctx.Done()
		if err := server.Close(); err != nil {
			return fmt.Errorf("shutting down metrics server: %w", err)
		}
		return nil
	})

	return &metrics{
		currentSessions: currentSessions,
		totalSessions:   totalSessions,
	}, nil
}

type metricMatcher struct {
	regexp *regexp.Regexp
	action func(m *metrics, matches ...string)
}

// 2026-02-26T10:04:55+00:00 sshd-session 4106298 alice Starting session: subsystem 'sf
// tp' for alice from ::1 port 38964 id 0
// 2026-02-26T10:04:55+00:00 internal-sftp 4106300 alice session opened for local user
// alice from [::1]
// 2026-02-26T10:04:55+00:00 internal-sftp 4106300 alice received client version 3
// 2026-02-26T10:04:55+00:00 internal-sftp 4106300 alice realpath "."
// 2026-02-26T10:04:57+00:00 internal-sftp 4106300 alice session closed for local user
// alice from [::1]
// 2026-02-26T10:04:57+00:00 sshd-session 4106298 alice Close session: user alice from
var matchers = []metricMatcher{
	{
		regexp: regexp.MustCompile(`session opened for local user`),
		action: func(m *metrics, matches ...string) {
			m.currentSessions.Inc()
			m.totalSessions.Inc()
		},
	},
	{
		regexp: regexp.MustCompile(`session closed for local user`),
		action: func(m *metrics, matches ...string) { m.currentSessions.Dec() },
	},
}

func (m *metrics) receive(msg message) {
	for _, mm := range matchers {
		if matches := mm.regexp.FindStringSubmatch(msg.Message); matches != nil {
			mm.action(m, matches...)
		}
	}
}
