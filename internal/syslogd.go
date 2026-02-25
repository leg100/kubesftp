package internal

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"os"

	"github.com/leodido/go-syslog/v4/rfc3164"
)

const sftpLogs1 = `<38>Feb 23 20:31:22 sshd-session[2237381]: Changed root directory to "/srv/sftp/jail/alice"`
const sftpLogs2 = `<38>Feb 23 20:31:22 sshd-session [2237381]: Starting session: subsystem 'sftp' for alice from ::1 port 51234 id 0`
const sftpLogs3 = `<38>Feb 23 20:31:22 internal-sftp[2237382]: session opened for local user alice from [::1]`
const sftpLogs4 = `<38>Feb 23 20:31:22 internal-sftp[2237382]: received client version 3`
const sftpLogs5 = `<38>Feb 23 20:31:22 internal-sftp[2237382]: realpath "."`

//i := []byte(`<165>4 2018-10-11T22:14:15.003Z mymach.it e - 1 [ex@32473 iut="3"] An application event log entry...`)
//p := rfc5424.NewParser()
//_, _ = p.Parse(i)

// syslogd aggregates and relays syslog messages from a user's chroot.
type syslogd struct {
	user     user
	dests    []io.Writer
	listener net.Listener
}

func newSyslogd(user user, dests ...io.Writer) (*syslogd, error) {
	// Remove existing socket file if it exists
	if err := os.RemoveAll(user.devLogPath()); err != nil {
		return nil, fmt.Errorf("failed to remove existing socket: %w", err)
	}
	listener, err := net.Listen("unix", user.devLogPath())
	if err != nil {
		return nil, err
	}
	return &syslogd{
		user:     user,
		dests:    dests,
		listener: listener,
	}, nil
}

func (s *syslogd) accept(ctx context.Context) error {
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
				// Check if listener is closed
				//if isClosedError(err) {
				//	return nil
				//}
				log.Printf("Unix socket accept error: %v", err)
				return nil
			}

			//s.wg.Add(1)
			go s.handleUnix(ctx, conn)
		}
	}
}

// handleUnix handles a Unix socket connection
func (s *syslogd) handleUnix(ctx context.Context, conn net.Conn) {
	//defer s.wg.Done()
	defer conn.Close()

	reader := bufio.NewReader(conn)
	for {
		select {
		case <-ctx.Done():
			return
		default:
			line, err := reader.ReadBytes('\n')
			if err != nil {
				if err != io.EOF {
					log.Printf("Unix socket read error: %v", err)
				}
				return
			}

			if len(line) > 0 {
				s.processMessage(line)
			}
		}
	}
}

// processMessage parses and handles a syslog message
func (s *syslogd) processMessage(data []byte) {
	parser := rfc3164.NewParser(rfc3164.WithYear(rfc3164.CurrentYear{}))
	msg, err := parser.Parse(data)
	if err != nil {
		log.Printf("Failed to parse message: %v (raw: %s)", err, string(data))
		return
	}
	rmsg, ok := msg.(*rfc3164.SyslogMessage)
	if !ok {
		return
	}
	for _, dest := range s.dests {
		fmt.Fprintf(dest, "%v %s %v: %s", rmsg.Timestamp, *rmsg.Appname, s.user, *rmsg.Message)
	}
}
