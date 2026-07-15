package intel

import "context"

type DocumentSnapshot struct {
	URI     string
	Version int32
	Source  []byte
}

type WorkspaceSnapshot struct {
	Root      string
	Documents map[string]DocumentSnapshot
}

type TypedSpan struct {
	Range Range  `json:"range"`
	Type  string `json:"type"`
}

type Signature struct {
	Name       string   `json:"name"`
	Parameters []string `json:"parameters,omitempty"`
	ReturnType string   `json:"returnType,omitempty"`
}

type SemanticDefinition struct {
	Name  string `json:"name"`
	URI   string `json:"uri"`
	Range Range  `json:"range"`
}

type SemanticResult struct {
	Protocol    string               `json:"protocol,omitempty"`
	Diagnostics []Diagnostic         `json:"diagnostics,omitempty"`
	Types       []TypedSpan          `json:"types,omitempty"`
	Signatures  []Signature          `json:"signatures,omitempty"`
	Definitions []SemanticDefinition `json:"definitions,omitempty"`
	HVM         string               `json:"hvm,omitempty"`
}

// SemanticBackend is the optional Bend compiler boundary. Implementations may
// call a Rust sidecar or a structured `bend check` process; bend-intel never
// reimplements Bend's type system.
type SemanticBackend interface {
	Check(context.Context, WorkspaceSnapshot, string) (*SemanticResult, error)
}

// SemanticLoweringBackend is an optional, on-demand extension. Lowering is
// deliberately separate from Check so an editor does not serialize a large
// generated HVM book on every keystroke.
type SemanticLoweringBackend interface {
	Lower(context.Context, WorkspaceSnapshot, string) (*SemanticResult, error)
}

type DisabledBackend struct{}

func (DisabledBackend) Check(context.Context, WorkspaceSnapshot, string) (*SemanticResult, error) {
	return &SemanticResult{}, nil
}
