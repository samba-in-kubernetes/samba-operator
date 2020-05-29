#!/usr/bin/env bash


SCRIPT_DIR="$(cd "$(dirname "${0}")" && pwd)"
BASE_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

GOPATH="$(cd "${BASE_DIR}/../.." && pwd)"
export GOPATH

GOROOT=$(go env GOROOT)
export GOROOT

#GOPROXY="https://proxy.golang.org,direct"
#export GOPROXY
