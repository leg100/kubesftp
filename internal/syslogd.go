package internal

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"time"

	"github.com/leodido/go-syslog/v4/rfc3164"
)

// syslogd aggregates and relays syslog messages from a user's chroot.
type syslogd struct {
	user      user
	receivers []syslogdReceiver
	conn      *net.UnixConn
}

type syslogdReceiver interface {
	receive(message)
}

type message struct {
	Timestamp *time.Time
	Appname   string
	PID       int
	Message   string
	User      user
	Level     string
}

func newSyslogd(chrootsParentDir string, user user, dests ...syslogdReceiver) (*syslogd, error) {
	socket := user.devLogPath(chrootsParentDir)
	// Remove existing socket file if it exists
	if err := os.RemoveAll(socket); err != nil {
		return nil, fmt.Errorf("failed to remove existing socket: %w", err)
	}
	addr, err := net.ResolveUnixAddr("unixgram", socket)
	if err != nil {
		return nil, err
	}
	conn, err := net.ListenUnixgram("unixgram", addr)
	if err != nil {
		return nil, err
	}
	if err := os.Chmod(socket, 0o666); err != nil {
		return nil, err
	}
	return &syslogd{
		user:      user,
		receivers: dests,
		conn:      conn,
	}, nil
}

func (s *syslogd) connect(ctx context.Context) error {
	buf := make([]byte, 1024)
	for {
		if ctx.Err() != nil {
			return nil
		}
		n, _, err := s.conn.ReadFrom(buf)
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return nil
			}
			return err
		}
		s.processMessage(buf[:n])
	}
}

// processMessage parses and handles a syslog message
func (s *syslogd) processMessage(data []byte) {
	parser := rfc3164.NewParser(rfc3164.WithYear(rfc3164.CurrentYear{}), rfc3164.WithBestEffort())
	msg, err := parser.Parse(data)
	if err != nil {
		log.Printf("Failed to parse message: %v (raw: %s)\n", err, string(data))
		return
	}
	rmsg, ok := msg.(*rfc3164.SyslogMessage)
	if !ok {
		log.Println("Not a rfc3164 message")
		return
	}
	kubemsg := message{
		Timestamp: rmsg.Timestamp,
		User:      s.user,
		Message:   *rmsg.Message,
		Appname:   *rmsg.Appname,
		Level:     *msg.SeverityShortLevel(),
	}
	for _, dest := range s.receivers {
		dest.receive(kubemsg)
	}
}
