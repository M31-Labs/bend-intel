# Compatibility snapshot

Snapshot date: 2026-07-15. Bend source checkout:
`HigherOrderCO/Bend@814453670d0e0d6777c1313c972764dba0491b7f` (0.2.38).
The grammar starts from the archived Tree-sitter Bend commit documented in
[`bendlang/grammar/UPSTREAM.md`](../bendlang/grammar/UPSTREAM.md), with the
source and generated artifacts checked into this repository.

## Current examples

The 16 files in Bend's `examples/` are the most useful smoke corpus. The
report distinguishes raw parser health from the range-preserving structural
view used by `intel`:

| Measure | Result |
|---|---:|
| Raw Go trees accepted without root error/stop/truncation | 16/16 |
| Structural trees complete after explicit recovery | 16/16 |
| Examples with zero `intel` diagnostics | 16/16 |
| Trees marked `Recovered()` | 0/16 |
| C/Go exact named-tree and metadata witness | 16/16 |

The current examples require no recovery. The recovery layer is still retained
for incomplete editor buffers and negative fixtures; it never edits the user's
source, masks only a fresh parser candidate, preserves byte/point ranges, and
exposes the distinction through `Document.Health()`.

Raw parser stop metadata is not hidden. A `no_stacks_alive` or truncated raw
tree is counted as unhealthy even when no named `ERROR` node was materialized.
This prevents the silent-prefix failure that motivated
[`docs/gotreesitter-findings.md`](gotreesitter-findings.md).

## Full Bend checkout

The current checkout contains 525 files across valid programs, negative tests,
and recovery fixtures. It is not a valid-language coverage denominator. The
latest report is:

```text
raw metadata-clean:       469/525
intel complete:           475/525
intel diagnostics-free:   475/525
recovered:                 56
incomplete:                50
diagnostics:              211
raw stop reasons:         accepted 524, no_stacks_alive 1
```

The intentionally invalid fixtures account for much of the latter population;
they are retained to exercise error recovery rather than converted into false
green compatibility claims.

The opt-in C witness compares 462/525 mixed fixtures exactly. The strict
release gate is intentionally based on the 16 current Bend examples (the
language's maintained smoke corpus), while negative and historical golden
fixtures continue to exercise error recovery and parser-boundary behavior.

## Reproduce

```sh
go run ./cmd/bend-corpus-report -path /path/to/Bend/examples
./tools/run-bend-parity.sh /path/to/Bend/examples
BEND_ROOT=/path/to/Bend ./tools/validate-roadmap.sh
```

`run-bend-parity.sh` is an opt-in C-runtime witness. It compares named
S-expression shape, fields, ranges, error/missing/extra flags, and byte
metadata. A mismatch is reported, never silently accepted.

For the strict parser-core gate:

```sh
BEND_ROOT=/path/to/Bend ./tools/validate-roadmap.sh --strict-parser-core
```

That gate is green for the current Bend examples. It remains a required check
when the parser core or grammar changes.

## Language boundary

The grammar package follows the compiler rather than inventing an independent
Bend dialect. When Bend syntax changes, add a compiler fixture, update the
grammar source, regenerate the blob, and record the parity result. Until a
grammar/runtime change is reviewed, `bend-intel` can still provide partial
trees, explicit health, symbols, navigation, and completion around the source.
