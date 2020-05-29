#!/usr/bin/env bash

set -e

RELEASE_VERSION="v0.17.1"
SDK_BASE="operator-sdk-${RELEASE_VERSION}-x86_64-linux-gnu"
ASC_BASE="${SDK_BASE}.asc"
URL_BASE="https://github.com/operator-framework/operator-sdk/releases/download/${RELEASE_VERSION}/"
URL_SDK="${URL_BASE}/${SDK_BASE}"
URL_ASC="${URL_SDK}.asc"
TARGET_DIR="/usr/local/bin"
TARGET="${TARGET_DIR}/operator-sdk"

curl -LO "${URL_SDK}"
curl -LO "${URL_ASC}"

gpg --verify "${ASC_BASE}"

chmod +x "${SDK_BASE}"

sudo cp "${SDK_BASE}" "${TARGET_DIR}"
sudo ln -fs "${SDK_BASE}" "${TARGET}"
