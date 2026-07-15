# Bend Intel

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

`bendls` is a stdio LSP server with incremental UTF-16 document sync, immediate syntax diagnostics, document symbols, folding ranges, hover, same-file definitions, and references. A minimal Neovim setup is:

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

The embedded grammar was generated from `HigherOrderCO-archive/tree-sitter-bend` commit `7e7b88b77103e4d9c11a68d719be8702e4d6ad7f`. See `bendlang/grammar/UPSTREAM.md` for the reproducible update command and provenance.

## Roadmap

1. Grammar/scanner parity and incremental edit corpus.
2. Syntax diagnostics, semantic tokens, symbols, folding, and selection ranges.
3. Bend-aware scopes, definitions, references, rename, and workspace imports.
4. LSP transport and contextual completion.
5. Optional structured Bend compiler backend for types and semantic diagnostics.

The repository is transfer-ready, but M31 Labs can maintain it independently so Bend's maintainers do not inherit an obligation.

## Original project charter

This implementation follows the referenced **Bend Language LSP Feasibility** specification: keep `gotreesitter-bend`-style grammar/runtime integration reusable, keep Bend compiler semantics authoritative, and layer a compiler-optional `bend-intel` engine plus `bendls` transport on top. The delivery mapping and deliberate non-goals are recorded in [the original-spec execution note](docs/original-spec.md).

## Current compatibility

The initial archived-grammar baseline parses 348 of 525 files in the current Bend repository without an error node. The 177 failures are tracked as compatibility work, not hidden behind permissive recovery. See [the compatibility snapshot](docs/compatibility.md).
