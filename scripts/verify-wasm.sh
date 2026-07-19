#!/bin/sh
set -eu

if [ "$#" -ne 1 ]; then
  echo "usage: $0 driver.wasm" >&2
  exit 2
fi

module=$1
dump=$(wasm-objdump -x "$module")

imports=$(printf '%s\n' "$dump" | sed -n '/^Import\[/,/^[A-Z][A-Za-z]*\[/p' | grep -c '^ - ' || true)
if [ "$imports" -ne 0 ]; then
  echo "$module imports host functions or memories" >&2
  exit 1
fi

if printf '%s\n' "$dump" | grep -q -- '-> "legate_image_attempt_open_v1"'; then
  kind=image
  hooks='legate_bind_v1 legate_image_attempt_open_v1 legate_image_transform_buffered_response_v1 legate_image_attempt_close_v1'
  expected_exports=7
else
  kind=text
  hooks='legate_bind_v1 legate_text_attempt_open_v1 legate_text_transform_buffered_response_v1 legate_text_sse_open_v1 legate_text_sse_transform_event_v1 legate_text_sse_finish_v1 legate_text_attempt_close_v1'
  expected_exports=10
fi

export_count=$(printf '%s\n' "$dump" | sed -n '/^Export\[/,/^[A-Z][A-Za-z]*\[/p' | grep -c '^ - ' || true)
if [ "$export_count" -ne "$expected_exports" ] && [ "$export_count" -ne $((expected_exports + 1)) ]; then
  echo "$module has an invalid $kind ABI export count: $export_count" >&2
  exit 1
fi

memory_exports=$(printf '%s\n' "$dump" | sed -n '/^Export\[/,/^[A-Z][A-Za-z]*\[/p' | grep -c -- '^ - memory\[[0-9][0-9]*\] -> "memory"$' || true)
if [ "$memory_exports" -ne 1 ]; then
  echo "$module must export exactly one memory, found $memory_exports" >&2
  exit 1
fi

for export in legate_alloc_v1 legate_free_v1 $hooks
do
  if ! printf '%s\n' "$dump" | grep -q -- "-> \"${export}\""; then
    echo "$module is missing export $export" >&2
    exit 1
  fi
done

actual_exports=$(printf '%s\n' "$dump" | sed -n '/^Export\[/,/^[A-Z][A-Za-z]*\[/s/.*-> "\([^"]*\)".*/\1/p')
for actual_export in $actual_exports; do
  case "$actual_export" in
    memory|_initialize|legate_alloc_v1|legate_free_v1|legate_bind_v1)
      ;;
    legate_text_attempt_open_v1|legate_text_transform_buffered_response_v1|legate_text_sse_open_v1|legate_text_sse_transform_event_v1|legate_text_sse_finish_v1|legate_text_attempt_close_v1)
      [ "$kind" = text ] || { echo "$module exports a text hook from an image module" >&2; exit 1; }
      ;;
    legate_image_attempt_open_v1|legate_image_transform_buffered_response_v1|legate_image_attempt_close_v1)
      [ "$kind" = image ] || { echo "$module exports an image hook from a text module" >&2; exit 1; }
      ;;
    *)
      echo "$module exports unsupported symbol $actual_export" >&2
      exit 1
      ;;
  esac
done

assert_signature() {
  symbol=$1
  expected=$2
  signature=$(printf '%s\n' "$dump" | sed -n "s/^ - func\[[0-9][0-9]*\] sig=\([0-9][0-9]*\) <${symbol}>$/\1/p" | head -n 1)
  if [ -z "$signature" ]; then
    echo "$module cannot resolve function signature for $symbol" >&2
    exit 1
  fi
  actual=$(printf '%s\n' "$dump" | sed -n "s/^ - type\[${signature}\] //p" | head -n 1)
  if [ "$actual" != "$expected" ]; then
    echo "$module export $symbol has signature $actual, want $expected" >&2
    exit 1
  fi
}

assert_signature legate_alloc_v1 '(i32) -> i32'
assert_signature legate_free_v1 '(i32, i32) -> nil'
initializer_count=$(printf '%s\n' "$dump" | sed -n '/^Export\[/,/^[A-Z][A-Za-z]*\[/p' | grep -c -- ' -> "_initialize"$' || true)
if [ "$initializer_count" -gt 1 ]; then
  echo "$module exports _initialize more than once" >&2
  exit 1
fi
if [ "$initializer_count" -eq 1 ]; then
  assert_signature _initialize '() -> nil'
fi
for hook in $hooks
do
  assert_signature "$hook" '(i32, i32) -> i64'
done

echo "$module: exact ABI export set present and no imports detected"
