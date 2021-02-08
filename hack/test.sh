#!/usr/bin/env bash

ENVTEST_ASSETS_DIR="$(pwd)/testbin"

mkdir -p "${ENVTEST_ASSETS_DIR}"

test -f "${ENVTEST_ASSETS_DIR}/setup-envtest.sh" || \
	curl -sSLo "${ENVTEST_ASSETS_DIR}/setup-envtest.sh" \
	https://raw.githubusercontent.com/kubernetes-sigs/controller-runtime/v0.6.3/hack/setup-envtest.sh

. "${ENVTEST_ASSETS_DIR}/setup-envtest.sh"

fetch_envtest_tools "${ENVTEST_ASSETS_DIR}"
setup_envtest_env "${ENVTEST_ASSETS_DIR}"

go test ./... -coverprofile cover.out
