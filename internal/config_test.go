package internal

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfig(t *testing.T) {
	os.Setenv("HOST_KEYS_SECRET", "host-keys")
	os.Setenv("HOST_KEYS_ALGORITHMS", "ed25519")
	os.Setenv("POD_NAMESPACE", "sftp")

	got, err := loadConfig()
	require.NoError(t, err)

	assert.Equal(t, got.HostKeysAlgorithms, []algorithm{ed25519Algorithm})
	assert.Equal(t, got.HostKeysSecret, "host-keys")
	assert.Equal(t, got.Namespace, "sftp")
}
