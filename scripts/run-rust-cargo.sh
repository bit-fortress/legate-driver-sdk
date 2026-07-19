#!/bin/sh
set -eu

if command -v cargo >/dev/null 2>&1; then
  exec cargo "$@"
fi
if command -v rustup >/dev/null 2>&1; then
  cargo_path=$(rustup which cargo --toolchain 1.97.0)
  PATH="$(dirname -- "$cargo_path"):$PATH"
  export PATH
  exec "$cargo_path" "$@"
fi

echo "cargo or rustup with toolchain 1.97.0 is required" >&2
exit 1
