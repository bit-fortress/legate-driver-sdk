#!/bin/sh
set -eu

if [ "$#" -ne 1 ]; then
  echo "usage: $0 driver.wasm" >&2
  exit 2
fi

module=$1
wabt_version=1.0.41

for tool in wasm2wat wat2wasm; do
  if ! command -v "$tool" >/dev/null 2>&1; then
    echo "$tool $wabt_version is required" >&2
    exit 1
  fi
  actual_version=$($tool --version)
  if [ "$actual_version" != "$wabt_version" ]; then
    echo "$tool version $actual_version is unsupported; want $wabt_version" >&2
    exit 1
  fi
done

wat=$(mktemp)
normalized_wat=$(mktemp)
normalized_wasm=$(mktemp)
trap 'rm -f "$wat" "$normalized_wat" "$normalized_wasm"' EXIT HUP INT TERM

wasm2wat "$module" -o "$wat"
seek_exports=$(grep -c '^[[:space:]]*(export "syscall\.seek" ' "$wat" || true)
if [ "$seek_exports" -ne 1 ]; then
  echo "$module must contain exactly one TinyGo 0.39.0 syscall.seek export before normalization; found $seek_exports" >&2
  exit 1
fi

sed '/^[[:space:]]*(export "syscall\.seek" /d' "$wat" >"$normalized_wat"
wat2wasm "$normalized_wat" -o "$normalized_wasm"
mv "$normalized_wasm" "$module"
