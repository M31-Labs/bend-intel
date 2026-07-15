#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
gotreesitter_dir=${GOTREESITTER_DIR:-/tmp/gotreesitter-src}

if [ ! -d "$gotreesitter_dir" ]; then
	printf '%s\n' "GOTREESITTER_DIR does not exist: $gotreesitter_dir" >&2
	exit 1
fi

cd "$gotreesitter_dir"
go run ./cmd/grammargen emit \
	-json "$root/bendlang/grammar/grammar.json" \
	-bin "$root/bendlang/grammar/bend.bin"

sha256sum "$root/bendlang/grammar/grammar.json" "$root/bendlang/grammar/bend.bin"
