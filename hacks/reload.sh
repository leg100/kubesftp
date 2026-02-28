#!/usr/bin/env bash

set -e

# build and load docker image into kind
make load

helm upgrade -i sftp ./charts/kubesftp/ --set podAnnotations.time="$(date +\"%s\")"
