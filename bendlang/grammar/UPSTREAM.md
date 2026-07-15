# Grammar provenance and regeneration

The starting point is the MIT-licensed
[`HigherOrderCO-archive/tree-sitter-bend`](https://github.com/HigherOrderCO-archive/tree-sitter-bend)
commit `7e7b88b77103e4d9c11a68d719be8702e4d6ad7f`. The compiler snapshot used
for the current compatibility report is
`HigherOrderCO/Bend@814453670d0e0d6777c1313c972764dba0491b7f` (0.2.38).

`bendlang/grammar/source/` is the maintained grammar source used to generate
the vendored `grammar.json`, parser witness, and gotreesitter `bend.bin`. It
is intentionally a small downstream fork, not a claim that Tree-sitter is the
language authority. Current changes include typed imperative parameters and
returns, current type applications, underscore/path identifiers, functional
surface fixtures, and the `#<digits>` natural-literal external token. The Go
scanner is a behavioral port of the upstream C scanner; the C scanner remains
under `internal/cparity/c/` for the optional witness.

Current generated hashes:

```text
grammar.json  bcd83776a0a9c43981000fb9a202c9f530105529936ae5a7dbcdf31571916148
bend.bin      e9fb687cf855c6d3c23a6de52fe4045938e41cd462a2c28405882c40ef339ece
```

The runtime blob is generated from the C witness with
`github.com/odvcencio/gotreesitter` commit
`a340d23e3b8e3addf0ecd9e13e882e42ff8e58bc`. This keeps the downstream
artifact byte-for-byte aligned with the parser tables while gotreesitter's
parser core is being reengineered.

## Regeneration

The source grammar needs the Tree-sitter CLI with the native JavaScript
runtime. From the repository root:

```sh
cd bendlang/grammar/source
tree-sitter generate --js-runtime native
cd ../../..
cp bendlang/grammar/source/src/grammar.json bendlang/grammar/grammar.json
cp bendlang/grammar/source/src/parser.c internal/cparity/c/parser.c
GOTREESITTER_DIR=/path/to/gotreesitter ./tools/generate-bend-grammar.sh
```

`tools/generate-bend-grammar.sh` extracts the generated C parser tables with
the pinned `ts2go` tool and embeds the deterministic runtime blob. Consumers
only load the blob and the attached Go scanner; they do not need the grammar
generator, Tree-sitter C runtime, CGo, or Node.js.

Any grammar update should be accompanied by:

1. current Bend examples and relevant compiler fixtures;
2. Go scanner and incremental-edit tests;
3. the opt-in C/Go parity report;
4. the strict `tools/validate-roadmap.sh --strict-parser-core` gate.
