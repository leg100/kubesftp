package internal

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/exp/maps"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type fakeSecretsClient struct {
	getFunc    func(ctx context.Context, name string, opts metav1.GetOptions) (*corev1.Secret, error)
	createFunc func(ctx context.Context, secret *corev1.Secret, opts metav1.CreateOptions) (*corev1.Secret, error)
}

func (f *fakeSecretsClient) Get(ctx context.Context, name string, opts metav1.GetOptions) (*corev1.Secret, error) {
	return f.getFunc(ctx, name, opts)
}

func (f *fakeSecretsClient) Create(ctx context.Context, secret *corev1.Secret, opts metav1.CreateOptions) (*corev1.Secret, error) {
	return f.createFunc(ctx, secret, opts)
}

// TestGetOrCreateHostKeys_SecretExists verifies that when the secret already
// exists, its data is written to the filesystem and the data keys are returned
// as paths.
func TestGetOrCreateHostKeys_SecretExists(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "ssh_host_ed25519_key")
	keyContent := []byte("fake-private-key")

	client := &fakeSecretsClient{
		getFunc: func(_ context.Context, _ string, _ metav1.GetOptions) (*corev1.Secret, error) {
			return &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "host-keys"},
				Data:       map[string][]byte{keyPath: keyContent},
			}, nil
		},
	}

	g := &generator{
		secrets: client,
		logger:  slog.New(slog.DiscardHandler),
		config:  config{HostKeysSecret: "host-keys"},
	}
	paths, err := g.getOrCreateHostKeys(t.Context(), "")
	require.NoError(t, err)
	assert.Equal(t, []string{keyPath}, paths)

	got, err := os.ReadFile(keyPath)
	require.NoError(t, err)
	assert.Equal(t, keyContent, got)
}

// TestGetOrCreateHostKeys_SecretNotFound verifies that when the secret does not
// exist, ssh-keygen is invoked, a new secret is created containing the
// generated keys, and the key file paths are returned.
func TestGetOrCreateHostKeys_SecretNotFound(t *testing.T) {
	if _, err := exec.LookPath("ssh-keygen"); err != nil {
		t.Skip("ssh-keygen not available")
	}

	prefix := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(prefix, "etc", "ssh"), 0o755))

	var createdSecret *corev1.Secret
	client := &fakeSecretsClient{
		getFunc: func(_ context.Context, name string, _ metav1.GetOptions) (*corev1.Secret, error) {
			return nil, errors.NewNotFound(schema.GroupResource{Resource: "secrets"}, name)
		},
		createFunc: func(_ context.Context, secret *corev1.Secret, _ metav1.CreateOptions) (*corev1.Secret, error) {
			createdSecret = secret
			return secret, nil
		},
	}

	g := &generator{
		secrets: client,
		logger:  slog.New(slog.DiscardHandler),
		config:  config{HostKeysSecret: "host-keys"},
	}

	paths, err := g.getOrCreateHostKeys(context.Background(), prefix)
	require.NoError(t, err)
	require.NotEmpty(t, paths)

	require.NotNil(t, createdSecret)
	assert.Equal(t, "host-keys", createdSecret.Name)
	assert.NotEmpty(t, createdSecret.Data)

	// Returned paths must match exactly what was stored in the secret.
	assert.ElementsMatch(t, paths, maps.Keys(createdSecret.Data))

	// Should be 6 keys: 3 pub and priv keys for 3 diff algos
	assert.Len(t, paths, 6)

	// Every path must exist on disk and be located inside the prefix.
	for _, p := range paths {
		assert.FileExists(t, p)
		assert.Contains(t, p, prefix)
	}
}

// TestGetOrCreateHostKeys_GetError verifies that a non-404 error from the
// secrets client is propagated with context.
func TestGetOrCreateHostKeys_GetError(t *testing.T) {
	client := &fakeSecretsClient{
		getFunc: func(_ context.Context, _ string, _ metav1.GetOptions) (*corev1.Secret, error) {
			return nil, fmt.Errorf("connection refused")
		},
	}

	g := &generator{
		secrets: client,
		logger:  slog.New(slog.DiscardHandler),
		config:  config{HostKeysSecret: "host-keys"},
	}

	_, err := g.getOrCreateHostKeys(context.Background(), "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "getting secret")
}
