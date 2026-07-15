# Original-spec execution note

This repository implements the charter from the referenced **Bend Language LSP
Feasibility** conversation. The charter called for two cooperating projects:

1. a reusable pure-Go Bend grammar/runtime package (`gotreesitter-bend` in the
   proposal); and
2. a reusable Bend intelligence engine with an LSP executable (`bend-intel`
   and `bendls`).

M31 Labs keeps those layers in one public MIT repository so the work can be
   maintained independently and transferred upstream later without first
   splitting history.

## Requirement matrix

| Original-spec requirement | Status and evidence |
|---|---|
| Import existing `grammar.json` and generated tables | Complete: `bendlang/grammar/grammar.json`, embedded `bend.bin`, source grammar, and `tools/generate-bend-grammar.sh` |
| Faithful stateful external scanner | Complete at the language boundary: Go port covers indentation serialization, comments, natural literals, and paths; scanner tests cover round trips and edits |
| No CGo for consumers | Complete: normal `bendlang`, `bend-intel`, and `bendls` builds are pure Go |
| Current Bend compatibility fixtures | Complete for all 16 current examples: raw Go acceptance 16/16, structural completion 16/16, diagnostics-free 16/16, and C/Go witness parity 16/16 |
| Incremental editor document store | Complete: UTF-16 edits call `Tree.Edit`/`ParseIncremental`, then select fresh recovery candidates when needed |
| Syntax-first author experience | Complete: diagnostics, explicit parse health, symbols, folds, selection ranges, hover, semantic tokens, definitions, references, rename, completion |
| Scope-aware Bend bindings | Complete as a structural model: parameters, lambdas, assignment/pattern bindings, match/switch/fold/bend scopes; compiler imports and unscoped semantics remain authoritative |
| Workspace intelligence | Complete as a syntax index: file discovery, import graph, workspace symbols, cross-file navigation/references/rename, contextual completion |
| Call hierarchy | Complete as a conservative syntax-derived LSP feature; overloads and higher-order resolution remain compiler questions |
| Bend remains semantic authority | Complete by design: no Go type checker or HVM evaluator is duplicated |
| Query substrate | Complete: highlights, locals, tags, folds, and indents are vendored and compiled against the generated language |
| Correctness gates | Complete and reproducible: unit/race/vet, scanner corpus, incremental edits, fuzz target, benchmarks, C witness, and WASM smoke |
| Compiler diagnostics and inferred types | Complete as an optional protocol and reference `semanticd`; LSP drops stale versions and works without the sidecar |
| Signature help and semantic hover | Complete when the backend supplies signatures/types; structural hover remains the fallback |
| Pattern coverage | Implemented conservatively in `intel.PatternCoverage` for duplicate/wildcard structural findings; exhaustive constructor coverage remains compiler-owned and is reported as such |
| Parallel-structure view | Implemented conservatively in `intel.ParallelStructure`; it reports explicit branch containers and never predicts scheduling or speedups |
| HVM lowering view | Complete when the optional sidecar is configured: `bend/hvmView` requests compiler-generated HVM; structural CST remains the explicit fallback |
| Binding visualization | Implemented through `bend/bindingInfo` and the `BindingInfo` model; exact unscoped-variable explanations remain compiler-owned |
| Full raw C/Go tree parity | Complete for the current examples: 16/16 raw Go trees accepted and 16/16 exact C/Go witness trees; the strict validation gate is green |
| Full performance/editor benchmark matrix | Complete baseline in [`docs/performance.md`](performance.md): full/incremental parse measurements plus the strict end-to-end editor-path gate; absolute budgets remain hardware-specific |

## Recovery and authority boundaries

gotreesitter can intentionally return partial trees for editor workloads. This
project therefore distinguishes:

- **raw parser health:** stop reason, root error metadata, and covered bytes;
- **structural recovery:** a fresh range-preserving candidate marked
  `Recovered()`; and
- **compiler semantics:** Bend diagnostics, types, imports, lowering, and
  runtime claims.

The recovery layer never rewrites `Document.Source`. It exists so symbols,
navigation, and completion remain useful while the user types or while a
grammar/runtime mismatch is being resolved. See
[`docs/gotreesitter-findings.md`](gotreesitter-findings.md) for the silent
`no_stacks_alive` fixture and the C/Go witness policy.

## Maintainer handoff

The repository is public under M31 Labs, MIT licensed, and transfer-ready. The
grammar provenance, Bend compiler pin, generated hashes, and regeneration steps
are in [`bendlang/grammar/UPSTREAM.md`](../bendlang/grammar/UPSTREAM.md).

The initial packages are useful and the strict roadmap validation passes for
the current Bend examples. The compiler-backed lowering view is available
through the optional sidecar; syntax recovery, negative fixtures, and compiler
semantics remain explicitly separated rather than being presented as false
parser acceptance.
