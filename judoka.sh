#!/bin/bash
set -e
set -o pipefail
here=$(dirname "$(readlink -f "$BASH_SOURCE")")
export PEBBLE_LOG_FORMAT="console"
appfile=$(mktemp)
cleanup() {
    rv=$?
    rm -f -- "$appfile"
    exit $rv
}
trap 'cleanup' EXIT
cd "$here/cli/"
go build -buildvcs=true -tags $(cat "$here/tags" | tr -d "\n") -o "$appfile"
cd - &> /dev/null
"$appfile" --logFormat console "$@"
