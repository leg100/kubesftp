package internal

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"encoding/pem"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	corev1 "k8s.io/api/core/v1"

	"golang.org/x/crypto/ssh"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type secretsClient interface {
	Get(context.Context, string, metav1.GetOptions) (*corev1.Secret, error)
	Create(context.Context, *corev1.Secret, metav1.CreateOptions) (*corev1.Secret, error)
	Update(context.Context, *corev1.Secret, metav1.UpdateOptions) (*corev1.Secret, error)
}

// getOrCreateHostKeys ensures host keys:
//
// 1. Exist in the configured kube secret
// 2. If secret does not exist, then one is created, and keys generated.
// 3. If secret exists but not all keys exist then missing keys are created.
// 4. Finally, keys are written to the filesystem and their paths returned.
func getOrCreateHostKeys(
	ctx context.Context,
	logger slog.Logger,
	cfg config,
	client secretsClient,
	destDir string,
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

	var createSecret bool

	secret, err := client.Get(ctx, cfg.HostKeysSecret, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		createSecret = true
	} else if err != nil {
		return nil, fmt.Errorf("getting secret: %w", err)
	}

	if createSecret {
		secret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name: cfg.HostKeysSecret,
			},
			Data: make(map[string][]byte, len(cfg.HostKeysAlgorithms)),
		}
	}
	keys, generated, err := getOrGenerateKeys(secret.Data, cfg.HostKeysAlgorithms)
	if err != nil {
		return nil, err
	}
	if createSecret {
		_, err := client.Create(ctx, secret, metav1.CreateOptions{})
		if err != nil {
			return nil, fmt.Errorf("creating secret: %w", err)
		}
	} else if generated {
		_, err = client.Update(ctx, secret, metav1.UpdateOptions{})
		if err != nil {
			return nil, err
		}
	}
	paths := make([]string, len(cfg.HostKeysAlgorithms))
	for algo, key := range keys {
		path := filepath.Join(destDir, fmt.Sprintf("ssh_host_%s_key", algo))
		os.WriteFile(path, key, 0o600)
	}

	return paths, nil
}

func getOrGenerateKeys(secretData map[string][]byte, algorithms []algorithm) (keys map[algorithm][]byte, generated bool, err error) {
	keys = make(map[algorithm][]byte, len(algorithms))
	for _, algo := range algorithms {
		key, ok := secretData[string(algo)]
		if !ok {
			key, err := generateHostKey(algo)
			if err != nil {
				return nil, false, err
			}
			secretData[string(algo)] = key
			generated = true
		}
		keys[algo] = key
	}
	return keys, generated, nil
}

func generateHostKey(algo algorithm) ([]byte, error) {
	switch algo {
	case ed25519Algorithm:
		// Generate ed25519 host key.
		_, priv, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			return nil, fmt.Errorf("generating ed25519 key: %w", err)
		}
		signer, err := ssh.NewSignerFromKey(priv)
		if err != nil {
			return nil, fmt.Errorf("creating signer: %w", err)
		}
		privBlock, err := ssh.MarshalPrivateKey(signer, "")
		if err != nil {
			return nil, fmt.Errorf("marshalling private key: %w", err)
		}
		privPEM := pem.EncodeToMemory(privBlock)
		return privPEM, nil
	case rsaAlgorithm:
		// Generate ed25519 host key.
		priv, err := rsa.GenerateKey(rand.Reader, 3072)
		if err != nil {
			return nil, fmt.Errorf("generating ed25519 key: %w", err)
		}
		signer, err := ssh.NewSignerFromKey(priv)
		if err != nil {
			return nil, fmt.Errorf("creating signer: %w", err)
		}
		privBlock, err := ssh.MarshalPrivateKey(signer, "")
		if err != nil {
			return nil, fmt.Errorf("marshalling private key: %w", err)
		}
		privPEM := pem.EncodeToMemory(privBlock)
		return privPEM, nil
	}
	return nil, nil
}

//log.Printf("secret %s/%s already exists, skipping key generation", cfg.Namespace, cfg.HostKeysSecret)
//log.Printf("created secret %s/%s with ed25519 host key", cfg.Namespace, cfg.HostKeysSecret)
