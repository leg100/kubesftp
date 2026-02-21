package internal

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfig(t *testing.T) {
	config := `
users:
  - username: bob
`

	configFilePath := filepath.Join(t.TempDir(), "config.yaml")
	err := os.WriteFile(configFilePath, []byte(config), 0o644)
	require.NoError(t, err)

	os.Setenv("HOST_KEYS_SECRET", "host-keys")
	os.Setenv("HOST_KEYS_ALGORITHMS", "ed25519")
	os.Setenv("POD_NAMESPACE", "sftp")
	os.Setenv("CONFIG_FILE_PATH", configFilePath)

	got, err := LoadConfig()
	require.NoError(t, err)

	assert.Equal(t, got.HostKeysAlgorithms, []algorithm{ed25519Algorithm})
	assert.Equal(t, got.HostKeysSecret, "host-keys")
	assert.Equal(t, got.Namespace, "sftp")

	assert.Len(t, got.Users, 1)
	assert.Contains(t, got.Users, user{Username: "bob"})
}
