package internal

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//log.Printf("secret %s/%s already exists, skipping key generation", cfg.Namespace, cfg.HostKeysSecret)
//log.Printf("created secret %s/%s with ed25519 host key", cfg.Namespace, cfg.HostKeysSecret)

type hostKeys struct {
	logger     *slog.Logger
	secrets    secretsClient
	secretName string
}

type secretsClient interface {
	Get(context.Context, string, metav1.GetOptions) (*corev1.Secret, error)
	Create(context.Context, *corev1.Secret, metav1.CreateOptions) (*corev1.Secret, error)
}

// getOrCreate ensures host keys:
//
// 1. Exist in the configured kube secret
// 2. If secret does not exist, then one is created, and keys generated.
// 4. Finally, keys are written to the filesystem and their paths returned
// (public keys are excluded).
//
// parentDir specifies the parent directory in which keys and their
// subdirectories are written, i.e. (/<parentDir>/etc/ssh/<key>).
func (g *hostKeys) getOrCreate(ctx context.Context, parentDir string) ([]string, error) {
	secret, err := g.secrets.Get(ctx, g.secretName, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		// Secret not found: generate host keys, and persist to secret.
		paths, err := g.generate(ctx, parentDir)
		if err != nil {
			return nil, err
		}
		g.logger.Info("generated host keys", "paths", paths)
		if err := g.createSecret(ctx, paths); err != nil {
			return nil, err
		}
		g.logger.Info("created secret containing host keys", "name", g.secretName)

		// Call function again now that secret exists with keys.
		return g.getOrCreate(ctx, parentDir)
	} else if err != nil {
		return nil, fmt.Errorf("retrieving secret: %w", err)
	}
	g.logger.Info("retrieved secret containing host keys", "name", g.secretName)

	// Ensure directory path exists
	parentDir = filepath.Join(parentDir, sshDir)
	if err := os.MkdirAll(parentDir, 0o755); err != nil {
		return nil, err
	}

	// Write private keys to disk and return their paths
	var privateKeyPaths []string
	for filename, key := range secret.Data {
		if strings.HasSuffix(filename, ".pub") {
			// Skip public keys, they're not needed by sshd (but the user may want
			// them for their clients, in which case they can retrieve them from the
			// secret).
			continue
		}
		path := filepath.Join(parentDir, filename)
		if err := os.WriteFile(path, key, 0o600); err != nil {
			return nil, fmt.Errorf("writing host key: %w", err)
		}
		privateKeyPaths = append(privateKeyPaths, path)
	}
	g.logger.Info("written host keys to disk", "paths", privateKeyPaths)
	return privateKeyPaths, nil
}

// generate generates the sshd host keys and returns the key paths
func (g *hostKeys) generate(ctx context.Context, parentDir string) ([]string, error) {
	cmd := exec.CommandContext(ctx, "ssh-keygen", "-A", "-f", parentDir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("generating ssh host keys: %s: %w", out, err)
	}
	paths, err := filepath.Glob(filepath.Join(parentDir, "/etc/ssh/ssh_host_*"))
	if err != nil {
		return nil, err
	}
	return paths, nil
}

func (g *hostKeys) createSecret(ctx context.Context, paths []string) error {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: g.secretName,
		},
		Data: make(map[string][]byte, len(paths)),
	}
	for _, path := range paths {
		key, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		secret.Data[filepath.Base(path)] = key
	}
	_, err := g.secrets.Create(ctx, secret, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("creating secret: %w", err)
	}
	return nil
}
