#!/bin/sh

set -e
SELFDIR="$(dirname "${0}")"
ROOTDIR="$(realpath "${SELFDIR}/../")"
LOCAL_BINDIR="${ROOTDIR}/.bin"

cd "${ROOTDIR}"
export PATH=${PATH}:${LOCAL_BINDIR}

gtest() {
    if [ "$SMBOP_TEST_CLUSTERED" ]; then
        go test -tags integration -v -count 1 -timeout 30m "$@"
    else
        go test -tags integration -v -count 1 -timeout 20m "$@"
    fi
}

if [ "$SMBOP_TEST_RUN" ]; then
    gtest "-run" "$SMBOP_TEST_RUN" ./tests/integration/
else
    gtest ./tests/integration/
fi
