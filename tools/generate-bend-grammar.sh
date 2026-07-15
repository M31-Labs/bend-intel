#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
gotreesitter_dir=${GOTREESITTER_DIR:-/tmp/gotreesitter-src}

if [ ! -d "$gotreesitter_dir" ]; then
	printf '%s\n' "GOTREESITTER_DIR does not exist: $gotreesitter_dir" >&2
	exit 1
fi

cd "$gotreesitter_dir"

# Keep the runtime artifact byte-for-byte aligned with the C Tree-sitter
# witness. grammargen remains useful for inspecting grammar.json, but the
# parser-core reengineering boundary is validated against parser.c itself.
# ts2go extracts the C parse tables and scanner-election metadata into the
# pure-Go blob consumed by Bend Intel; no CGo is needed at runtime.
generated=$(mktemp -d)
trap 'rm -rf "$generated"' EXIT
go run ./cmd/ts2go \
	-input "$root/internal/cparity/c/parser.c" \
	-output "$generated/bend_generated.go" \
	-package bendgenerated \
	-name bend
cp "$generated/grammar_blobs/bend.bin" "$root/bendlang/grammar/bend.bin"

sha256sum "$root/bendlang/grammar/grammar.json" "$root/bendlang/grammar/bend.bin"
