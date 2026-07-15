package intel

import (
	"sort"
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/M31-Labs/bend-intel/bendlang"
)

var SemanticTokenTypes = []string{"type", "parameter", "variable", "property", "function", "keyword", "comment", "string", "number", "operator"}

type semanticToken struct {
	line, character, length, kind uint32
	start                         uint32
}

// SemanticTokens returns LSP delta-encoded tokens produced by Bend's query.
func (d *Document) SemanticTokens() ([]uint32, error) {
	query, err := bendlang.Highlights()
	if err != nil {
		return nil, err
	}
	var tokens []semanticToken
	for _, match := range query.Execute(d.Tree) {
		for _, capture := range match.Captures {
			n := capture.Node
			if n == nil || n.StartPoint().Row != n.EndPoint().Row {
				continue
			}
			kind, ok := semanticKind(capture.Name)
			if !ok {
				continue
			}
			position := d.positionAtOffset(int(n.StartByte()))
			length := uint32(len(utf16.Encode([]rune(n.Text(d.Source)))))
			if length > 0 {
				tokens = append(tokens, semanticToken{position.Line, position.Character, length, kind, n.StartByte()})
			}
		}
	}
	if d.Recovered() {
		for _, span := range sourceCommentSpans(d.Source) {
			position := d.positionAtOffset(span.start)
			length := uint32(len(utf16.Encode([]rune(string(d.Source[span.start:span.end])))))
			if length > 0 {
				tokens = append(tokens, semanticToken{position.Line, position.Character, length, 6, uint32(span.start)})
			}
		}
	}
	sort.Slice(tokens, func(i, j int) bool {
		if tokens[i].start == tokens[j].start {
			return tokens[i].length > tokens[j].length
		}
		return tokens[i].start < tokens[j].start
	})
	data := make([]uint32, 0, len(tokens)*5)
	var prevLine, prevChar uint32
	var lastStart uint32 = ^uint32(0)
	for _, token := range tokens {
		if token.start == lastStart {
			continue
		}
		lastStart = token.start
		deltaLine := token.line - prevLine
		deltaChar := token.character
		if deltaLine == 0 {
			deltaChar -= prevChar
		}
		data = append(data, deltaLine, deltaChar, token.length, token.kind, 0)
		prevLine, prevChar = token.line, token.character
	}
	return data, nil
}

type sourceSpan struct{ start, end int }

func sourceCommentSpans(source []byte) []sourceSpan {
	var spans []sourceSpan
	inString := byte(0)
	for i := 0; i < len(source); i++ {
		switch source[i] {
		case '"', '\'':
			if inString == 0 {
				inString = source[i]
			} else if inString == source[i] && (i == 0 || source[i-1] != '\\') {
				inString = 0
			}
		case '#':
			if inString != 0 {
				continue
			}
			end := i
			for end < len(source) && source[end] != '\n' {
				end++
			}
			spans = append(spans, sourceSpan{i, end})
			i = end - 1
		}
	}
	return spans
}

func semanticKind(capture string) (uint32, bool) {
	base := strings.Split(capture, ".")[0]
	name := map[string]string{"constructor": "type", "type": "type", "variable": "variable", "property": "property", "function": "function", "keyword": "keyword", "comment": "comment", "string": "string", "character": "string", "number": "number", "operator": "operator"}[base]
	if strings.Contains(capture, "parameter") {
		name = "parameter"
	}
	for i, candidate := range SemanticTokenTypes {
		if candidate == name {
			return uint32(i), true
		}
	}
	return 0, false
}

func (d *Document) positionAtOffset(offset int) Position {
	if offset > len(d.Source) {
		offset = len(d.Source)
	}
	var position Position
	for i := 0; i < offset; {
		r, size := utf8.DecodeRune(d.Source[i:])
		if r == '\n' {
			position.Line++
			position.Character = 0
		} else {
			position.Character += uint32(len(utf16.Encode([]rune{r})))
		}
		i += size
	}
	return position
}
