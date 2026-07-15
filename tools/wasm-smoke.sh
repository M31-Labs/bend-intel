#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
out=${TMPDIR:-/tmp}/bend-intel-wasm-smoke
rm -rf "$out"
mkdir -p "$out"
cd "$root"

env CGO_ENABLED=0 GOOS=js GOARCH=wasm go test -c -o "$out/bendlang.test.wasm" ./bendlang
env CGO_ENABLED=0 GOOS=js GOARCH=wasm go test -c -o "$out/intel.test.wasm" ./intel
env CGO_ENABLED=0 GOOS=js GOARCH=wasm go test -c -o "$out/lsp.test.wasm" ./lsp
env CGO_ENABLED=0 GOOS=js GOARCH=wasm go build ./cmd/bend-intel ./cmd/bendls

ls -lh "$out"/*.wasm
