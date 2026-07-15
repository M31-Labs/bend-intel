# Grammar provenance

- Source: `https://github.com/HigherOrderCO-archive/tree-sitter-bend`
- Commit: `7e7b88b77103e4d9c11a68d719be8702e4d6ad7f`
- License: MIT
- Resolved grammar: `bendlang/grammar/grammar.json` (vendored from the same commit)
- External scanner: behaviorally ported from `src/scanner.c` to pure Go.

Regenerate the blob from the vendored `grammar.json` with a current gotreesitter
checkout. The generated artifact is the grammar package's runtime source; the
scanner is attached by `bendlang.AttachExternalScanner`.

```sh
GOTREESITTER_DIR=/path/to/gotreesitter \
  ./tools/generate-bend-grammar.sh
```

The script records SHA-256 hashes so generated changes are reviewable. The
archived `parser.c` remains in `internal/cparity/c/` as the C-runtime witness;
it is not a consumer dependency.
