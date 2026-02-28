package internal

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
)

func TestMetricsReceiveStartingSession(t *testing.T) {
	m := NewMetricsServer()

	m.receive(message{Message: "Starting session: subsystem 'sftp' for alice from ::1 port 38964 id 0"})

	assert.Equal(t, float64(1), testutil.ToFloat64(m.currentSessions))
}

func TestMetricsReceiveCloseSession(t *testing.T) {
	m := NewMetricsServer()
	m.currentSessions.Set(1)

	m.receive(message{Message: "Close session: user alice from ::1"})

	assert.Equal(t, float64(0), testutil.ToFloat64(m.currentSessions))
}

func TestMetricsReceiveUnrelated(t *testing.T) {
	m := NewMetricsServer()

	m.receive(message{Message: `realpath "."`})

	assert.Equal(t, float64(0), testutil.ToFloat64(m.currentSessions))
}

func TestMetricsSessionsMultiple(t *testing.T) {
	m := NewMetricsServer()

	m.receive(message{Message: "Starting session: subsystem 'sftp' for alice from ::1 port 38964 id 0"})
	m.receive(message{Message: "Starting session: subsystem 'sftp' for bob from ::1 port 38965 id 1"})
	assert.Equal(t, float64(2), testutil.ToFloat64(m.currentSessions))

	m.receive(message{Message: "Close session: user alice from ::1"})
	assert.Equal(t, float64(1), testutil.ToFloat64(m.currentSessions))

	m.receive(message{Message: "Close session: user bob from ::1"})
	assert.Equal(t, float64(0), testutil.ToFloat64(m.currentSessions))
}
