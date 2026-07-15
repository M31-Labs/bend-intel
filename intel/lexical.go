package intel

import (
	"bytes"
	"sort"
)

// lexicalSymbols is a deliberately conservative fallback for source that is
// newer than the vendored Tree-sitter grammar. The CST remains the primary
// source of symbols; this pass only recovers top-level names when parser error
// recovery has stopped before a later definition. It never attempts to infer
// types, bindings, or imports.
func lexicalSymbols(source []byte, existing []Symbol) []Symbol {
	seen := make(map[string]bool, len(existing))
	for _, symbol := range existing {
		seen[symbol.Name+"\x00"+itoaLine(symbol.Range.Start.Line)] = true
	}
	var out []Symbol
	var line uint32
	for lineStart := 0; lineStart < len(source); {
		lineEnd := bytes.IndexByte(source[lineStart:], '\n')
		if lineEnd < 0 {
			lineEnd = len(source)
		} else {
			lineEnd += lineStart
		}
		if lineStart < lineEnd && source[lineStart] != ' ' && source[lineStart] != '\t' {
			if name, kind, start, end := topLevelSymbol(source[lineStart:lineEnd]); name != "" {
				key := name + "\x00" + itoaLine(line)
				if !seen[key] {
					seen[key] = true
					out = append(out, Symbol{Name: name, Kind: kind, Range: Range{Start: Position{Line: line, Character: uint32(start)}, End: Position{Line: line, Character: uint32(end)}}})
				}
			}
		}
		if lineEnd == len(source) {
			break
		}
		lineStart = lineEnd + 1
		line++
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Range.Start.Line == out[j].Range.Start.Line {
			return out[i].Range.Start.Character < out[j].Range.Start.Character
		}
		return out[i].Range.Start.Line < out[j].Range.Start.Line
	})
	return out
}

func topLevelSymbol(line []byte) (name, kind string, start, end int) {
	offset := 0
	for offset < len(line) && (line[offset] == ' ' || line[offset] == '\t') {
		offset++
	}
	trimmed := bytes.TrimRight(line[offset:], " \t\r")
	if len(trimmed) == 0 || trimmed[0] == '#' {
		return "", "", 0, 0
	}
	if wordAt(trimmed, 0, "import") || wordAt(trimmed, 0, "from") {
		return "", "", 0, 0
	}
	for _, keyword := range []string{"unchecked", "const"} {
		if wordAt(trimmed, 0, keyword) {
			offset += len(keyword)
			for offset < len(line) && (line[offset] == ' ' || line[offset] == '\t') {
				offset++
			}
			trimmed = bytes.TrimRight(line[offset:], " \t\r")
			break
		}
	}
	if len(trimmed) == 0 {
		return "", "", 0, 0
	}
	for _, definition := range []struct {
		keyword string
		kind    string
	}{{"def", "function"}, {"type", "type"}, {"object", "object"}, {"hvm", "function"}} {
		if wordAt(trimmed, 0, definition.keyword) {
			offset += len(definition.keyword)
			for offset < len(line) && (line[offset] == ' ' || line[offset] == '\t') {
				offset++
			}
			rest := bytes.TrimRight(line[offset:], " \t\r")
			name, nameStart, nameEnd := firstName(rest)
			if name == "" {
				return "", "", 0, 0
			}
			return name, definition.kind, offset + nameStart, offset + nameEnd
		}
	}
	eq := topLevelEquals(trimmed)
	if eq < 0 {
		return "", "", 0, 0
	}
	lhsStart, lhsEnd := 0, eq
	for lhsStart < lhsEnd && (trimmed[lhsStart] == ' ' || trimmed[lhsStart] == '\t') {
		lhsStart++
	}
	for lhsEnd > lhsStart && (trimmed[lhsEnd-1] == ' ' || trimmed[lhsEnd-1] == '\t') {
		lhsEnd--
	}
	if lhsStart == lhsEnd {
		return "", "", 0, 0
	}
	lhs := trimmed[lhsStart:lhsEnd]
	if lhs[0] == '(' {
		lhsStart++
		for lhsStart < lhsEnd && (trimmed[lhsStart] == ' ' || trimmed[lhsStart] == '\t') {
			lhsStart++
		}
		lhs = trimmed[lhsStart:lhsEnd]
	}
	name, nameStart, nameEnd := firstName(lhs)
	if name == "" {
		return "", "", 0, 0
	}
	return name, "function", offset + lhsStart + nameStart, offset + lhsStart + nameEnd
}

func firstName(source []byte) (string, int, int) {
	start := 0
	for start < len(source) && (source[start] == ' ' || source[start] == '\t' || source[start] == '(' || source[start] == '*') {
		start++
	}
	end := start
	for end < len(source) && isNameByte(source[end]) {
		end++
	}
	return string(source[start:end]), start, end
}

func isNameByte(b byte) bool {
	return isWordByte(b) || b == '.' || b == '/' || b == '-'
}

func topLevelEquals(source []byte) int {
	depth := 0
	for i, b := range source {
		switch b {
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			if depth > 0 {
				depth--
			}
		case '=':
			if depth == 0 && (i == 0 || source[i-1] != '=') && (i+1 == len(source) || source[i+1] != '=') {
				return i
			}
		}
	}
	return -1
}

func itoaLine(line uint32) string {
	// The small fixed-width conversion avoids pulling formatting into the hot
	// document indexing path.
	var buf [10]byte
	i := len(buf)
	if line == 0 {
		return "0"
	}
	for line > 0 {
		i--
		buf[i] = byte('0' + line%10)
		line /= 10
	}
	return string(buf[i:])
}
