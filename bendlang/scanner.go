package bendlang

import gotreesitter "github.com/odvcencio/gotreesitter"

const (
	tokenNewline = iota
	tokenIndent
	tokenDedent
	tokenComment
	tokenNat
	tokenPath
	tokenPathExpr
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
			if !(isValid(valid, tokenIndent) || isValid(valid, tokenDedent) || isValid(valid, tokenNewline) || isValid(valid, tokenNat)) {
				goto whitespaceDone
			}
			lexer.Advance(false)
			if lexer.Lookahead() == '{' {
				// Multiline comments are ordinary grammar extras. Leave the
				// input untouched for the regular lexer to consume them.
				return false
			}
			if lexer.Lookahead() >= '0' && lexer.Lookahead() <= '9' {
				if !isValid(valid, tokenNat) {
					return false
				}
				for lexer.Lookahead() >= '0' && lexer.Lookahead() <= '9' {
					lexer.Advance(false)
				}
				lexer.MarkEnd()
				return s.emit(lexer, tokenNat)
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
	if isValid(valid, tokenPathExpr) && isValid(valid, tokenPath) && isIdentifierChar(lexer.Lookahead()) {
		// The path-expression token is only elected when the general PATH
		// context is also eligible. This avoids speculatively consuming an
		// ordinary identifier on runtimes that do not rewind a failed external
		// scan before the next GLR branch.
		lexer.Advance(false)
		for isIdentifierChar(lexer.Lookahead()) {
			lexer.Advance(false)
		}
		if lexer.Lookahead() == '/' {
			lexer.MarkEnd()
			lexer.Advance(false)
			// Inspect the path's final component before choosing the token
			// class. The constructor delimiter follows the component
			// (`Tree/Node {`), not the slash itself.
			for isIdentifierChar(lexer.Lookahead()) {
				lexer.Advance(false)
			}
			if isValid(valid, tokenPath) && (lexer.Lookahead() == '{' || lexer.Lookahead() == ':') {
				return s.emit(lexer, tokenPath)
			}
			if isValid(valid, tokenPathExpr) {
				return s.emit(lexer, tokenPathExpr)
			}
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
	return r >= '0' && r <= '9' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r == '.' || r == '-' || r == '_'
}
