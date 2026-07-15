#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
bend_root=${BEND_ROOT:-/tmp/bend-current}
strict=0
if [ "${1:-}" = "--strict-parser-core" ]; then
	strict=1
fi

if [ ! -d "$bend_root/examples" ]; then
	printf '%s\n' "BEND_ROOT must point to a Bend checkout (missing examples/): $bend_root" >&2
	exit 2
fi

cd "$root"
report=$(mktemp)
parity=$(mktemp)
semanticd_result=""
trap 'rm -f "$report" "$parity" "$semanticd_result"' EXIT

printf '%s\n' '== Go tests =='
go test ./...
go test -race ./...
go vet ./...

printf '%s\n' '== C-runtime witness =='
go test -tags='cgo parity' ./internal/cparity

printf '%s\n' '== Optional Bend semantic sidecar =='
if command -v cargo >/dev/null 2>&1; then
	cargo check --manifest-path semanticd/Cargo.toml
	cargo build --manifest-path semanticd/Cargo.toml >/tmp/bend-intel-semanticd-build.txt
	semanticd_bin="$root/semanticd/target/debug/bend-semanticd"
	if [ -x "$semanticd_bin" ]; then
		semanticd_result=$(mktemp)
		printf '%s\n' '{"protocol":"bend-intel/1","uri":"file:///__bend_intel_smoke__.bend","documents":[{"uri":"file:///__bend_intel_smoke__.bend","version":1,"source":"def main():\n  return 0\n"}],"includeHVM":true}' | "$semanticd_bin" >"$semanticd_result"
		python3 - "$semanticd_result" <<'PY'
import json, sys
result = json.load(open(sys.argv[1]))
if result.get("protocol") != "bend-intel/1" or not result.get("hvm"):
    raise SystemExit("semanticd HVM smoke did not return compiler output")
print("semanticd HVM smoke: ok")
PY
	fi
else
	printf '%s\n' 'cargo not installed; semanticd check skipped' >&2
fi

printf '%s\n' '== WASM smoke =='
./tools/wasm-smoke.sh >/tmp/bend-intel-wasm-validation.txt
tail -n 3 /tmp/bend-intel-wasm-validation.txt

printf '%s\n' '== Current Bend examples =='
go run ./cmd/bend-corpus-report -path "$bend_root/examples" >"$report"
python3 - "$report" "$strict" <<'PY'
import json, sys
report = json.load(open(sys.argv[1]))
strict = int(sys.argv[2])
print("files={files} rawClean={rawClean} intelClean={intelClean} intelComplete={intelComplete} recovered={recovered} diagnostics={diagnostics}".format(**report))
if report["intelClean"] != report["files"] or report["intelComplete"] != report["files"] or report["diagnostics"] != 0:
    raise SystemExit("syntax-first Bend examples are not fully useful")
if strict and report["rawClean"] != report["files"]:
    raise SystemExit("strict parser-core gate: raw Go trees are still stopped or errored")
if report["rawClean"] != report["files"]:
    print("warning: strict raw parser health is below 100%; recovered trees are explicitly marked")
PY

printf '%s\n' '== C/Go shape parity =='
set +e
./tools/run-bend-parity.sh "$bend_root/examples" >"$parity" 2>/tmp/bend-intel-parity-validation.err
parity_status=$?
set -e
python3 - "$parity" "$strict" "$parity_status" <<'PY'
import json, sys
rows = json.load(open(sys.argv[1]))
equal = sum(1 for row in rows if row.get("equal"))
print(f"equal={equal}/{len(rows)} sharedCError={sum(1 for row in rows if row.get('cError'))} sharedGoError={sum(1 for row in rows if row.get('goError'))}")
if int(sys.argv[2]) and equal != len(rows):
    raise SystemExit("strict parser-core gate: C/Go trees are not identical")
if equal != len(rows):
    print("warning: C/Go witness differences remain in the parser-core reengineering boundary")
PY

if [ "$strict" -eq 1 ]; then
	printf '%s\n' 'roadmap validation passed (strict parser-core, raw health, and C/Go parity gates)'
else
	printf '%s\n' 'roadmap validation passed (use --strict-parser-core for raw health and C/Go parity gates)'
fi
