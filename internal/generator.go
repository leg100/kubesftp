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
	sshDir     = "/etc/ssh"
	chrootsDir = "/chroots"
)

var (
	sshdConfigPath    = filepath.Join(sshDir, "sshd_config")
	authorizedKeysDir = filepath.Join(sshDir, "authorized_keys")
)

type Generator struct {
	hostKeys *hostKeys
	logger   *slog.Logger
	config   config
}

func NewGenerator(logger *slog.Logger, secrets secretsClient, config config) *Generator {
	return &Generator{
		logger: logger,
		config: config,
		hostKeys: &hostKeys{
			logger:     logger,
			secrets:    secrets,
			secretName: config.HostKeysSecret,
		},
	}
}

func (g *Generator) Generate(ctx context.Context) error {
	paths, err := g.hostKeys.getOrCreate(ctx, "")
	if err != nil {
		return fmt.Errorf("retrieving host keys: %w", err)
	}

	if err := g.writeSSHDConfig(paths); err != nil {
		return fmt.Errorf("generating sshd config: %w", err)
	}
	g.logger.Info("generated sshd config", "path", sshdConfigPath)

	if err := g.setupUsers(ctx); err != nil {
		return fmt.Errorf("setting up users: %w", err)
	}

	syslogConfig := g.generateSyslogConfig()
	if err := os.WriteFile("/etc/syslog-ng/syslog-ng.conf", []byte(syslogConfig), 0o644); err != nil {
		return fmt.Errorf("writing syslog config: %w", err)
	}
	return nil
}

func (g *Generator) writeSSHDConfig(hostKeyPaths []string) error {
	config, err := g.generateSSHDConfig(hostKeyPaths)
	if err != nil {
		return err
	}
	if err := os.WriteFile(sshdConfigPath, []byte(config), 0o644); err != nil {
		return err
	}
	return nil
}

func (g *Generator) generateSSHDConfig(hostKeyPaths []string) (string, error) {
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

func (g *Generator) setupUsers(ctx context.Context) error {
	script, err := g.generateCreateUsersScript()
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

func (g *Generator) generateCreateUsersScript() (string, error) {
	var b strings.Builder
	b.WriteString("set -e\n")
	fmt.Fprintf(&b, "mkdir -p %s\n", chrootsDir)
	fmt.Fprintf(&b, "chmod 755 %s\n", chrootsDir)
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
		fmt.Fprintf(&b, "mkdir -p %s\n", user.chrootHomeDir())
		// Make them owner of their home dir.
		fmt.Fprintf(&b, "chown %s:%[1]s %s\n", username, user.chrootHomeDir())
		for _, key := range user.AuthorizedKeys {
			fmt.Fprintf(&b, "echo %s\\n >> %s/%s\n", key, authorizedKeysDir, username)
		}
		// Create the dev directory for their log device
		fmt.Fprintf(&b, "mkdir -p %s\n", filepath.Dir(user.devLogPath()))
		fmt.Fprintf(&b, "chmod 755 %s\n", filepath.Dir(user.devLogPath()))
		uid++
		g.logger.Info("created user account", "username", user, "home", user.homeDir(), "chroot", user.chrootDir())
	}

	return b.String(), nil
}

func (g *Generator) generateSyslogConfig() string {
	var b strings.Builder

	b.WriteString("@version: 4.10\n")
	b.WriteString("@include \"scl.conf\"\n")
	b.WriteString("options {ts-format(iso) ; };")

	for _, user := range g.config.Users {
		fmt.Fprintf(&b, "source %v {\n", user)
		fmt.Fprintf(&b, "  unix-dgram(\"%s\");\n", user.devLogPath())
		b.WriteString("};\n")
		b.WriteString("log {\n")
		fmt.Fprintf(&b, " source(%v);\n", user)
		b.WriteString("  filter{program(\"internal-sftp\");};\n")
		fmt.Fprintf(&b, " destination { stdout(persist-name(\"%v-stdout\") template(\"$(format-json level=${PRIORITY} program=${PROGRAM} pid=${PID} msg=${MESSAGE} date=${ISODATE} user=%v)\n\")); };", user, user)
		fmt.Fprintf(&b, " destination { unix-dgram(\"%s\" persist-name(\"%v-unix\") template(\"$(format-json level=${PRIORITY} program=${PROGRAM} pid=${PID} msg=${MESSAGE} date=${ISODATE} user=%v)\n\")); };", user, user)
		b.WriteString("};\n")
	}

	fmt.Fprintf(&b, "destination sftp { stdout(template(\"${ISODATE} ${PROGRAM} ${PID} ${MESSAGE}\\n\")); };\n")
	b.WriteString("log { source(sftp); destination(sftp); };\n")

	return b.String()
}
