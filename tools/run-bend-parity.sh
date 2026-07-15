#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
path=${1:-.}
cd "$root"
go test -tags='cgo parity' ./internal/cparity >&2
go run -tags='cgo parity' ./cmd/bend-parity -path "$path"
