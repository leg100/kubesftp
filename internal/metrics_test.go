package internal

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
)

func TestMetricsReceiveStartingSession(t *testing.T) {
	m := StartMetricsServer()

	m.receive(message{Message: "session opened for local user alice from [::1]"})
	// alice from [::1]

	assert.Equal(t, float64(1), testutil.ToFloat64(m.currentSessions))
}

func TestMetricsReceiveCloseSession(t *testing.T) {
	m := StartMetricsServer()
	m.currentSessions.Set(1)

	m.receive(message{Message: "session closed for local user alice from [::1]"})

	assert.Equal(t, float64(0), testutil.ToFloat64(m.currentSessions))
}

func TestMetricsReceiveUnrelated(t *testing.T) {
	m := StartMetricsServer()

	m.receive(message{Message: `realpath "."`})

	assert.Equal(t, float64(0), testutil.ToFloat64(m.currentSessions))
}

func TestMetricsSessionsMultiple(t *testing.T) {
	m := StartMetricsServer()

	m.receive(message{Message: "session opened for local user alice from [::1]"})
	m.receive(message{Message: "session opened for local user bob from [::1]"})

	assert.Equal(t, float64(2), testutil.ToFloat64(m.currentSessions))

	m.receive(message{Message: "session closed for local user alice from [::1]"})
	assert.Equal(t, float64(1), testutil.ToFloat64(m.currentSessions))

	m.receive(message{Message: "session closed for local user bob from [::1]"})
	assert.Equal(t, float64(0), testutil.ToFloat64(m.currentSessions))
}
