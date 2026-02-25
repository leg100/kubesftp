package internal

import "github.com/leodido/go-syslog/v4/rfc5424"

const sftpLogs1 = `<38>Feb 23 20:31:22 sshd-session[2237381]: Changed root directory to "/srv/sftp/jail/alice"`
const sftpLogs2 = `<38>Feb 23 20:31:22 sshd-session [2237381]: Starting session: subsystem 'sftp' for alice from ::1 port 51234 id 0`
const sftpLogs3 = `<38>Feb 23 20:31:22 internal-sftp[2237382]: session opened for local user alice from [::1]`
const sftpLogs4 = `<38>Feb 23 20:31:22 internal-sftp[2237382]: received client version 3`
const sftpLogs5 = `<38>Feb 23 20:31:22 internal-sftp[2237382]: realpath "."`

func newLogger() {
	i := []byte(`<165>4 2018-10-11T22:14:15.003Z mymach.it e - 1 [ex@32473 iut="3"] An application event log entry...`)
	p := rfc5424.NewParser()
	_, _ = p.Parse(i)
}
