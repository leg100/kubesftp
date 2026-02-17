FROM golang:1.24-alpine AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o /kubesftp .

FROM alpine:3.23.3

RUN --mount=type=cache,target=/etc/apk/cache \
  apk add --no-cache --upgrade openssh

RUN ssh-keygen -A
ADD sshd_config /etc/ssh/

COPY --from=builder /kubesftp /usr/local/bin/kubesftp

ENTRYPOINT ["/usr/sbin/sshd", "-D", "-e"]
