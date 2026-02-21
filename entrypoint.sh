#!/bin/sh

ulimit -n 65535

exec /usr/sbin/sshd -D -e
