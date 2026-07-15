package intel

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"
)

// SemanticProtocolVersion is exchanged by the optional Bend compiler bridge.
// The syntax-first engine remains usable when no bridge is installed.
const SemanticProtocolVersion = "bend-intel/1"

// SemanticRequest is the stable newline-delimited JSON request sent to a
// compiler sidecar. Sources are strings rather than []byte so the wire format
// stays human-readable and preserves UTF-8 exactly.
type SemanticRequest struct {
	Protocol      string             `json:"protocol"`
	URI           string             `json:"uri"`
	WorkspaceRoot string             `json:"workspaceRoot,omitempty"`
	Documents     []SemanticDocument `json:"documents"`
	IncludeHVM    bool               `json:"includeHVM,omitempty"`
}

type SemanticDocument struct {
	URI     string `json:"uri"`
	Version int32  `json:"version"`
	Source  string `json:"source"`
}

// SemanticCommandBackend runs a long-lived-protocol-compatible command for a
// single check. A process is intentionally created per request for now: this
// keeps cancellation and workspace snapshots isolated while the Rust sidecar
// protocol settles. It is also safe for editors that restart semantic work on
// every keystroke.
type SemanticCommandBackend struct {
	Command string
	Args    []string
	Dir     string
	Env     []string
	Timeout time.Duration
}

func NewSemanticCommandBackend(command string, args ...string) *SemanticCommandBackend {
	return &SemanticCommandBackend{Command: command, Args: append([]string(nil), args...), Timeout: 10 * time.Second}
}

func (b *SemanticCommandBackend) Check(ctx context.Context, workspace WorkspaceSnapshot, uri string) (*SemanticResult, error) {
	return b.run(ctx, workspace, uri, false)
}

// Lower asks the sidecar for a real compiler-generated HVM view. It is an
// explicit request because the output can be substantially larger than the
// structural CST fallback used by the syntax-first LSP.
func (b *SemanticCommandBackend) Lower(ctx context.Context, workspace WorkspaceSnapshot, uri string) (*SemanticResult, error) {
	return b.run(ctx, workspace, uri, true)
}

func (b *SemanticCommandBackend) run(ctx context.Context, workspace WorkspaceSnapshot, uri string, includeHVM bool) (*SemanticResult, error) {
	if b == nil || strings.TrimSpace(b.Command) == "" {
		return nil, fmt.Errorf("semantic backend command is empty")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if b.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, b.Timeout)
		defer cancel()
	}
	request := SemanticRequest{Protocol: SemanticProtocolVersion, URI: uri, WorkspaceRoot: workspace.Root, IncludeHVM: includeHVM}
	request.Documents = make([]SemanticDocument, 0, len(workspace.Documents))
	for candidateURI, document := range workspace.Documents {
		request.Documents = append(request.Documents, SemanticDocument{URI: candidateURI, Version: document.Version, Source: string(document.Source)})
	}
	// Stable ordering makes sidecar traces and tests deterministic.
	sortSemanticDocuments(request.Documents)
	payload, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}
	cmd := exec.CommandContext(ctx, b.Command, b.Args...)
	cmd.Dir = b.Dir
	if len(b.Env) > 0 {
		cmd.Env = append(os.Environ(), b.Env...)
	}
	cmd.Stdin = bytes.NewReader(append(payload, '\n'))
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return nil, fmt.Errorf("semantic backend: %s", message)
	}
	var result SemanticResult
	decoder := json.NewDecoder(bytes.NewReader(stdout.Bytes()))
	if err := decoder.Decode(&result); err != nil {
		if err == io.EOF {
			return nil, fmt.Errorf("semantic backend returned no result")
		}
		return nil, fmt.Errorf("decode semantic result: %w", err)
	}
	if result.Protocol != "" && result.Protocol != SemanticProtocolVersion {
		return nil, fmt.Errorf("semantic backend protocol %q, want %q", result.Protocol, SemanticProtocolVersion)
	}
	return &result, nil
}

func sortSemanticDocuments(documents []SemanticDocument) {
	for i := 1; i < len(documents); i++ {
		for j := i; j > 0 && documents[j].URI < documents[j-1].URI; j-- {
			documents[j], documents[j-1] = documents[j-1], documents[j]
		}
	}
}

// SemanticHover turns compiler-provided typed spans into a compact Markdown
// hover. The structural hover remains the fallback when no matching span is
// available.
func (d *Document) SemanticHover(position Position, result *SemanticResult) string {
	if d == nil || result == nil {
		return ""
	}
	for _, typed := range result.Types {
		if rangeContains(typed.Range, position) && typed.Type != "" {
			return fmt.Sprintf("```bend\n%s\n```", typed.Type)
		}
	}
	if node := identifierAt(d, position); node != nil {
		name := node.Text(d.Source)
		for _, signature := range result.Signatures {
			if signature.Name != name {
				continue
			}
			if signature.ReturnType == "" {
				return fmt.Sprintf("**%s**", signature.Name)
			}
			return fmt.Sprintf("`%s` → `%s`", signature.Name, signature.ReturnType)
		}
	}
	return ""
}

func rangeContains(r Range, p Position) bool {
	return positionLE(r.Start, p) && positionLE(p, r.End)
}
