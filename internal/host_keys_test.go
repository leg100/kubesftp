package internal

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
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
	keyName := "ssh_host_ed25519_key"
	keyPath := filepath.Join(dir, sshDir, "ssh_host_ed25519_key")
	keyContent := []byte("fake-private-key")

	client := &fakeSecretsClient{
		getFunc: func(_ context.Context, _ string, _ metav1.GetOptions) (*corev1.Secret, error) {
			return &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "host-keys"},
				Data:       map[string][]byte{keyName: keyContent},
			}, nil
		},
	}

	g := &hostKeys{
		secrets:    client,
		logger:     slog.New(slog.DiscardHandler),
		secretName: "host-keys",
	}
	paths, err := g.getOrCreate(t.Context(), dir)
	require.NoError(t, err)
	assert.Equal(t, []string{keyPath}, paths)

	got, err := os.ReadFile(keyPath)
	require.NoError(t, err)
	assert.Equal(t, keyContent, got)
}

// TestHostKeys_SecretNotFound verifies that when the secret does not
// exist, ssh-keygen is invoked, a new secret is created containing the
// generated keys, and the key file paths are returned.
func TestHostKeys_SecretNotFound(t *testing.T) {
	prefix := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(prefix, "etc", "ssh"), 0o755))

	var createdSecret *corev1.Secret
	client := &fakeSecretsClient{
		getFunc: func(_ context.Context, name string, _ metav1.GetOptions) (*corev1.Secret, error) {
			// Retrun secret if it has been created, otherwise return not found.
			if createdSecret != nil {
				return createdSecret, nil
			} else {
				return nil, errors.NewNotFound(schema.GroupResource{Resource: "secrets"}, name)
			}
		},
		createFunc: func(_ context.Context, secret *corev1.Secret, _ metav1.CreateOptions) (*corev1.Secret, error) {
			createdSecret = secret
			return secret, nil
		},
	}

	g := &hostKeys{
		secrets:    client,
		logger:     slog.New(slog.DiscardHandler),
		secretName: "host-keys",
	}

	paths, err := g.getOrCreate(t.Context(), prefix)
	require.NoError(t, err)
	require.NotEmpty(t, paths)
	filenames := make([]string, len(paths))
	for i, path := range paths {
		filenames[i] = filepath.Base(path)
	}

	require.NotNil(t, createdSecret)
	assert.Equal(t, "host-keys", createdSecret.Name)
	assert.NotEmpty(t, createdSecret.Data)

	// Retrieve paths of private keys from secret.
	secretPrivateKeyPaths := slices.DeleteFunc(
		maps.Keys(createdSecret.Data),
		func(path string) bool { return strings.HasSuffix(path, ".pub") },
	)
	// Ensure that they match the filenames returned by the func under test.
	assert.ElementsMatch(t, filenames, secretPrivateKeyPaths)

	// Should be 3 private keys
	assert.Len(t, paths, 3)

	// Every private key path must exist on disk and be located inside the prefix.
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

	g := &hostKeys{
		secrets:    client,
		logger:     slog.New(slog.DiscardHandler),
		secretName: "host-keys",
	}

	_, err := g.getOrCreate(t.Context(), "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "retrieving secret")
}
