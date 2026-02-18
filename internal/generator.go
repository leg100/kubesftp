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

const (
	sshdConfigPath    = "/etc/ssh/sshd_config"
	authorizedKeysDir = "/etc/ssh/authorized_keys"
	chrootsDir        = "/chroots"
)

type generator struct {
	logger  *slog.Logger
	secrets secretsClient
	config  config
}

type secretsClient interface {
	Get(context.Context, string, metav1.GetOptions) (*corev1.Secret, error)
	Create(context.Context, *corev1.Secret, metav1.CreateOptions) (*corev1.Secret, error)
}

func (g *generator) generate(ctx context.Context) error {
	paths, err := g.getOrCreateHostKeys(ctx, "")
	if err != nil {
		return err
	}
	if err := g.writeSSHDConfig(paths); err != nil {
		return err
	}
	if err := g.setupUsers(ctx); err != nil {
		return err
	}
	return nil
}

// getOrCreateHostKeys ensures host keys:
//
// 1. Exist in the configured kube secret
// 2. If secret does not exist, then one is created, and keys generated.
// 4. Finally, keys are written to the filesystem and their paths returned
// (public keys are excluded).
func (g *generator) getOrCreateHostKeys(ctx context.Context, prefix string) ([]string, error) {
	secret, err := g.secrets.Get(ctx, g.config.HostKeysSecret, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		return g.generateHostKeysAndCreateSecret(ctx, prefix)
	} else if err != nil {
		return nil, fmt.Errorf("getting secret: %w", err)
	}
	var privateKeyPaths []string
	for path, key := range secret.Data {
		var perms os.FileMode = 0o600
		if strings.HasSuffix(path, ".pub") {
			perms = 0o644
		} else {
			privateKeyPaths = append(privateKeyPaths, path)
		}
		if err := os.WriteFile(path, key, perms); err != nil {
			return nil, err
		}
	}
	return privateKeyPaths, nil
}

func (g *generator) generateHostKeysAndCreateSecret(ctx context.Context, prefix string) ([]string, error) {
	cmd := exec.CommandContext(ctx, "ssh-keygen", "-A", "-f", prefix)
	if err := cmd.Run(); err != nil {
		return nil, err
	}
	paths, err := filepath.Glob(filepath.Join(prefix, "/etc/ssh/ssh_host_*"))
	if err != nil {
		return nil, err
	}
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: g.config.HostKeysSecret,
		},
		Data: make(map[string][]byte, len(paths)),
	}
	for _, path := range paths {
		key, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		secret.Data[path] = key
	}
	_, err = g.secrets.Create(ctx, secret, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("creating secret: %w", err)
	}
	return paths, nil
}

func (g *generator) writeSSHDConfig(hostKeyPaths []string) error {
	config, err := g.generateSSHDConfig(hostKeyPaths)
	if err != nil {
		return err
	}
	if err := os.WriteFile(sshdConfigPath, []byte(config), 0o644); err != nil {
		return err
	}
	return nil
}

func (g *generator) generateSSHDConfig(hostKeyPaths []string) (string, error) {
	var b strings.Builder
	for _, path := range hostKeyPaths {
		fmt.Fprintf(&b, "HostKey %s\n", path)
	}
	fmt.Fprintf(&b, "AuthorizedKeysFile %s/%%u\n", authorizedKeysDir)
	fmt.Fprintf(&b, "ChrootDirectory %s/%%u\n", chrootsDir)
	b.WriteString(`
ForceCommand internal-sftp -f AUTH -l VERBOSE
AllowTcpForwarding no
X11Forwarding no
PasswordAuthentication no
Subsystem sftp internal-sftp
`)
	return b.String(), nil
}

func (g *generator) setupUsers(ctx context.Context) error {
	script, err := g.generateCreateUsersScript()
	if err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, "sh", "-c", script)
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}

func (g *generator) generateCreateUsersScript() (string, error) {
	var b strings.Builder
	b.WriteString("set -e\n")
	fmt.Fprintf(&b, "mkdir -p %s\n", chrootsDir)
	fmt.Fprintf(&b, "mkdir -p %s\n", authorizedKeysDir)

	uid := 4096
	for username, profile := range g.config.Users {
		homeDir := filepath.Join(chrootsDir, username, "home", username)

		fmt.Fprintf(&b, "addgroup -g %d %s\n", uid, username)
		// -D: don't assign a password yet.
		// -H: don't create the home dir yet.
		fmt.Fprintf(&b, "adduser -G %s -h %s -H -D -u %d -s /sbin/nologin %[1]s\n", username, homeDir, uid)
		// Disable password login
		fmt.Fprintf(&b, "echo %s:'*' | chpasswd\n", username)
		// Make their home dir inside chroots
		fmt.Fprintf(&b, "mkdir -p %s\n", homeDir)
		// Make them owner of their home dir.
		fmt.Fprintf(&b, "chown %s:%[1]s %s\n", username, homeDir)
		for _, key := range profile.AuthorizedKeys {
			fmt.Fprintf(&b, "echo %s\\n >> %s/%s\n", key, authorizedKeysDir, username)
		}
		uid++
	}

	return b.String(), nil
}
