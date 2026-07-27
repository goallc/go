#!/usr/bin/env bash

set -euo pipefail

script_dir="$(cd "$(dirname "$0")" && pwd -P)"

echo "gen_llvm_config_static.sh is deprecated; use gen_llvm_config.sh --link static" >&2
exec "${script_dir}/gen_llvm_config.sh" "$@" --link static
