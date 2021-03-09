#!/bin/sh

set -e
cd "$(dirname "${0}")/.."

go test -tags integration -v -count 1 ./tests/integration/
