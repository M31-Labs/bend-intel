# Bend semantic backend protocol

`bend-intel` keeps compiler semantics behind a small newline-delimited JSON
boundary. The syntax engine does not reimplement Bend's type checker, and the
LSP remains useful when this process is unavailable.

The protocol version is `bend-intel/1`. Each request is one JSON object on
stdin; each response is one JSON object on stdout.

## Request

```json
{
  "protocol": "bend-intel/1",
  "uri": "file:///workspace/main.bend",
  "workspaceRoot": "/workspace",
  "documents": [
    {
      "uri": "file:///workspace/main.bend",
      "version": 7,
      "source": "def main():\\n  return 0\\n"
    }
  ],
  "includeHVM": false
}
```

The document list is a versioned workspace snapshot. The reference
`bend-semanticd` implementation prefers those in-memory sources for imports
and falls back to Bend's normal filesystem loader for unopened files.

The optional `includeHVM: true` request flag asks the sidecar to include the
compiler-generated HVM book. `bendls` uses this only for the explicit
`bend/hvmView` request; ordinary semantic checks omit it so a large lowering
result is not serialized on every edit.

## Response

```json
{
  "protocol": "bend-intel/1",
  "uri": "file:///workspace/main.bend",
  "diagnostics": [],
  "types": [
    {"range": {"start": {"line": 0, "character": 0}, "end": {"line": 1, "character": 0}}, "type": "u24"}
  ],
  "signatures": [
    {"name": "main", "parameters": [], "returnType": "u24"}
  ],
  "definitions": [
    {"name": "main", "uri": "file:///workspace/main.bend", "range": {"start": {"line": 0, "character": 0}, "end": {"line": 1, "character": 0}}}
  ],
  "hvm": "@main = 0\n"
}
```

`hvm` is present only when `includeHVM` was requested and compilation
succeeded. Without it, the LSP's HVM view is an explicitly labelled
range-preserving CST view.

Diagnostics use LSP-style zero-based line and character positions. Severity is
`1` error, `2` warning, and `3` informational/allowed. Compiler diagnostics
are labelled `bend-compiler`; syntax diagnostics are produced immediately by
the Go CST layer and labelled `bend-syntax` or `bend-parser`.

## Running the reference sidecar

```sh
cargo build --manifest-path semanticd/Cargo.toml
BEND_SEMANTICD=$PWD/semanticd/target/debug/bend-semanticd bendls
```

`BEND_SEMANTICD` is optional. `bendls` starts the command once per semantic
check, applies a ten-second timeout by default, cancels stale work after an
edit, and discards results whose document version is no longer current.

The sidecar is pinned to the Bend commit documented in
[`bendlang/grammar/UPSTREAM.md`](../bendlang/grammar/UPSTREAM.md). Updating it
is intentionally a separate compatibility decision: compiler diagnostics and
inferred types must follow Bend, while the syntax-first engine can continue to
operate during the transition.
