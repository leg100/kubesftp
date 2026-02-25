package internal

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/exec"
	"github.com/testcontainers/testcontainers-go/wait"
)

// startAlpineContainer starts a long-running Alpine container for executing
// commands inside.
func startAlpineContainer(t *testing.T) testcontainers.Container {
	t.Helper()

	container, err := testcontainers.GenericContainer(t.Context(), testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:      "alpine:3.23.3",
			Cmd:        []string{"sleep", "infinity"},
			WaitingFor: wait.ForExec([]string{"true"}),
		},
		Started: true,
	})
	testcontainers.CleanupContainer(t, container, testcontainers.StopTimeout(0))
	require.NoError(t, err)
	return container
}

// containerExec runs a command inside the container and returns stdout.
func containerExec(t *testing.T, ctx context.Context, container testcontainers.Container, cmd []string) (int, string) {
	t.Helper()

	code, reader, err := container.Exec(ctx, cmd, exec.Multiplexed())
	require.NoError(t, err)

	data, err := io.ReadAll(reader)
	require.NoError(t, err)

	return code, string(data)
}

// TestGenerateSSHDConfig_Valid uses testcontainers to start an OpenSSH server
// with the generated sshd_config and verifies that sshd accepts the config.
func TestGenerateSSHDConfig_Valid(t *testing.T) {
	g := &Generator{
		logger: slog.New(slog.DiscardHandler),
		config: config{
			HostKeysSecret: "host-keys",
		},
	}

	hostKeyPaths := []string{
		"/etc/ssh/ssh_host_ed25519_key",
		"/etc/ssh/ssh_host_ecdsa_key",
		"/etc/ssh/ssh_host_rsa_key",
	}

	sshdConfig, err := g.generateSSHDConfig(hostKeyPaths)
	require.NoError(t, err)

	// Verify HostKey directives are present.
	for _, path := range hostKeyPaths {
		assert.Contains(t, sshdConfig, fmt.Sprintf("HostKey %s", path))
	}

	// Start an Alpine container with OpenSSH, generate host keys, write our
	// config, and validate it with `sshd -t`.
	container, err := testcontainers.GenericContainer(t.Context(), testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:      "sftp:edge",
			Entrypoint: []string{"sh", "-c", "ssh-keygen -A && sshd -t -f /etc/ssh/sshd_config"},
			Files: []testcontainers.ContainerFile{
				{
					Reader:            strings.NewReader(sshdConfig),
					ContainerFilePath: "/etc/ssh/sshd_config",
					FileMode:          0o644,
				},
			},
			WaitingFor: wait.ForExit(),
		},
		Started: true,
	})
	testcontainers.CleanupContainer(t, container)
	require.NoError(t, err)

	// sshd -t exits 0 if config is valid, non-zero otherwise.
	state, err := container.State(t.Context())
	require.NoError(t, err)
	if !assert.Equal(t, 0, state.ExitCode, "sshd -t should exit 0 for a valid config") {
		logs, _ := container.Logs(t.Context())
		b, _ := io.ReadAll(logs)
		t.Logf("--- container logs ---\n%s", string(b))
	}
}

// TestCreateUsers verifies that createUsers creates the expected groups, users,
// home directories, and authorized_keys files inside an Alpine container.
func TestCreateUsers(t *testing.T) {
	container := startAlpineContainer(t)

	users := []user{
		{
			Username: "alice",
			AuthorizedKeys: []string{
				"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIAlice alice@example.com",
			},
		},
		{
			Username: "bob",
			AuthorizedKeys: []string{
				"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIBob1 bob@example.com",
				"ssh-rsa AAAAB3NzaC1yc2EAAAABob2 bob@work.com",
			},
		},
	}

	g := &Generator{
		logger: slog.New(slog.DiscardHandler),
		config: config{
			Users: users,
		},
	}

	// Build a shell script that replicates what createUsers does.
	script, err := g.generateCreateUsersScript()
	require.NoError(t, err)

	code, output := containerExec(t, t.Context(), container, []string{"sh", "-c", script})
	require.Equal(t, 0, code, "createUsers script failed: %s", output)

	// Verify groups were created.
	for _, user := range users {
		code, output := containerExec(t, t.Context(), container, []string{"getent", "group", user.Username})
		assert.Equal(t, 0, code, "group %s should exist: %s", user.Username, output)
	}

	// Verify users were created with correct properties.
	for _, user := range users {
		code, output := containerExec(t, t.Context(), container, []string{"getent", "passwd", user.Username})
		assert.Equal(t, 0, code, "user %s should exist", user.Username)
		assert.Contains(t, output, fmt.Sprintf("/chroots/%s", user.Username), "user %s should have correct home dir", user.Username)
		assert.Contains(t, output, "/sbin/nologin", "user %s should have nologin shell", user.Username)
	}

	// Verify home directories exist and have correct ownership.
	for _, user := range users {
		homeDir := fmt.Sprintf("/chroots/%s/home/%s", user.Username, user.Username)
		code, _ := containerExec(t, t.Context(), container, []string{"test", "-d", homeDir})
		assert.Equal(t, 0, code, "home dir %s should exist", homeDir)

		// Check ownership: stat outputs uid:gid.
		code, output := containerExec(t, t.Context(), container, []string{"stat", "-c", "%U:%G", homeDir})
		assert.Equal(t, 0, code)
		assert.Contains(t, output, fmt.Sprintf("%s:%s", user.Username, user.Username),
			"home dir should be owned by %s:%s", user.Username, user.Username)
	}

	// Verify authorized_keys files.
	for _, user := range users {
		path := fmt.Sprintf("/etc/ssh/authorized_keys/%s", user.Username)
		code, output := containerExec(t, t.Context(), container, []string{"cat", path})
		assert.Equal(t, 0, code, "authorized_keys file for %s should exist", user.Username)
		for _, key := range user.AuthorizedKeys {
			assert.Contains(t, output, key, "authorized_keys for %s should contain key", user.Username)
		}
	}
}

func TestGenerateSyslogConfig(t *testing.T) {

	g := &Generator{
		logger: slog.New(slog.DiscardHandler),
		config: config{
			Users: []user{
				{
					Username: "alice",
					AuthorizedKeys: []string{
						"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIAlice alice@example.com",
					},
				},
				{
					Username: "bob",
					AuthorizedKeys: []string{
						"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIBob1 bob@example.com",
						"ssh-rsa AAAAB3NzaC1yc2EAAAABob2 bob@work.com",
					},
				},
			},
		},
	}
	got := g.generateSyslogConfig()
	want := `set -e
@version: 4.10
@include "scl.conf"
source sftp {
  unix-dgram("/chroots/alice/dev/log");
  unix-dgram("/chroots/bob/dev/log");
};
destination sftp { stdout(template("${ISODATE} ${PROGRAM} ${PID} ${MESSAGE}\n")); };
log { source(sftp); destination(sftp); };
`
	assert.Equal(t, want, got)
}
