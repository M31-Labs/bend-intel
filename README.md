# Bend Intel

[M31-Labs/bend-intel](https://github.com/M31-Labs/bend-intel) is a public MIT-licensed project.

Pure-Go syntax and code intelligence for [Bend](https://github.com/HigherOrderCO/Bend), built on [gotreesitter](https://github.com/odvcencio/gotreesitter).

This M31 Labs project is deliberately split into reusable layers:

- `bendlang` embeds the Bend grammar, provides the Go external scanner, and exposes parsing and queries without CGo.
- `intel` turns Bend concrete syntax trees into diagnostics and author-facing document symbols.
- `cmd/bend-intel` is a small CLI proving the engine independently of any editor protocol.

The Bend compiler remains authoritative for language semantics. This project owns the fast, resilient structural layer used while a file is incomplete or a compiler is unavailable.

## Try it

```sh
go run ./cmd/bend-intel outline example.bend
go run ./cmd/bend-intel check example.bend
go run ./cmd/bendls
```

`check` exits non-zero when Tree-sitter reports syntax errors. `outline` emits stable JSON suitable for editor and agent integrations.

`bendls` is a stdio LSP server with incremental UTF-16 document sync, immediate syntax diagnostics, document symbols, folding ranges, hover, definitions, references, semantic tokens, workspace symbols, contextual candidates, and cross-file rename. A minimal Neovim setup is:

```lua
vim.api.nvim_create_autocmd("FileType", {
  pattern = "bend",
  callback = function()
    vim.lsp.start({ name = "bendls", cmd = { "/path/to/bendls" } })
  end,
})
```

Build both commands with `go build ./cmd/bend-intel ./cmd/bendls`.

## Development

```sh
go test ./...
```

The vendored resolved grammar and embedded blob come from `HigherOrderCO-archive/tree-sitter-bend` commit `7e7b88b77103e4d9c11a68d719be8702e4d6ad7f`. See `bendlang/grammar/UPSTREAM.md` and `tools/generate-bend-grammar.sh` for the reproducible generation workflow. The C-runtime parity harness is opt-in with `-tags='cgo parity'`.

## Delivered and next

The initial substrate now covers grammar generation/scanner parity, incremental
edits, queries, syntax diagnostics, semantic tokens, scopes, workspace imports,
navigation, rename, contextual completion, and the LSP transport. The next
upstream-facing work is to synchronize the grammar with current functional
syntax and add a structured Bend compiler backend for inferred types and
semantic diagnostics.

This is the completion point for the initial two packages, not a claim that
every phase of the broader proposal is finished. The archived grammar still
has five documented current-example gaps, and compiler-backed types,
pattern-coverage/HVM views, and parallel-structure visualizations remain
follow-on work.

The repository is transfer-ready, but M31 Labs can maintain it independently so Bend's maintainers do not inherit an obligation.

Downstream parser-core findings and regression boundaries are recorded in
[the gotreesitter findings note](docs/gotreesitter-findings.md).

## Original project charter

This implementation follows the referenced **Bend Language LSP Feasibility** specification: keep `gotreesitter-bend`-style grammar/runtime integration reusable, keep Bend compiler semantics authoritative, and layer a compiler-optional `bend-intel` engine plus `bendls` transport on top. The delivery mapping and deliberate non-goals are recorded in [the original-spec execution note](docs/original-spec.md).

## Current compatibility

The initial archived-grammar baseline parses 348 of 525 files in the current Bend repository without an error node. The syntax-first recovery view produces a useful structural outline for all 16 current examples and a diagnostics-free structural tree for 11; the remaining five examples are explicit functional-syntax compatibility gaps, not hidden behind permissive recovery. See [the compatibility snapshot](docs/compatibility.md) and reproduce it with `go run ./cmd/bend-corpus-report -path /path/to/Bend/examples`.
