# Compatibility snapshot

Snapshot date: 2026-07-15. Source checkout: `HigherOrderCO/Bend` at
`814453670d0e0d6777c1313c972764dba0491b7f` (Bend 0.2.38). Grammar source:
the archived `tree-sitter-bend` commit recorded in
[`bendlang/grammar/UPSTREAM.md`](../bendlang/grammar/UPSTREAM.md).

This repository deliberately reports two different populations. The current
examples are a useful language sample; the golden-test tree contains both
valid and intentionally invalid fixtures and must not be presented as current
language coverage.

| Corpus | Files | Raw Go tree without `ERROR` | Intel recovery without diagnostics | C/Go exact shape | Shared C+Go error | Other parity classes |
|---|---:|---:|---:|---:|---:|---:|
| `examples/` | 16 | 2 | 11 | 1 | 14 | 1 tree mismatch |
| `tests/golden_tests/` | 508 | fixture-dependent | fixture-dependent | 295 | 140 | 8 C-only, 45 Go-only, 20 tree mismatch |

The archived grammar predates current typed signatures and other Bend 0.2.38
surface syntax. `intel` has a deliberately narrow, range-preserving recovery
view for typed headers, current shift expressions, `unchecked`/`const` markers,
constructor return paths, and the scanner's comment-after-indentation edge. It
does not rewrite the user's source or validate those semantics; Bend remains
the authority. The recovery view makes 11 examples structurally clean and
keeps lexical top-level symbols for all 16, while the five remaining examples
retain explicit diagnostics for older functional constructs. This is an
honest compatibility signal, not a claim that the archived grammar recognizes
all current Bend.

## Reproduce

Build the pure-Go checker and run the optional C-runtime witness:

```sh
go build -o /tmp/bend-intel ./cmd/bend-intel
go run ./cmd/bend-corpus-report -path /path/to/Bend/examples
./tools/run-bend-parity.sh /path/to/Bend/examples
./tools/run-bend-parity.sh /path/to/Bend/tests/golden_tests
```

The parity command emits one JSON record per file with `equal`, `cError`, and
`goError` fields. A mismatch is actionable evidence for grammar/runtime work;
it is not silently converted into a successful parse.

`bend-corpus-report` emits raw parser error counts, recovery use, structural
diagnostic counts, and symbol counts. It is intentionally separate from the
C-runtime witness so the report remains available in pure-Go builds.

## Current-language boundary

The grammar package currently follows the archived Tree-sitter grammar rather
than inventing a second Bend dialect. Syntax added by the compiler should be
introduced through a corpus fixture, a grammar update, and a parity report.
Until then, `bend-intel` still provides useful partial trees, syntax
diagnostics, symbols, navigation, folding, and completion around incomplete or
unsupported code. Bend's compiler remains the authority for type checking,
imports, and semantic diagnostics.
