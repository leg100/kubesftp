package internal

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"time"

	"github.com/leodido/go-syslog/v4/rfc3164"
	"golang.org/x/sync/errgroup"
)

// syslogd aggregates and relays syslog messages from a user's chroot.
type syslogd struct {
	user      user
	receivers []receiver
	conn      *net.UnixConn
	logger    *slog.Logger
}

type receiver interface {
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

func StartSyslogDaemons(
	ctx context.Context,
	logger *slog.Logger,
	g *errgroup.Group,
	cfg config,
	receivers ...receiver,
) error {
	for _, user := range cfg.Users {
		d, err := newSyslogd(cfg.ChrootsDir, logger, user, receivers...)
		if err != nil {
			return err
		}
		g.Go(func() error {
			return d.connect(ctx)
		})
		g.Go(func() error {
			<-ctx.Done()
			return d.conn.Close()
		})
	}
	logger.Info("started syslog daemons", "num_users", len(cfg.Users))
	return nil
}

func newSyslogd(
	chrootsDir string,
	logger *slog.Logger,
	user user,
	dests ...receiver,
) (*syslogd, error) {
	socket := user.devLogPath(chrootsDir)
	// Remove existing socket file if it exists
	if err := os.RemoveAll(socket); err != nil {
		return nil, fmt.Errorf("removing existing socket: %w", err)
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
		logger:    logger,
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
		if err := s.processMessage(buf[:n]); err != nil {
			s.logger.Error("processing syslog message", "error", err)
		}
	}
}

// processMessage parses and handles a syslog message
func (s *syslogd) processMessage(data []byte) error {
	parser := rfc3164.NewParser(rfc3164.WithYear(rfc3164.CurrentYear{}), rfc3164.WithBestEffort())
	msg, err := parser.Parse(data)
	if err != nil {
		return fmt.Errorf("parsing message: %w", err)
	}
	rmsg, ok := msg.(*rfc3164.SyslogMessage)
	if !ok {
		return fmt.Errorf("received non-rfc3164 compliant message")
	}
	kubemsg := message{
		Timestamp: rmsg.Timestamp,
		User:      s.user,
	}
	if rmsg.Message != nil {
		kubemsg.Message = *rmsg.Message
	}
	if rmsg.Appname != nil {
		kubemsg.Appname = *rmsg.Appname
	}
	if rmsg.SeverityShortLevel() != nil {
		kubemsg.Level = *msg.SeverityShortLevel()
	}
	for _, dest := range s.receivers {
		dest.receive(kubemsg)
	}
	return nil
}
