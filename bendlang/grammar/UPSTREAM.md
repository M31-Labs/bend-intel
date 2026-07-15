# Grammar provenance

- Source: `https://github.com/HigherOrderCO-archive/tree-sitter-bend`
- Commit: `7e7b88b77103e4d9c11a68d719be8702e4d6ad7f`
- License: MIT
- External scanner: behaviorally ported from `src/scanner.c` to pure Go.

Regenerate the blob with a current gotreesitter checkout:

```sh
go run ./cmd/ts2go \
  -input /path/to/tree-sitter-bend/src/parser.c \
  -name bend \
  -output /tmp/bend_generated.go \
  -package bendlang
cp /tmp/grammar_blobs/bend.bin bendlang/grammar/bend.bin
```

The generated Go loader is intentionally replaced by `bendlang/language.go`, which attaches the Go external scanner.
