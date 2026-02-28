package internal

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	sshDir       = "/etc/ssh"
	etcMountPath = "/mnt/etc"
)

var (
	sshdConfigPath    = filepath.Join(sshDir, "sshd_config")
	authorizedKeysDir = filepath.Join(sshDir, "authorized_keys")
)

type generator struct {
	hostKeys *hostKeys
	logger   *slog.Logger
	config   config
}

func Generate(ctx context.Context, logger *slog.Logger, secrets secretsClient, config config) error {
	if config.HostKeysSecret == "" {
		return fmt.Errorf("host keys secret is required")
	}
	g := &generator{
		logger: logger,
		config: config,
		hostKeys: &hostKeys{
			logger:     logger,
			secrets:    secrets,
			secretName: config.HostKeysSecret,
		},
	}
	return g.generate(ctx)
}

func (g *generator) generate(ctx context.Context) error {
	paths, err := g.hostKeys.getOrCreate(ctx, "")
	if err != nil {
		return fmt.Errorf("retrieving host keys: %w", err)
	}

	if err := g.writeSSHDConfig(paths); err != nil {
		return fmt.Errorf("generating sshd config: %w", err)
	}
	g.logger.Info("generated sshd config", "path", sshdConfigPath)

	if err := g.generateAndRunScript(ctx, g.generateCreateUsersScript); err != nil {
		return fmt.Errorf("setting up users: %w", err)
	}

	if err := g.generateAndRunScript(ctx, g.generateCopyPathsScript); err != nil {
		return fmt.Errorf("copying paths: %w", err)
	}

	return nil
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
	fmt.Fprintf(&b, "ChrootDirectory %s/%%u\n", g.config.ChrootsDir)
	b.WriteString(`
ForceCommand internal-sftp -f AUTH -l VERBOSE
AllowTcpForwarding no
X11Forwarding no
PasswordAuthentication no
Subsystem sftp internal-sftp -f AUTH -l VERBOSE
SyslogFacility AUTH
LogLevel VERBOSE

`)
	return b.String(), nil
}

type createScriptFunc func() (string, error)

func (g *generator) generateAndRunScript(ctx context.Context, fn createScriptFunc) error {
	script, err := fn()
	if err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, "sh", "-c", script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %w", string(out), err)
	}
	return nil
}

func (g *generator) generateCreateUsersScript() (string, error) {
	var b strings.Builder
	b.WriteString("set -e\n")
	fmt.Fprintf(&b, "mkdir -p %s\n", g.config.ChrootsDir)
	fmt.Fprintf(&b, "chmod 755 %s\n", g.config.ChrootsDir)
	fmt.Fprintf(&b, "mkdir -p %s\n", authorizedKeysDir)

	uid := 4096
	for _, user := range g.config.Users {
		username := user.Username

		fmt.Fprintf(&b, "addgroup -g %d %s\n", uid, username)
		// -D: don't assign a password yet.
		// -H: don't create the home dir yet.
		fmt.Fprintf(&b, "adduser -G %s -h %s -H -D -u %d -s /sbin/nologin %[1]s\n", username, user.homeDir(), uid)
		// Disable password login
		fmt.Fprintf(&b, "echo %s:'*' | chpasswd\n", username)
		// Make their home dir inside chroots
		fmt.Fprintf(&b, "mkdir -p %s\n", user.chrootHomeDir(g.config.ChrootsDir))
		// Make them owner of their home dir.
		fmt.Fprintf(&b, "chown %s:%[1]s %s\n", username, user.chrootHomeDir(g.config.ChrootsDir))
		for _, key := range user.AuthorizedKeys {
			fmt.Fprintf(&b, "echo %s\\n >> %s/%s\n", key, authorizedKeysDir, username)
		}
		// Create the dev directory for their log device
		fmt.Fprintf(&b, "mkdir -p %s\n", filepath.Dir(user.devLogPath(g.config.ChrootsDir)))
		fmt.Fprintf(&b, "chmod 755 %s\n", filepath.Dir(user.devLogPath(g.config.ChrootsDir)))
		uid++
		g.logger.Info("created user account", "username", user, "home", user.homeDir(), "chroot", user.chrootDir(g.config.ChrootsDir))
	}

	return b.String(), nil
}

func (g *generator) generateCopyPathsScript() (string, error) {
	var b strings.Builder
	b.WriteString("set -e\n")
	fmt.Fprintf(&b, "cp -a %s %s\n", sshDir, etcMountPath)
	fmt.Fprintf(&b, "cp /etc/passwd %s\n", etcMountPath)
	fmt.Fprintf(&b, "cp /etc/shadow %s\n", etcMountPath)
	fmt.Fprintf(&b, "cp /etc/group %s\n", etcMountPath)
	return b.String(), nil
}
