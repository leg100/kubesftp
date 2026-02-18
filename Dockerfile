FROM alpine:3.23.3

RUN --mount=type=cache,target=/etc/apk/cache \
  apk add --no-cache --upgrade openssh

RUN ulimit -n 65535

ENTRYPOINT ["/usr/sbin/sshd", "-D", "-e"]
