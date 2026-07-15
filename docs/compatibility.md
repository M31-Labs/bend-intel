# Compatibility snapshot

Snapshot date: 2026-07-15

| Input | Result |
|---|---:|
| Current `HigherOrderCO/Bend` `.bend` files discovered | 525 |
| Parsed without an `ERROR` node | 348 (66.3%) |
| Parsed with one or more `ERROR` nodes | 177 (33.7%) |

This is intentionally a baseline, not a compatibility claim. It confirms the archived grammar and scanner are useful enough to bootstrap the structural layer while exposing a substantial current-language gap. No failed file is silently classified as compatible.

Likely mismatch classes include syntax added since the archived grammar's Bend 0.2.37 target and gotreesitter/C-runtime parity issues. The next parity harness must classify these separately before grammar rules are changed.

Reproduce against a Bend checkout:

```sh
go build -o /tmp/bend-intel ./cmd/bend-intel
find /path/to/Bend -type f -name '*.bend' -print0 |
  xargs -0 -n1 sh -c '/tmp/bend-intel check "$0" >/dev/null'
```
