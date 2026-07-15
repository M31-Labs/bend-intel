# Bend Intel

[M31-Labs/bend-intel](https://github.com/M31-Labs/bend-intel) is a public
MIT-licensed Bend tooling project. It is maintained as a pure-Go downstream
exercise for [gotreesitter](https://github.com/odvcencio/gotreesitter), with
Bend's compiler remaining the semantic authority.

The repository contains the two packages from the original
[Bend Language LSP Feasibility](docs/original-spec.md) charter:

- `bendlang`: a reusable Bend grammar package with an embedded grammar blob,
  faithful stateful external scanner, queries, fuzzing, benchmarks, and no
  CGo requirement for consumers.
- `intel`: an editor-independent document, scope, workspace, recovery, and
  semantic-backend library.
- `bendls`: a stdio LSP transport over `intel`.
- `semanticd`: an optional Rust sidecar that calls the pinned Bend compiler for
  diagnostics, inferred types, signatures, and definitions.

## What is useful today

Without a Bend compiler installed, `bendls` provides:

- incremental UTF-16 text synchronization;
- syntax diagnostics and explicit parse-health telemetry;
- semantic tokens, document/workspace symbols, folds, and selection ranges;
- scope-aware definitions, references, rename, and contextual completion;
- signature/call hierarchy plumbing;
- conservative Bend-specific parallel-structure, binding, and
  pattern-coverage views;
- an on-demand compiler-generated HVM lowering view when `BEND_SEMANTICD` is
  configured, with the structural CST view as a safe fallback;
- a lexical outline fallback when parser recovery stops before a later
  definition.

With `BEND_SEMANTICD` configured, hover and signature help are enriched by
Bend's own checker. The sidecar protocol is documented in
[`docs/semantic-protocol.md`](docs/semantic-protocol.md).

```sh
cargo build --manifest-path semanticd/Cargo.toml
BEND_SEMANTICD=$PWD/semanticd/target/debug/bend-semanticd bendls
```

The LSP is intentionally compiler-optional: syntax work remains responsive
while the compiler is absent, restarting, or checking a stale document.

## Try it

```sh
go run ./cmd/bend-intel outline /path/to/file.bend
go run ./cmd/bend-intel status /path/to/file.bend
go run ./cmd/bend-intel check /path/to/file.bend
go run ./cmd/bend-intel calls /path/to/file.bend
go run ./cmd/bend-intel parallel /path/to/file.bend
go run ./cmd/bend-intel patterns /path/to/file.bend
go run ./cmd/bend-intel hvm /path/to/file.bend
go run ./cmd/bendls
```

The CLI's `hvm` command returns a clearly labelled structural CST view. The
LSP's `bend/hvmView` request returns a real compiler-generated HVM view on
demand when the optional sidecar is configured, and otherwise uses that
structural fallback. Tree-sitter never pretends to perform Bend's desugaring.

A minimal Neovim setup is:

```lua
vim.api.nvim_create_autocmd("FileType", {
  pattern = "bend",
  callback = function()
    vim.lsp.start({ name = "bendls", cmd = { "/path/to/bendls" } })
  end,
})
```

## Compatibility and honesty

The snapshot used for validation is Bend commit
`814453670d0e0d6777c1313c972764dba0491b7f` (0.2.38). The archived grammar
is vendored as source under `bendlang/grammar/source/` and regenerated into
`grammar.json` and `bend.bin`; the current-language shims and recovery policy
are described in [`docs/compatibility.md`](docs/compatibility.md).

On Bend's 16 example programs, the syntax-first layer currently reports:

```text
raw gotreesitter trees accepted:       16/16
intel structural trees complete:       16/16
intel diagnostics-free examples:       16/16
explicitly recovered structural trees:  0/16
C/Go exact witness tree shape:         16/16
```

The runtime blob is extracted from the generated C witness with the pinned
gotreesitter `ts2go` tool, so the parser-core reengineering boundary is
measured against the same 1730-state/283-symbol tables that Bend's C runtime
uses. The pure-Go runtime now agrees with the C witness on named shape,
fields, ranges, error/missing/extra flags, and byte metadata for every current
example.

The reproducible default validation command accepts the useful recovered view
and fails on diagnostics or incomplete editor trees:

```sh
BEND_ROOT=/path/to/Bend ./tools/validate-roadmap.sh
```

Use `--strict-parser-core` when changing the parser runtime or grammar. It
requires raw acceptance and C/Go shape parity for every current example.

## Development

```sh
go test ./...
go test -race ./...
go vet ./...
go test -tags='cgo parity' ./internal/cparity
./tools/wasm-smoke.sh
```

The C runtime is only an opt-in parity witness. Normal consumers of `bendlang`
and `bendls` do not need a C compiler, CGo, Node.js, or the Tree-sitter C
runtime. The package also builds for WASM; the smoke script writes test
artifacts under `/tmp/bend-intel-wasm-smoke`.

Parser and incremental-edit baselines are recorded in
[`docs/performance.md`](docs/performance.md).

## Project boundaries

`bendlang` owns syntax integration, scanner state, queries, and compatibility
fixtures. `intel` owns fast structural indexing and recovery. Bend owns type
checking, import semantics, inferred types, compiler diagnostics, lowering,
and runtime claims. Keeping those authorities separate is the central design
choice from the original specification and lets each project improve without
forcing the other maintainer to absorb a new compiler implementation.
