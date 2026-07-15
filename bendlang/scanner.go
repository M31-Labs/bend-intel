package bendlang

import gotreesitter "github.com/odvcencio/gotreesitter"

const (
	tokenNewline = iota
	tokenIndent
	tokenDedent
	tokenComment
	tokenPath
	tokenErrorSentinel
	bendTokenCount
)

type scannerState struct{ indents []uint16 }

type externalScanner struct{ symbols []gotreesitter.Symbol }

func newExternalScanner(symbols []gotreesitter.Symbol) externalScanner {
	return externalScanner{symbols: append([]gotreesitter.Symbol(nil), symbols...)}
}

func (externalScanner) Create() any { return &scannerState{indents: []uint16{0}} }
func (externalScanner) Destroy(any) {}

func (externalScanner) Serialize(payload any, buf []byte) int {
	state := payload.(*scannerState)
	n := len(state.indents) - 1
	if n > len(buf) {
		n = len(buf)
	}
	for i := 0; i < n; i++ {
		buf[i] = byte(state.indents[i+1])
	}
	return n
}

func (externalScanner) Deserialize(payload any, buf []byte) {
	state := payload.(*scannerState)
	state.indents = append(state.indents[:0], 0)
	for _, indent := range buf {
		state.indents = append(state.indents, uint16(indent))
	}
}

func (s externalScanner) Scan(payload any, lexer *gotreesitter.ExternalLexer, valid []bool) bool {
	state := payload.(*scannerState)
	if isValid(valid, tokenErrorSentinel) {
		return false
	}
	lexer.MarkEnd()
	foundEOL := false
	indent := uint32(0)
	firstCommentIndent := int64(-1)
	for {
		switch lexer.Lookahead() {
		case '\n':
			foundEOL, indent = true, 0
			lexer.Advance(true)
		case ' ':
			indent++
			lexer.Advance(true)
		case '\r', '\f':
			indent = 0
			lexer.Advance(true)
		case '\t':
			indent += 8
			lexer.Advance(true)
		case '#':
			if !(isValid(valid, tokenIndent) || isValid(valid, tokenDedent) || isValid(valid, tokenNewline)) {
				goto whitespaceDone
			}
			if !foundEOL {
				return false
			}
			if firstCommentIndent < 0 {
				firstCommentIndent = int64(indent)
			}
			for lexer.Lookahead() != 0 && lexer.Lookahead() != '\n' {
				lexer.Advance(true)
			}
			lexer.Advance(true)
			indent = 0
		case 0:
			indent, foundEOL = 0, true
			goto whitespaceDone
		default:
			goto whitespaceDone
		}
	}

whitespaceDone:
	if foundEOL {
		current := state.indents[len(state.indents)-1]
		if isValid(valid, tokenIndent) && indent > uint32(current) {
			state.indents = append(state.indents, uint16(indent))
			return s.emit(lexer, tokenIndent)
		}
		if (isValid(valid, tokenDedent) || !isValid(valid, tokenNewline)) && indent < uint32(current) && firstCommentIndent < int64(current) {
			state.indents = state.indents[:len(state.indents)-1]
			return s.emit(lexer, tokenDedent)
		}
		if isValid(valid, tokenNewline) {
			return s.emit(lexer, tokenNewline)
		}
	}
	if isValid(valid, tokenPath) && isIdentifierChar(lexer.Lookahead()) {
		lexer.Advance(false)
		for isIdentifierChar(lexer.Lookahead()) {
			lexer.Advance(false)
		}
		if lexer.Lookahead() == '/' {
			lexer.MarkEnd()
			return s.emit(lexer, tokenPath)
		}
	}
	return false
}

func (s externalScanner) emit(lexer *gotreesitter.ExternalLexer, token int) bool {
	lexer.SetResultSymbol(s.symbols[token])
	return true
}

func isValid(valid []bool, token int) bool { return token >= 0 && token < len(valid) && valid[token] }

func isIdentifierChar(r rune) bool {
	return r >= '0' && r <= '9' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r == '.' || r == '-'
}
