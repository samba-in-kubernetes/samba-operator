#!/bin/bash

set -e

source hack/operator-sdk-common.sh

if [ -x "${OPERATOR_SDK}" ]; then
	if "${OPERATOR_SDK}" version | grep -q "\"${OPERATOR_SDK_VERSION}\"" ; then
		echo "Using operator-sdk cached at ${OPERATOR_SDK}"
		exit 0
	else
		echo "operator sdk cached at ${OPERATOR_SDK} does not report the desired version - updating"
	fi
fi

echo "Downloading operator-sdk ${OPERATOR_SDK_VERSION}-${OPERATOR_SDK_PLATFORM}"
mkdir -p "$(dirname ${OPERATOR_SDK})"
curl -JL "${OPERATOR_SDK_URL}/${OPERATOR_SDK_VERSION}/${OPERATOR_SDK_BIN}" -o "${OPERATOR_SDK}"
chmod +x "${OPERATOR_SDK}"
