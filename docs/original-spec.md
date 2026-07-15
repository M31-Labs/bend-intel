# Original-spec execution note

This repository is the implementation of the referenced ChatGPT conversation, “Bend Language LSP Feasibility.” That charter called for two cooperating projects:

1. A reusable pure-Go Bend grammar package (`gotreesitter-bend` in the proposal).
2. A reusable Bend intelligence engine with an LSP executable (`bend-intel` and `bendls`).

The present M31 Labs repository keeps those as packages in one transfer-ready repository so the work can be maintained independently and moved upstream later without first splitting history.

| Original-spec requirement | Current implementation |
|---|---|
| Import existing `grammar.json`/parser tables | Embedded `bendlang/grammar/bend.bin`, generated from the archived parser tables |
| Faithful stateful external scanner | `bendlang/scanner.go`, including indentation serialization and slash-terminated paths |
| No CGo for consumers | `bendlang` and `bendls` build as pure Go |
| Incremental editor document store | UTF-16 edits call `Tree.Edit` and `ParseIncremental` |
| Syntax-first author experience | Diagnostics, symbols, folds, hover, definitions, references, semantic tokens |
| Workspace intelligence | `.bend` discovery, imports, workspace symbols, completions, cross-file references, rename |
| Bend remains semantic authority | No type checker is duplicated; compiler integration remains an optional next backend |
| Current-language honesty | The archived grammar baseline is documented in `docs/compatibility.md` rather than silently accepting recovery trees |
| Distinctive Bend tooling | The next layer is reserved for compiler-backed pattern coverage, HVM lowering, and parallel-structure views |

## Deliberate boundaries

The grammar package does not own type checking, import semantics, runtime performance claims, or HVM execution. The intelligence engine may use syntax facts while a document is incomplete or the compiler is unavailable. Compiler diagnostics, inferred types, signatures, and pattern coverage should arrive through a stable Bend-side JSON or semantic-daemon protocol instead of reimplementing Bend's Rust semantics in Go.

## Maintainer handoff

The repository is MIT licensed, uses the `github.com/M31-Labs/bend-intel` module path, and is intended to be public under M31 Labs. The archived grammar commit and regeneration workflow are recorded in `bendlang/grammar/UPSTREAM.md`.
