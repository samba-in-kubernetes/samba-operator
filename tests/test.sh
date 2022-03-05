#!/bin/sh

set -e
cd "$(dirname "${0}")/.."

gtest() {
    if [ "$SMBOP_TEST_CLUSTERED" ]; then
        go test -tags integration -v -count 1 -timeout 20m "$@"
    else
        go test -tags integration -v -count 1 "$@"
    fi
}

if [ "$SMBOP_TEST_RUN" ]; then
    gtest "-run" "$SMBOP_TEST_RUN" ./tests/integration/
else
    gtest ./tests/integration/
fi
