# C parity witness

This optional package compiles the archived Bend C parser and scanner beside
the pure-Go gotreesitter parser. It is a test oracle, not a runtime dependency.

Run it in a Linux environment with a C compiler and the opt-in `parity` tag:

```sh
go test -tags='cgo parity' ./internal/cparity
./tools/run-bend-parity.sh /path/to/Bend/examples
```

The copied C sources come from `HigherOrderCO-archive/tree-sitter-bend` at
`7e7b88b77103e4d9c11a68d719be8702e4d6ad7f`; the provenance and update policy
are documented in `bendlang/grammar/UPSTREAM.md`.
Normal `go test ./...` and downstream packages do not require CGo.
