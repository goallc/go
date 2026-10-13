#!/usr/bin/env bash
# Run ordinary standard-library tests through each LTO path without exclusions.
set -euo pipefail
mode=${1:?usage: goallc-lto-stdlib.sh full|thin}
case "$mode" in full|thin) ;; *) echo "invalid LTO mode: $mode" >&2; exit 2 ;; esac
export CGO_ENABLED=0
export GOMAXPROCS=${GOMAXPROCS:-2}
packages=(bytes strings strconv sort encoding/binary encoding/json hash/crc32 unicode/utf8)
"${GOROOT:?}/bin/go" test -lto="$mode" -short -count=1 -p=1 -timeout=20m -json "${packages[@]}"
