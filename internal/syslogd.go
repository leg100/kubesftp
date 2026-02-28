package internal

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"time"

	"github.com/leodido/go-syslog/v4/rfc3164"
	"golang.org/x/sync/errgroup"
)

// syslogd aggregates and relays syslog messages from a user's chroot.
type syslogd struct {
	user      user
	receivers []syslogdReceiver
	listener  net.Listener
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
	// Remove existing socket file if it exists
	if err := os.RemoveAll(user.devLogPath(chrootsParentDir)); err != nil {
		return nil, fmt.Errorf("failed to remove existing socket: %w", err)
	}
	listener, err := net.Listen("unix", user.devLogPath(chrootsParentDir))
	if err != nil {
		return nil, err
	}
	if err := os.Chmod(user.devLogPath(chrootsParentDir), 0o666); err != nil {
		return nil, err
	}
	return &syslogd{
		user:      user,
		receivers: dests,
		listener:  listener,
	}, nil
}

func (s *syslogd) accept(ctx context.Context, g *errgroup.Group) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			conn, err := s.listener.Accept()
			if err != nil {
				if ctx.Err() != nil {
					return nil
				}
				return err
			}

			g.Go(func() error {
				s.handleUnix(ctx, conn)
				return nil
			})
		}
	}
}

// handleUnix handles a Unix socket connection
func (s *syslogd) handleUnix(ctx context.Context, conn net.Conn) {
	//defer s.wg.Done()
	defer conn.Close()

	reader := bufio.NewScanner(conn)
	reader.Split(func(data []byte, atEOF bool) (advance int, token []byte, err error) {
		if atEOF && len(data) == 0 {
			return 0, nil, nil
		}
		if i := bytes.IndexByte(data, 0); i >= 0 {
			// We have a full null-terminated line.
			return i + 1, dropNull(data[0:i]), nil
		}
		// If we're at EOF, we have a final, non-terminated line. Return it.
		if atEOF {
			return len(data), dropNull(data), nil
		}
		// Request more data.
		return 0, nil, nil
	})

	for reader.Scan() {
		if err := reader.Err(); err != nil {
			log.Printf("received reader err: %v\n", err)
			return
		}
		//log.Printf("received line: %v\n", reader.Text())

		s.processMessage(reader.Bytes())
	}
}

// dropNull drops a terminal NULL from the data.
func dropNull(data []byte) []byte {
	if len(data) > 0 && data[len(data)-1] == 0 {
		return data[0 : len(data)-1]
	}
	return data
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
