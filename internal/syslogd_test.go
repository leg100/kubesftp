package internal

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	sftpLogs1 = `<38>Feb 23 20:31:22 sshd-session[2237381]: Changed root directory to "/srv/sftp/jail/alice"`
	sftpLogs2 = `<38>Feb 23 20:31:22 sshd-session [2237381]: Starting session: subsystem 'sftp' for alice from ::1 port 51234 id 0`
	sftpLogs3 = `<38>Feb 23 20:31:22 internal-sftp[2237382]: session opened for local user alice from [::1]`
	sftpLogs4 = `<38>Feb 23 20:31:22 internal-sftp[2237382]: received client version 3`
	sftpLogs5 = `<38>Feb 23 20:31:22 internal-sftp[2237382]: realpath "."`
)

//i := []byte(`<165>4 2018-10-11T22:14:15.003Z mymach.it e - 1 [ex@32473 iut="3"] An application event log entry...`)
//p := rfc5424.NewParser()
//_, _ = p.Parse(i)

type mockReceiver struct {
	mu       sync.Mutex
	messages []message
}

func (m *mockReceiver) receive(msg message) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = append(m.messages, msg)
}

func (m *mockReceiver) received() []message {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]message(nil), m.messages...)
}

func TestProcessMessage(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantApp string
		wantPID int
		wantMsg string
		wantLvl string
	}{
		{
			name:    "chroot message",
			input:   sftpLogs1 + "\n",
			wantApp: "sshd-session",
			wantPID: 2237381,
			wantMsg: `Changed root directory to "/srv/sftp/jail/alice"`,
			wantLvl: "info",
		},
		{
			name:    "session opened",
			input:   sftpLogs3 + "\n",
			wantApp: "internal-sftp",
			wantPID: 2237382,
			wantMsg: "session opened for local user alice from [::1]",
			wantLvl: "info",
		},
		{
			name:    "client version",
			input:   sftpLogs4 + "\n",
			wantApp: "internal-sftp",
			wantPID: 2237382,
			wantMsg: "received client version 3",
			wantLvl: "info",
		},
		{
			name:    "realpath",
			input:   sftpLogs5 + "\n",
			wantApp: "internal-sftp",
			wantPID: 2237382,
			wantMsg: `realpath "."`,
			wantLvl: "info",
		},
		{
			name:    "session closed",
			input:   `<86>Feb 27 11:42:16 sshd-session[732409]: pam_unix(sshd:session): session closed for user bob`,
			wantApp: "sshd-session",
			wantPID: 732409,
			wantMsg: `pam_unix(sshd:session): session closed for user bob`,
			wantLvl: "info",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recv := &mockReceiver{}
			u := user{Username: "alice"}
			s := &syslogd{user: u, receivers: []syslogdReceiver{recv}}

			s.processMessage([]byte(tt.input))

			msgs := recv.received()
			require.Len(t, msgs, 1)
			assert.Equal(t, tt.wantApp, msgs[0].Appname)
			assert.Equal(t, tt.wantMsg, msgs[0].Message)
			assert.Equal(t, tt.wantLvl, msgs[0].Level)
			assert.Equal(t, u, msgs[0].User)
		})
	}
}

func TestProcessMessageInvalid(t *testing.T) {
	recv := &mockReceiver{}
	s := &syslogd{user: user{Username: "alice"}, receivers: []syslogdReceiver{recv}}

	s.processMessage([]byte("this is not a syslog message\n"))

	assert.Empty(t, recv.received())
}

func TestProcessMessageMultipleReceivers(t *testing.T) {
	recv1 := &mockReceiver{}
	recv2 := &mockReceiver{}
	s := &syslogd{
		user:      user{Username: "alice"},
		receivers: []syslogdReceiver{recv1, recv2},
	}

	s.processMessage([]byte(sftpLogs1 + "\n"))

	assert.Len(t, recv1.received(), 1)
	assert.Len(t, recv2.received(), 1)
}

func TestHandleUnix(t *testing.T) {
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "dev", "log")
	require.NoError(t, os.MkdirAll(filepath.Dir(socketPath), 0o755))

	addr := &net.UnixAddr{Name: socketPath, Net: "unixgram"}
	conn, err := net.ListenUnixgram("unixgram", addr)
	require.NoError(t, err)

	recv := &mockReceiver{}
	s := &syslogd{
		user:      user{Username: "alice"},
		receivers: []syslogdReceiver{recv},
		conn:      conn,
	}

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	go s.connect(ctx) //nolint

	conn, err = net.DialUnix("unixgram", nil, addr)
	require.NoError(t, err)
	defer conn.Close()

	_, err = conn.Write([]byte(sftpLogs3 + "\n"))
	require.NoError(t, err)
	conn.Close()

	assert.Eventually(t, func() bool {
		return len(recv.received()) == 1
	}, time.Second, 10*time.Millisecond)

	msgs := recv.received()
	assert.Equal(t, "internal-sftp", msgs[0].Appname)
}

func TestAcceptContextCancellation(t *testing.T) {
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "dev", "log")
	require.NoError(t, os.MkdirAll(filepath.Dir(socketPath), 0o755))

	addr := &net.UnixAddr{Name: socketPath, Net: "unixgram"}
	conn, err := net.ListenUnixgram("unixgram", addr)
	require.NoError(t, err)

	s := &syslogd{
		user: user{Username: "alice"},
		conn: conn,
	}

	ctx, cancel := context.WithCancel(t.Context())

	done := make(chan struct{})
	go func() {
		defer close(done)
		s.connect(ctx)
	}()

	cancel()
	conn.Close()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("accept did not return after context cancellation")
	}
}
