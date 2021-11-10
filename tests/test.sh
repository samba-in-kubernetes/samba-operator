#!/bin/sh

set -e
cd "$(dirname "${0}")/.."

gtest() {
    go test -tags integration -v -count 1 "$@"
}

if [ "$SMBOP_TEST_RUN" ]; then
    gtest "-run" "$SMBOP_TEST_RUN" ./tests/integration/
else
    gtest ./tests/integration/
fi
