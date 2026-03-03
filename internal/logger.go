package internal

import (
	"log/slog"
)

// Logger logs messages from sftpd
type Logger struct {
	*slog.Logger
}

func (l *Logger) receive(msg message) {
	l.Logger.Info(msg.Message, "user", msg.User, "program", msg.Appname)
}
