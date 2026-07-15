# Editor-path performance baseline

The parser and document layers carry small, reproducible benchmarks so parser
core changes can be evaluated as editor workloads rather than only as full
parse throughput. Results below were recorded on 2026-07-15 in WSL on an
Intel Core Ultra 9 285 (`linux/amd64`, Go 1.23):

| Workload | Result | Allocations |
|---|---:|---:|
| `bendlang` full parse (`BenchmarkParse`) | 283 µs/op | 2.95 MB, 26 allocs/op |
| `bendlang` one-byte incremental edit (`BenchmarkIncrementalParse`) | 1.36 µs/op | 3.5 KB, 8 allocs/op |
| 16-file Bend example corpus | 16/16 raw and structural clean | 0 recovered |
| Strict C/Go witness corpus | 16/16 exact | 0 shared errors |

Run the parser benchmarks with:

```sh
go test -run='^$' -bench=. -benchmem ./bendlang
```

The roadmap gate also exercises the complete editor path—incremental LSP
synchronization, syntax diagnostics, symbols, semantic tokens, selection and
navigation, the optional compiler sidecar, and a WASM build:

```sh
BEND_ROOT=/path/to/Bend ./tools/validate-roadmap.sh --strict-parser-core
```

The numbers are a baseline, not a universal latency promise. Hardware,
compiler version, editor message size, and workspace shape all affect absolute
latency. The correctness contract is stable: incremental trees must equal
clean trees, stale semantic results are discarded, and the strict current-
example corpus must remain raw-clean and C/Go-identical.
