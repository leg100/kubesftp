package internal

import (
	"net/http"
	"regexp"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// metrics exposes various metrics about the sftp server to the administrator.
type metrics struct {
	registry        *prometheus.Registry
	currentSessions prometheus.Gauge
	totalSessions   prometheus.Counter
}

func NewMetricsServer() *metrics {
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

	return &metrics{
		registry:        registry,
		currentSessions: currentSessions,
		totalSessions:   totalSessions,
	}
}

func (m *metrics) RegisterHandler() {
	http.Handle("/metrics", promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{Registry: m.registry}))
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
		regexp: regexp.MustCompile(`^Starting session:`),
		action: func(m *metrics, matches ...string) {
			m.currentSessions.Inc()
			m.totalSessions.Inc()
		},
	},
	{
		regexp: regexp.MustCompile(`^Close session:`),
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
