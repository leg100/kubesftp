FROM golang:alpine3.23
WORKDIR /src

RUN --mount=type=cache,target=/etc/apk/cache \
  apk add --no-cache --upgrade make

# Unused files/folders are filtered out with the .dockerignore file.
COPY . .

# Compile the binaries
RUN --mount=type=cache,target=/go/pkg/mod \
  --mount=type=cache,target=/root/.cache/go-build \
  make build

FROM alpine:3.23.3

RUN --mount=type=cache,target=/etc/apk/cache \
  apk add --no-cache --upgrade openssh gcompat

USER root

COPY entrypoint.sh /
COPY --from=0 /src/_build/linux/amd64/generator /usr/local/bin/

ENTRYPOINT ["/entrypoint.sh"]
