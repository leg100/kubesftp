package internal

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"

	"golang.org/x/exp/maps"
	corev1 "k8s.io/api/core/v1"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type secretsClient interface {
	Get(context.Context, string, metav1.GetOptions) (*corev1.Secret, error)
	Create(context.Context, *corev1.Secret, metav1.CreateOptions) (*corev1.Secret, error)
}

// getOrCreateHostKeys ensures host keys:
//
// 1. Exist in the configured kube secret
// 2. If secret does not exist, then one is created, and keys generated.
// 4. Finally, keys are written to the filesystem and their paths returned.
func getOrCreateHostKeys(
	ctx context.Context,
	logger slog.Logger,
	cfg config,
	client secretsClient,
	prefix string,
) ([]string, error) {
	//restCfg, err := rest.InClusterConfig()
	//if err != nil {
	//	return fmt.Errorf("building in-cluster config: %w", err)
	//}
	//client, err := kubernetes.NewForConfig(restCfg)
	//if err != nil {
	//	return fmt.Errorf("creating kubernetes client: %w", err)
	//}
	//secrets := client.CoreV1().Secrets(cfg.Namespace)

	secret, err := client.Get(ctx, cfg.HostKeysSecret, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		return generateHostKeysAndCreateSecret(ctx, logger, client, cfg.HostKeysSecret, prefix)
	} else if err != nil {
		return nil, fmt.Errorf("getting secret: %w", err)
	}
	for path, key := range secret.Data {
		if err := os.WriteFile(path, key, 0o600); err != nil {
			return nil, err
		}
	}
	return maps.Keys(secret.Data), nil
}

func generateHostKeysAndCreateSecret(ctx context.Context, logger slog.Logger, client secretsClient, name string, prefix string) ([]string, error) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
	cmd := exec.CommandContext(ctx, "ssh-keygen", "-A", "-f", prefix)
	if err := cmd.Run(); err != nil {
		return nil, err
	}
	paths, err := filepath.Glob(filepath.Join(prefix, "/etc/ssh/ssh_host_*"))
	if err != nil {
		return nil, err
	}
	secret.Data = make(map[string][]byte, len(paths))
	for _, path := range paths {
		key, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		secret.Data[path] = key
	}
	_, err = client.Create(ctx, secret, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("creating secret: %w", err)
	}
	return paths, nil
}

//log.Printf("secret %s/%s already exists, skipping key generation", cfg.Namespace, cfg.HostKeysSecret)
//log.Printf("created secret %s/%s with ed25519 host key", cfg.Namespace, cfg.HostKeysSecret)
