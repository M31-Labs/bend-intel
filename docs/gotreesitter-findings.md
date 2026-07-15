# gotreesitter findings from Bend

This repository is intentionally a downstream exercise for the gotreesitter
parser core. The Bend package does not fork the runtime; it records failures as
small, reproducible fixtures and keeps the compatibility shims at the language
boundary.

## Candidate-parser state

When a parser has just produced an error tree, reparsing a masked candidate on
the same parser can truncate later top-level definitions after an indentation
block. `intel` therefore creates a fresh parser for every full recovery
candidate. The regression fixture is
`TestCurrentTypedBendRecoveryKeepsLaterDefinitions`.

This is a useful runtime contract for editor integrations: incremental parsing
can reuse an edited tree, but independent recovery hypotheses must not inherit
the failed parser's recovery checkpoints.

## Stateful scanner and comments

The Bend scanner carries indentation state and emits comments as external
tokens. In the archived grammar, a comment immediately after a recovered
indentation block can prevent the next top-level definition from materializing
in the pure-Go tree. The final recovery candidate masks only comment bytes,
preserving newlines and all source offsets. `Document.Recovered()` exposes that
the resulting CST is a structural view, and semantic tokens re-add comment
spans from the original source.

## Parity policy

`internal/cparity` is an opt-in C-runtime witness. It compares named tree
shape, fields, ranges, error/missing/extra flags, and byte metadata. A mismatch
is classified rather than turned into a passing parse. The current Bend
snapshot is recorded in `docs/compatibility.md`; the archived grammar and the
current compiler intentionally remain separate authorities until a grammar
update is reviewed.

These findings should become upstream gotreesitter regression tests or scanner
diagnostics when the parser core reengineering reaches the corresponding
interfaces.
