#!/usr/bin/env bash
# Run complete short standard-library suites through FullLTO without exclusions.
set -euo pipefail
export CGO_ENABLED=0
export GOMAXPROCS=${GOMAXPROCS:-2}
packages=(bytes strings strconv sort encoding/binary encoding/json hash/crc32 unicode/utf8)
"${GOROOT:?}/bin/go" test -lto=full -short -count=1 -p=1 -timeout=20m -json "${packages[@]}"
