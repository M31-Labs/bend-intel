# Original-spec execution note

This repository is the implementation of the referenced ChatGPT conversation, “Bend Language LSP Feasibility.” That charter called for two cooperating projects:

1. A reusable pure-Go Bend grammar package (`gotreesitter-bend` in the proposal).
2. A reusable Bend intelligence engine with an LSP executable (`bend-intel` and `bendls`).

The present M31 Labs repository keeps those as packages in one transfer-ready repository so the work can be maintained independently and moved upstream later without first splitting history.

## Acceptance status

The initial package acceptance gates are complete and reproducible: generated
grammar/scanner artifacts, C-vs-Go witness coverage, incremental edits,
queries, recovery diagnostics, workspace navigation, LSP synchronization,
fuzz/benchmark/WASM checks, and the optional semantic boundary all have tests
or executable checks. This does not mean the full proposal's later compiler
backend and Bend-native visualization phases are complete. The archived
grammar also leaves five current examples as explicit functional-syntax gaps;
see the compatibility snapshot rather than treating those files as silently
accepted.

| Original-spec requirement | Current implementation |
|---|---|
| Import existing `grammar.json`/parser tables | Vendored `bendlang/grammar/grammar.json`, generated `bendlang/grammar/bend.bin`, and `tools/generate-bend-grammar.sh` |
| Faithful stateful external scanner | `bendlang/scanner.go`, including indentation serialization and slash-terminated paths |
| No CGo for consumers | `bendlang` and `bendls` build as pure Go |
| Incremental editor document store | UTF-16 edits call `Tree.Edit` and `ParseIncremental` |
| Syntax-first author experience | Diagnostics, symbols, folds, selection ranges, hover, definitions, references, semantic tokens |
| Workspace intelligence | `.bend` discovery, resolved import graph, workspace symbols, visible completions, cross-file references, rename |
| Bend remains semantic authority | No type checker is duplicated; `intel.SemanticBackend` is an optional compiler boundary |
| Query substrate | Highlight, locals, tags, folds, and indent queries compile against the generated language |
| Correctness gates | C-vs-Go witness, incremental edit tests, fuzz target, benchmarks, opt-in parity script, and WASM smoke build |
| Current-language honesty | The archived grammar baseline is documented in `docs/compatibility.md` rather than silently accepting recovery trees |
| Distinctive Bend tooling | The next layer is reserved for compiler-backed pattern coverage, HVM lowering, and parallel-structure views |

## Deliberate boundaries

The grammar package does not own type checking, import semantics, runtime performance claims, or HVM execution. The intelligence engine may use syntax facts while a document is incomplete or the compiler is unavailable. Compiler diagnostics, inferred types, signatures, and pattern coverage should arrive through a stable Bend-side JSON or semantic-daemon protocol instead of reimplementing Bend's Rust semantics in Go.

The syntax-first layer also has a narrow, range-preserving recovery view for
current typed `def`/`type` headers and a few lexical constructs that the
archived grammar cannot yet model. It uses a fresh parser for every candidate,
preserves the original byte offsets, and exposes `Document.Recovered()` so
consumers can distinguish a structural view from an exact parse. It never
claims to validate the type or replace Bend's parser/type checker. A lexical
top-level outline fallback keeps definitions visible when parser recovery
stops before a later functional definition.

## Maintainer handoff

The repository is MIT licensed, uses the `github.com/M31-Labs/bend-intel` module path, and is intended to be public under M31 Labs. The archived grammar commit and regeneration workflow are recorded in `bendlang/grammar/UPSTREAM.md`.
