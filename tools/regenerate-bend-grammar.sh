#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
source_dir="$root/bendlang/grammar/source"
tree_sitter=${TREE_SITTER:-tree-sitter}

cd "$source_dir"
"$tree_sitter" generate --js-runtime native
cp src/grammar.json "$root/bendlang/grammar/grammar.json"
cp src/parser.c "$root/internal/cparity/c/parser.c"
cd "$root"
GOTREESITTER_DIR=${GOTREESITTER_DIR:-/tmp/gotreesitter-src} ./tools/generate-bend-grammar.sh
