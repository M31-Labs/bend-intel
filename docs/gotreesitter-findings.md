# gotreesitter findings from Bend

This repository is a downstream exercise for the gotreesitter parser core. It
does not fork the runtime. Instead it records small fixtures, exposes parse
health to editor clients, and keeps compatibility shims at the language
boundary while the parser is being reengineered.

## Silent stopped trees

The most important finding is that a pure-Go parse can return a plausible
`source_file` prefix with:

```text
root.HasError()       == true
Tree.ParseStopReason() == no_stacks_alive
root.EndByte()        < len(source)
no named ERROR node in the visible subtree
```

The minimal regression fixture is in
`intel/document_test.go` (`TestParseHealthCatchesSilentNoStacksPrefix`): a
typed function matching `List/Nil` and returning `List/Nil`. Counting only
named `ERROR` nodes can make the prefix appear clean; `Document.Complete`,
`Document.Health`, and the corpus report reject that false green result. The
current core returns an error-bearing accepted tree for this fixture, so the
health check deliberately considers both stop metadata and root error state.

## Recovery boundaries

Recovery candidates are always parsed with a fresh parser. Reusing a parser
after a failed hypothesis can truncate the next top-level definition because
recovery checkpoints survive the first parse. The regression fixture is
`TestCurrentTypedBendRecoveryKeepsLaterDefinitions`.

The final editor fallback for a stopped, path-heavy body keeps each top-level
`def` header and masks the body with one placeholder statement. It preserves
all source offsets and marks the resulting document `Recovered()`. This is
structural navigation support, not semantic validation; Bend checks the
original bytes through the optional sidecar.

## Stateful scanner and comments

Bend's external scanner carries an indentation stack, serializes it for
incremental parsing, emits comments, recognizes `#<digits>` natural literals,
and elects slash-terminated path tokens. A comment immediately after a
recovered indentation block can prevent a later top-level definition from
materializing, so the compatibility layer masks only comment bytes in a fresh
candidate and re-adds comment spans to semantic tokens from the original
source.

The scanner tests cover nested indentation, multiple dedents, blank and
comment-only lines, tabs, EOF, serialization round trips, path tokens, natural
literals, and edits across indentation boundaries.

## C/Go witness policy

`internal/cparity` compares the C Tree-sitter runtime with gotreesitter,
including named shape, fields, ranges, missing/extra/error flags, and byte
metadata. On the current Bend examples, all 16 trees are exact. Two fixes were
needed at the parser boundary: repeated zero-width Bend dedents are now
allowed in gotreesitter, and `ts2go` decodes Tree-sitter universal-character
escapes in symbol names. The Bend grammar also gives adjacent parenthesized
functional patterns an explicit dynamic precedence so both runtimes choose the
same branch.

The strict parser-core gate in `tools/validate-roadmap.sh` now passes for the
current examples. The default LSP validation still accepts only complete,
diagnostics-free structural views and keeps raw health visible for negative and
incomplete fixtures.

The grammar side of the boundary also records two ambiguity fixes exercised by
the compiler's functional fixtures: adjacent parenthesized patterns receive a
dynamic precedence, and inline `let` continuations use a left-associative,
optional-newline rule so nested lambdas can terminate at `)` or EOF. Those
fixtures now produce clean C and Go trees; remaining mixed-corpus differences
are primarily negative or historical recovery cases, plus a small set of
compiler fixtures outside the maintained release smoke corpus.

These findings should become upstream gotreesitter regression tests or scanner
diagnostics when the corresponding parser-core interfaces settle.
