#!/bin/bash
#
# List or check that the golang version embedded in files (specifically our
# Dockerfile) is using a supported version of Go.
# Run with --show to list the images the script thinks are current.
# Run with --check (the default) to verify the Dockerfile contains a current
# image & version tag.
#

set -e
SCRIPT_DIR="$(readlink -f "$(dirname "${0}")")"
PROJECT_DIR="$(readlink -f "${SCRIPT_DIR}/..")"
WORKDIR="$(mktemp -d)"
CONTAINER_FILES=("${PROJECT_DIR}/Dockerfile")
GOVERSURL='https://go.dev/dl/?mode=json'
ACTION=check


fetch_versions() {
    local url="$1"
    local out="$2"
    if [ -f "${out}" ]; then
        return
    fi
    echo "Fetching Go versions..." >&2
    curl --fail -sL -o "${out}" "${url}"
}

extract_versions() {
    local jsonfile="$1"
    jq -r '.[].version' < "${jsonfile}" | \
        cut -d. -f1,2 | sort -u | sed -e 's,^go,,'
}

to_images() {
    for a in "$@"; do
        echo "docker.io/golang:${a}"
    done
}

cleanup() {
    # only used in trap, ignoe unreachable error
    # shellcheck disable=SC2317
    rm -rf "${WORKDIR}"
}
trap cleanup EXIT


opts=$(getopt --name "$0" \
    -o "csf:" -l "check,show,file:,url:,versionsfile:" -- "$@")
eval set -- "$opts"

cli_cfiles=()
while true ; do
    case "$1" in
        -c|--check)
            ACTION=check
            shift
        ;;
        -s|--show)
            ACTION=show
            shift
        ;;
        -f|--file)
            cli_cfiles+=("$2")
            shift 2
        ;;
        --url)
            GOVERSURL="$2"
            shift 2
        ;;
        --versionsfile)
            GOVERSIONSFILE="$2"
            shift 2
        ;;
        --)
            shift
            break
        ;;
        *)
            echo "unexpected option: $1" >&2
            exit 2
        ;;
    esac
done

if [ "${#cli_cfiles}" -gt 0 ]; then
    CONTAINER_FILES=("${cli_cfiles[@]}")
fi

GV="${GOVERSIONSFILE:-${WORKDIR}/go-versions.json}"
fetch_versions "${GOVERSURL}" "${GV}"
mapfile -t go_major_vers < <(extract_versions "${GV}")

echo "Found Go versions:" "${go_major_vers[@]}" >&2
if [ "${ACTION}" = show ]; then
    to_images "${go_major_vers[@]}"
    exit 1
fi

mapfile -t valid_images < <(to_images "${go_major_vers[@]}")
errors=0
for cf in "${CONTAINER_FILES[@]}"; do
    echo "Checking $cf ..." >&2
    if grep -q -e "${valid_images[0]}" -e "${valid_images[1]}" "${cf}" ; then
        echo "${cf}: OK" >&2
    else
        echo "${cf}: no current golang image found" >&2
        errors=1
    fi
done

exit "${errors}"
