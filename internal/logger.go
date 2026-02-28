package internal

import (
	"fmt"
	"io"
)

// Logger logs messages from sftpd to an output device (typically stdout).
type Logger struct {
	Out io.Writer
}

func (l *Logger) receive(msg message) {
	fmt.Fprintf(l.Out, "%v %s %v: %s\n", msg.Timestamp, msg.Appname, msg.User, msg.Message)
}
