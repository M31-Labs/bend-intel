package intel

import (
	"context"
	"runtime"
	"testing"
	"time"
)

func TestSemanticCommandBackendRoundTrip(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses the POSIX shell as a tiny protocol fixture")
	}
	backend := NewSemanticCommandBackend("sh", "-c", `cat >/dev/null; printf '%s\n' '{"protocol":"bend-intel/1","uri":"file:///main.bend","signatures":[{"name":"main","returnType":"u24"}]}'`)
	result, err := backend.Check(context.Background(), WorkspaceSnapshot{Documents: map[string]DocumentSnapshot{
		"file:///main.bend": {URI: "file:///main.bend", Version: 3, Source: []byte("def main():\n  return 0\n")},
	}}, "file:///main.bend")
	if err != nil {
		t.Fatal(err)
	}
	if result.Protocol != SemanticProtocolVersion || len(result.Signatures) != 1 || result.Signatures[0].Name != "main" {
		t.Fatalf("result = %#v", result)
	}
}

func TestSemanticCommandBackendLoweringRequest(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses the POSIX shell as a tiny protocol fixture")
	}
	backend := NewSemanticCommandBackend("sh", "-c", `read request; case "$request" in *'"includeHVM":true'*) printf '%s\n' '{"protocol":"bend-intel/1","uri":"file:///main.bend","hvm":"@main = 0"}' ;; *) exit 3 ;; esac`)
	result, err := backend.Lower(context.Background(), WorkspaceSnapshot{Documents: map[string]DocumentSnapshot{
		"file:///main.bend": {URI: "file:///main.bend", Source: []byte("def main():\n  return 0\n")},
	}}, "file:///main.bend")
	if err != nil {
		t.Fatal(err)
	}
	if result.HVM != "@main = 0" {
		t.Fatalf("lowering result = %#v", result)
	}
}

func TestSemanticCommandBackendHonorsCancellation(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses the POSIX shell as a tiny protocol fixture")
	}
	backend := NewSemanticCommandBackend("sh", "-c", `sleep 2`)
	backend.Timeout = 0
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, err := backend.Check(ctx, WorkspaceSnapshot{}, "file:///main.bend"); err == nil {
		t.Fatal("expected cancellation error")
	}
}
