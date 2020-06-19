#!/bin/bash

OPERATOR_SDK_URL="${OPERATOR_SDK_URL:-https://github.com/operator-framework/operator-sdk/releases/download}"
OPERATOR_SDK_VERSION="${OPERATOR_SDK_VERSION:-v0.17.1}"
OPERATOR_SDK_PLATFORM="x86_64-linux-gnu"
OS_TYPE=$(uname)
if [ "$OS_TYPE" == "Darwin" ]; then
	OPERATOR_SDK_PLATFORM="x86_64-apple-darwin"
fi
OPERATOR_SDK_BIN="operator-sdk-${OPERATOR_SDK_VERSION}-${OPERATOR_SDK_PLATFORM}"
TOOLS_DIR="${TOOLS_DIR:-build/_tools}"
OPERATOR_SDK="${OPERATOR_SDK:-${TOOLS_DIR}/${OPERATOR_SDK_BIN}}"
