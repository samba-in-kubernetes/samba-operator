#!/usr/bin/env bash
export LC_ALL=C
unset CDPATH
ARCH=${1:-amd64}

# Currently, aupport amd64 (x86_64) and arm64 (aarch64) architectures
case "${ARCH}" in "amd64") ;; "arm64") ;; \
	*) echo "illegal ${ARCH}" && exit 1 ;; esac

# Prerequisites checks
_require_command() {
	command -v "$1" > /dev/null || (echo "missing: $1" && exit 1)
}

_require_command realpath
_require_command git
_require_command podman
_require_command qemu-x86_64-static
_require_command qemu-aarch64-static

# Fail on error
set -o errexit
set -o nounset
set -o pipefail

# Create tar-ball using git
BASEDIR=$(realpath "$(dirname "${BASH_SOURCE[0]}")/../")
cd "${BASEDIR}"
git archive --format tar.gz HEAD > hack/samba-operator.tar.gz
function cleanup_on_exit() { rm -f hack/samba-operator.tar.gz; }
trap cleanup_on_exit EXIT

# Build within container using podman
podman build \
  --arch=${ARCH} \
  --tag localhost/samba-operator-build:${ARCH} \
  --file hack/Dockerfile.build
