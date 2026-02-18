FROM alpine:3.23.3

RUN --mount=type=cache,target=/etc/apk/cache \
  apk add --no-cache --upgrade openssh

# todo: write entrypoint script that sets ulimit
ENTRYPOINT ["/usr/sbin/sshd", "-D", "-e"]
