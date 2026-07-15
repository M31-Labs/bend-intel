package intel

import (
	"bytes"
	"sort"

	gotreesitter "github.com/odvcencio/gotreesitter"
)

// parseWithRecovery keeps the archived Tree-sitter grammar useful while the
// Bend compiler syntax is evolving. It first parses the original bytes. When
// a current-style typed `def`/`type` header is the only obstruction, it masks
// the type spelling with same-length spaces and reparses. Ranges still point
// into the user's original source, while the compiler remains authoritative
// for the masked semantics.
func parseWithRecovery(parser *gotreesitter.Parser, lang *gotreesitter.Language, source []byte) (*gotreesitter.Tree, []byte, error) {
	tree, err := parser.Parse(source)
	if err != nil {
		return nil, nil, err
	}
	return chooseRecoveredTree(parser, lang, source, source, tree)
}

func chooseRecoveredTree(parser *gotreesitter.Parser, lang *gotreesitter.Language, original, parsedSource []byte, tree *gotreesitter.Tree) (*gotreesitter.Tree, []byte, error) {
	bestTree, bestSource := tree, parsedSource
	bestErrors, bestMissing := recoveryScore(original, tree, lang)
	// A clean CST is already the strongest result. Do not replace it with a
	// masked compatibility tree merely because the heuristic outline pass did
	// not recognize a newer top-level spelling; that would report recovery for
	// valid current Bend and discard compiler-faithful structure.
	if bestErrors == 0 {
		return bestTree, bestSource, nil
	}
	masked, changed := maskCurrentSyntax(original)
	if changed && !bytes.Equal(masked, parsedSource) {
		bestTree, bestSource, bestErrors, bestMissing = chooseCandidate(parser, lang, original, masked, bestTree, bestSource, bestErrors, bestMissing)
	}
	// A path token can be valid in a pattern and still cause the pure-Go GLR
	// runtime to abandon an expression-level stack (for example
	// `return List/Nil`). Mask only the separator, preserving every byte range
	// while allowing the ordinary identifier DFA to carry the structural tree.
	pathsMasked := maskCurrentPathSeparators(original)
	if !bytes.Equal(pathsMasked, original) {
		bestTree, bestSource, bestErrors, bestMissing = chooseCandidate(parser, lang, original, pathsMasked, bestTree, bestSource, bestErrors, bestMissing)
	}
	if changed {
		pathsAfterSyntax := maskCurrentPathSeparators(masked)
		if !bytes.Equal(pathsAfterSyntax, masked) {
			bestTree, bestSource, bestErrors, bestMissing = chooseCandidate(parser, lang, original, pathsAfterSyntax, bestTree, bestSource, bestErrors, bestMissing)
		}
	}
	if bestMissing > 0 {
		constructorMasked := maskCurrentConstructorReturns(masked)
		if !bytes.Equal(constructorMasked, masked) {
			bestTree, bestSource, bestErrors, bestMissing = chooseCandidate(parser, lang, original, constructorMasked, bestTree, bestSource, bestErrors, bestMissing)
		}
	}
	if bestMissing > 0 {
		// The archived external scanner can lose the next top-level definition
		// when a line comment follows a recovered indentation block. A comment
		// mask is a last-resort structural fallback; it keeps line/byte ranges
		// stable while leaving the original source available for highlighting.
		commentsMasked := maskCurrentComments(bestSource)
		if !bytes.Equal(commentsMasked, bestSource) {
			bestTree, bestSource, bestErrors, bestMissing = chooseCandidate(parser, lang, original, commentsMasked, bestTree, bestSource, bestErrors, bestMissing)
		}
	}
	// A few large, path-heavy imperative bodies can exhaust the GLR survivor
	// set before the next top-level definition. As a final editor-only
	// fallback, keep each definition header and replace its body with one
	// range-preserving `return 0` placeholder. This is considered only after a
	// stopped/truncated parse, is marked as recovered, and never competes with
	// an accepted exact tree. It gives symbols, scopes and navigation a complete
	// document while Bend remains authoritative for the hidden body semantics.
	if needsBodyResync(bestTree, len(original)) {
		bodyMasked := maskCurrentDefinitionBodies(masked)
		if !bytes.Equal(bodyMasked, masked) {
			bestTree, bestSource, bestErrors, bestMissing = chooseCandidate(parser, lang, original, bodyMasked, bestTree, bestSource, bestErrors, bestMissing)
		}
	}
	return bestTree, bestSource, nil
}

func needsBodyResync(tree *gotreesitter.Tree, sourceLen int) bool {
	if tree == nil || tree.RootNode() == nil {
		return true
	}
	return tree.RootNode().HasError() || tree.ParseStopReason() != gotreesitter.ParseStopAccepted || int(tree.RootNode().EndByte()) < sourceLen
}

// maskCurrentDefinitionBodies preserves top-level imperative definition
// headers and source offsets while replacing bodies that are too ambiguous for
// a stopped editor parse. It is deliberately conservative: functional
// equation rules (which use `=` rather than a colon body) are left untouched.
func maskCurrentDefinitionBodies(source []byte) []byte {
	lines := bytes.SplitAfter(source, []byte{'\n'})
	starts := make([]int, 0)
	for i, line := range lines {
		if topLevelDefinitionLine(line) {
			starts = append(starts, i)
		}
	}
	masked := append([]byte(nil), source...)
	offsets := make([]int, len(lines)+1)
	for i, line := range lines {
		offsets[i+1] = offsets[i] + len(line)
	}
	for si, start := range starts {
		line := trimLineEnding(lines[start])
		if !wordAt(line, 0, "def") || bytes.Contains(line, []byte{'='}) || !bytes.Contains(line, []byte{':'}) {
			continue
		}
		end := len(lines)
		if si+1 < len(starts) {
			end = starts[si+1]
		}
		firstBody := -1
		for i := start + 1; i < end; i++ {
			body := trimLineEnding(lines[i])
			if len(bytes.TrimSpace(body)) == 0 || bytes.HasPrefix(bytes.TrimSpace(body), []byte{'#'}) {
				continue
			}
			firstBody = i
			break
		}
		if firstBody < 0 {
			continue
		}
		for i := start + 1; i < end; i++ {
			line := lines[i]
			limit := len(trimLineEnding(line))
			if limit == 0 {
				continue
			}
			for j := 0; j < limit; j++ {
				masked[offsets[i]+j] = ' '
			}
			if i != firstBody {
				continue
			}
			indent := 0
			for indent < limit && (line[indent] == ' ' || line[indent] == '\t') {
				indent++
			}
			placeholder := []byte("return 0")
			if limit-indent < len(placeholder) {
				placeholder = []byte("0")
			}
			copy(masked[offsets[i]+indent:], placeholder)
		}
	}
	return masked
}

func trimLineEnding(line []byte) []byte {
	if len(line) > 0 && line[len(line)-1] == '\n' {
		line = line[:len(line)-1]
	}
	if len(line) > 0 && line[len(line)-1] == '\r' {
		line = line[:len(line)-1]
	}
	return line
}

func topLevelDefinitionLine(line []byte) bool {
	line = trimLineEnding(line)
	if len(line) == 0 || line[0] == ' ' || line[0] == '\t' || line[0] == '#' {
		return false
	}
	for _, keyword := range []string{"def", "type", "object", "hvm", "import", "from", "unchecked", "const"} {
		if wordAt(line, 0, keyword) {
			return true
		}
	}
	return topLevelEquals(line) >= 0
}

// maskCurrentSyntax applies only range-preserving lexical shims for syntax
// that is present in the current Bend examples but absent from the archived
// Tree-sitter grammar. It deliberately leaves the original source untouched;
// the compiler remains responsible for deciding whether the spelling is valid.
func maskCurrentSyntax(source []byte) ([]byte, bool) {
	masked, changed := maskCurrentTypes(source)
	changed = maskFunTypeFields(source, masked) || changed
	changed = maskFunctionalSignatures(source, masked) || changed
	changed = maskCurrentKeywords(source, masked) || changed
	changed = maskCurrentShifts(source, masked) || changed
	return masked, changed
}

func maskFunTypeFields(source, masked []byte) bool {
	changed := false
	for lineStart := 0; lineStart < len(source); {
		lineEnd := bytes.IndexByte(source[lineStart:], '\n')
		if lineEnd < 0 {
			lineEnd = len(source)
		} else {
			lineEnd += lineStart
		}
		cursor := lineStart
		for cursor < lineEnd && (source[cursor] == ' ' || source[cursor] == '\t') {
			cursor++
		}
		if wordAt(source, cursor, "type") {
			eq := bytes.IndexByte(source[cursor:lineEnd], '=')
			if eq >= 0 {
				eq += cursor
				for i := eq + 1; i < lineEnd; i++ {
					if source[i] != ':' {
						continue
					}
					nameEnd := i
					for nameEnd > eq && (source[nameEnd-1] == ' ' || source[nameEnd-1] == '\t') {
						nameEnd--
					}
					nameStart := nameEnd
					for nameStart > eq && isNameByte(source[nameStart-1]) {
						nameStart--
					}
					if nameStart == nameEnd {
						continue
					}
					open := nameStart - 1
					for open > eq && (source[open] == ' ' || source[open] == '\t') {
						open--
					}
					if open <= eq || source[open] != '(' {
						continue
					}
					close := matchingDelimiter(source[:lineEnd], open, '(', ')')
					if close < 0 {
						continue
					}
					changed = maskRange(masked, open, nameStart) || changed
					changed = maskRange(masked, i, close+1) || changed
					i = close
				}
			}
		}
		if lineEnd == len(source) {
			break
		}
		lineStart = lineEnd + 1
	}
	return changed
}

func maskFunctionalSignatures(source, masked []byte) bool {
	changed := false
	for lineStart := 0; lineStart < len(source); {
		lineEnd := bytes.IndexByte(source[lineStart:], '\n')
		if lineEnd < 0 {
			lineEnd = len(source)
		} else {
			lineEnd += lineStart
		}
		cursor := lineStart
		for cursor < lineEnd && (source[cursor] == ' ' || source[cursor] == '\t') {
			cursor++
		}
		if cursor < lineEnd && !wordAt(source, cursor, "def") && !wordAt(source, cursor, "type") && !wordAt(source, cursor, "object") && !wordAt(source, cursor, "hvm") && !wordAt(source, cursor, "import") && !wordAt(source, cursor, "from") {
			line := source[cursor:lineEnd]
			if bytes.Contains(line, []byte("->")) && topLevelEquals(line) < 0 {
				changed = maskRange(masked, lineStart, lineEnd) || changed
			}
		}
		if lineEnd == len(source) {
			break
		}
		lineStart = lineEnd + 1
	}
	return changed
}

func chooseCandidate(parser *gotreesitter.Parser, lang *gotreesitter.Language, original, candidateSource []byte, bestTree *gotreesitter.Tree, bestSource []byte, bestErrors, bestMissing int) (*gotreesitter.Tree, []byte, int, int) {
	// A parser that has just produced an error tree can retain recovery
	// checkpoints. Reusing it for a range-preserving candidate can truncate the
	// candidate at the first prior failure (notably after an indentation block).
	// Candidate parses are cheap compared with editor latency and must start
	// from a clean parser state.
	candidateParser := gotreesitter.NewParser(lang)
	candidate, err := candidateParser.Parse(candidateSource)
	if err != nil {
		return bestTree, bestSource, bestErrors, bestMissing
	}
	errors, missing := recoveryScore(original, candidate, lang)
	if errors < bestErrors || errors == bestErrors && missing < bestMissing {
		return candidate, candidateSource, errors, missing
	}
	return bestTree, bestSource, bestErrors, bestMissing
}

func parseIncrementalWithRecovery(parser *gotreesitter.Parser, lang *gotreesitter.Language, original, parsedSource []byte, oldTree *gotreesitter.Tree) (*gotreesitter.Tree, []byte, error) {
	tree, err := parser.ParseIncremental(parsedSource, oldTree)
	if err != nil {
		return nil, nil, err
	}
	return chooseRecoveredTree(parser, lang, original, parsedSource, tree)
}

func recoveryScore(source []byte, tree *gotreesitter.Tree, lang *gotreesitter.Language) (int, int) {
	if tree == nil || tree.RootNode() == nil {
		return 1, 0
	}
	errors := errorCount(tree.RootNode())
	// HasError also covers a parser failure materialized only in the root
	// compatibility metadata. ParseStopReason catches the more dangerous
	// silent-prefix case where no named ERROR node was materialized at all.
	if tree.RootNode().HasError() || tree.ParseStopReason() != gotreesitter.ParseStopAccepted || int(tree.RootNode().EndByte()) < len(source) {
		errors++
	}
	return errors, len(missingDefinitionLines(source, tree.RootNode(), lang))
}

func missingDefinitionLines(source []byte, root *gotreesitter.Node, lang *gotreesitter.Language) []uint32 {
	needed := map[uint32]bool{}
	for lineStart := 0; lineStart < len(source); {
		lineEnd := bytes.IndexByte(source[lineStart:], '\n')
		if lineEnd < 0 {
			lineEnd = len(source)
		} else {
			lineEnd += lineStart
		}
		cursor := lineStart
		for cursor < lineEnd && (source[cursor] == ' ' || source[cursor] == '\t') {
			cursor++
		}
		// Only unindented definitions belong to the book-level recovery score.
		// Assignments inside a body are ordinary expressions and must not be
		// reported as missing top-level definitions.
		if cursor == lineStart && definitionCandidateLine(source, cursor, lineEnd) {
			line := uint32(bytes.Count(source[:cursor], []byte{'\n'}))
			needed[line] = true
		}
		if lineEnd == len(source) {
			break
		}
		lineStart = lineEnd + 1
	}
	walkTree(root, func(node *gotreesitter.Node) {
		typ := node.Type(lang)
		if typ == "imp_function_definition" || typ == "fun_function_definition" || typ == "hvm_definition" {
			delete(needed, node.StartPoint().Row)
		}
	})
	lines := make([]uint32, 0, len(needed))
	for line := range needed {
		lines = append(lines, line)
	}
	sort.Slice(lines, func(i, j int) bool { return lines[i] < lines[j] })
	return lines
}

func definitionCandidateLine(source []byte, start, end int) bool {
	if wordAt(source, start, "def") || wordAt(source, start, "unchecked") {
		return true
	}
	if wordAt(source, start, "type") || wordAt(source, start, "hvm") || wordAt(source, start, "import") || wordAt(source, start, "from") {
		return false
	}
	for i := start; i < end; i++ {
		if source[i] == '=' && (i+1 >= end || source[i+1] != '=') && (i == start || source[i-1] != '=') {
			return true
		}
	}
	return false
}

func maskCurrentKeywords(source, masked []byte) bool {
	changed := false
	for lineStart := 0; lineStart < len(source); {
		lineEnd := bytes.IndexByte(source[lineStart:], '\n')
		if lineEnd < 0 {
			lineEnd = len(source)
		} else {
			lineEnd += lineStart
		}
		cursor := lineStart
		for cursor < lineEnd && (source[cursor] == ' ' || source[cursor] == '\t') {
			cursor++
		}
		for _, keyword := range []string{"unchecked", "const"} {
			if wordAt(source, cursor, keyword) {
				changed = maskRange(masked, cursor, cursor+len(keyword)) || changed
				break
			}
		}
		if lineEnd == len(source) {
			break
		}
		lineStart = lineEnd + 1
	}
	return changed
}

func maskCurrentShifts(source, masked []byte) bool {
	changed := false
	for lineStart := 0; lineStart < len(source); {
		lineEnd := bytes.IndexByte(source[lineStart:], '\n')
		if lineEnd < 0 {
			lineEnd = len(source)
		} else {
			lineEnd += lineStart
		}
		inString := byte(0)
		for i := lineStart; i+1 < lineEnd; i++ {
			if source[i] == '#' && inString == 0 {
				break
			}
			switch source[i] {
			case '"', '\'':
				if inString == 0 {
					inString = source[i]
				} else if inString == source[i] && (i == lineStart || source[i-1] != '\\') {
					inString = 0
				}
			}
			if inString != 0 || source[i] != '<' && source[i] != '>' || source[i+1] != source[i] {
				continue
			}
			prev := i - 1
			for prev >= lineStart && (source[prev] == ' ' || source[prev] == '\t') {
				prev--
			}
			if prev < lineStart || source[prev] == '(' || source[prev] == ',' {
				continue
			}
			end := i + 2
			for end < lineEnd && (source[end] == ' ' || source[end] == '\t') {
				end++
			}
			if end < lineEnd && source[end] == '(' {
				if close := matchingDelimiter(source[:lineEnd], end, '(', ')'); close >= 0 {
					end = close + 1
				}
			} else {
				for end < lineEnd && isExpressionByte(source[end]) {
					end++
				}
			}
			changed = maskRange(masked, i, end) || changed
			i = end - 1
		}
		if lineEnd == len(source) {
			break
		}
		lineStart = lineEnd + 1
	}
	return changed
}

func maskCurrentComments(source []byte) []byte {
	masked := append([]byte(nil), source...)
	for lineStart := 0; lineStart < len(source); {
		lineEnd := bytes.IndexByte(source[lineStart:], '\n')
		if lineEnd < 0 {
			lineEnd = len(source)
		} else {
			lineEnd += lineStart
		}
		inString := byte(0)
		for i := lineStart; i < lineEnd; i++ {
			switch source[i] {
			case '"', '\'':
				if inString == 0 {
					inString = source[i]
				} else if inString == source[i] && (i == lineStart || source[i-1] != '\\') {
					inString = 0
				}
			case '#':
				if inString == 0 {
					maskRange(masked, i, lineEnd)
					i = lineEnd
				}
			}
		}
		if lineEnd == len(source) {
			break
		}
		lineStart = lineEnd + 1
	}
	return masked
}

func maskCurrentPathSeparators(source []byte) []byte {
	masked := append([]byte(nil), source...)
	var quote byte
	for i := 0; i < len(source); i++ {
		c := source[i]
		if quote != 0 {
			if c == '\\' {
				i++
				continue
			}
			if c == quote {
				quote = 0
			}
			continue
		}
		if c == '"' || c == '\'' {
			quote = c
			continue
		}
		if c == '#' {
			for i < len(source) && source[i] != '\n' {
				i++
			}
			continue
		}
		if c != '/' || i == 0 || i+1 >= len(source) || !isPathWordByte(source[i-1]) || !isPathWordByte(source[i+1]) {
			continue
		}
		masked[i] = '_'
	}
	return masked
}

func isPathWordByte(b byte) bool {
	return b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= '0' && b <= '9' || b == '_' || b == '.' || b == '-'
}

func isExpressionByte(b byte) bool {
	return isWordByte(b) || b == '.' || b == '+' || b == '-'
}

func walkTree(node *gotreesitter.Node, visit func(*gotreesitter.Node)) {
	if node == nil {
		return
	}
	visit(node)
	for i := 0; i < node.NamedChildCount(); i++ {
		walkTree(node.NamedChild(i), visit)
	}
}

func errorCount(node *gotreesitter.Node) int {
	if node == nil {
		return 0
	}
	count := 0
	if node.IsError() {
		count++
	}
	for i := 0; i < node.NamedChildCount(); i++ {
		count += errorCount(node.NamedChild(i))
	}
	return count
}

func maskCurrentTypes(source []byte) ([]byte, bool) {
	masked := append([]byte(nil), source...)
	changed := false
	typeIndent := -1
	for lineStart := 0; lineStart < len(source); {
		lineEnd := bytes.IndexByte(source[lineStart:], '\n')
		if lineEnd < 0 {
			lineEnd = len(source)
		} else {
			lineEnd += lineStart
		}
		cursor := lineStart
		for cursor < lineEnd && (source[cursor] == ' ' || source[cursor] == '\t') {
			cursor++
		}
		indent := cursor - lineStart
		trimmed := cursor < lineEnd && source[cursor] != '#'
		if typeIndent >= 0 && trimmed && indent <= typeIndent && !wordAt(source, cursor, "type") {
			typeIndent = -1
		}
		switch {
		case wordAt(source, cursor, "def"):
			changed = maskDefHeader(source, masked, cursor, lineEnd) || changed
		case wordAt(source, cursor, "type"):
			changed = maskTypeParameters(source, masked, cursor, lineEnd) || changed
			changed = maskFunTypeParameters(source, masked, cursor, lineEnd) || changed
			if headerColon(source, cursor+len("type")) >= 0 {
				typeIndent = indent
			}
		default:
			if typeIndent >= 0 && indent > typeIndent {
				changed = maskBracedFieldTypes(source, masked, lineStart, lineEnd) || changed
			}
			if indent == 0 && trimmed {
				changed = maskFunHeaderTypes(source, masked, cursor, lineEnd) || changed
			}
		}
		if lineEnd == len(source) {
			break
		}
		lineStart = lineEnd + 1
	}
	return masked, changed
}

func maskBracedFieldTypes(source, masked []byte, start, end int) bool {
	changed := false
	braceDepth := 0
	for i := start; i < end; i++ {
		switch source[i] {
		case '{':
			braceDepth++
		case '}':
			if braceDepth > 0 {
				braceDepth--
			}
		case ':':
			if braceDepth == 0 {
				continue
			}
			limit := topLevelBraceCommaOrEnd(source, i+1, end)
			changed = maskRange(masked, i, limit) || changed
			i = limit - 1
		}
	}
	return changed
}

func topLevelBraceCommaOrEnd(source []byte, start, end int) int {
	depth := 0
	for i := start; i < end; i++ {
		switch source[i] {
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			if depth == 0 {
				return i
			}
			depth--
		case ',':
			if depth == 0 {
				return i
			}
		}
	}
	return end
}

func maskDefHeader(source, masked []byte, start, lineEnd int) bool {
	open := bytes.IndexByte(source[start:lineEnd], '(')
	arrow := bytes.Index(source[start:lineEnd], []byte("->"))
	if open >= 0 {
		open += start
	}
	if arrow >= 0 {
		arrow += start
	}
	if open < 0 || arrow >= 0 && open > arrow {
		colon := headerColon(source, start+len("def"))
		if colon < 0 {
			return false
		}
		nameEnd := start + len("def")
		for nameEnd < colon && (source[nameEnd] == ' ' || source[nameEnd] == '\t') {
			nameEnd++
		}
		for nameEnd < colon && source[nameEnd] != ' ' && source[nameEnd] != '\t' && source[nameEnd] != '-' {
			nameEnd++
		}
		return maskRange(masked, nameEnd, colon)
	}
	close := matchingDelimiter(source, open, '(', ')')
	if close < 0 {
		return false
	}
	changed := maskParameterTypes(source, masked, open+1, close)
	colon := headerColon(source, close+1)
	if colon >= 0 {
		changed = maskRange(masked, close+1, colon) || changed
	}
	return changed
}

func maskParameterTypes(source, masked []byte, start, end int) bool {
	changed := false
	depth := 0
	for i := start; i < end; i++ {
		switch source[i] {
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			if depth > 0 {
				depth--
			}
		case ':':
			if depth != 0 {
				continue
			}
			limit := topLevelCommaOrEnd(source, i+1, end)
			changed = maskRange(masked, i, limit) || changed
			i = limit - 1
		}
	}
	return changed
}

func maskTypeParameters(source, masked []byte, start, lineEnd int) bool {
	open := bytes.IndexByte(source[start:lineEnd], '(')
	if open < 0 {
		return false
	}
	open += start
	close := matchingDelimiter(source, open, '(', ')')
	if close < 0 || headerColon(source, close+1) < 0 {
		return false
	}
	return maskRange(masked, open, close+1)
}

func maskFunTypeParameters(source, masked []byte, start, lineEnd int) bool {
	eq := bytes.IndexByte(source[start:lineEnd], '=')
	if eq < 0 {
		return false
	}
	eq += start
	nameEnd := start + len("type")
	for nameEnd < eq && (source[nameEnd] == ' ' || source[nameEnd] == '\t') {
		nameEnd++
	}
	for nameEnd < eq && source[nameEnd] != ' ' && source[nameEnd] != '\t' {
		nameEnd++
	}
	if nameEnd >= eq {
		return false
	}
	return maskRange(masked, nameEnd, eq)
}

func maskFunHeaderTypes(source, masked []byte, start, lineEnd int) bool {
	eq := bytes.IndexByte(source[start:lineEnd], '=')
	if eq < 0 {
		return false
	}
	eq += start
	changed := false
	for i := start; i < eq; i++ {
		if source[i] != '(' {
			continue
		}
		close := matchingDelimiter(source, i, '(', ')')
		if close < 0 || close > eq {
			continue
		}
		changed = maskParameterTypes(source, masked, i+1, close) || changed
		i = close
	}
	depth := 0
	for i := start; i < eq; i++ {
		switch source[i] {
		case '(':
			depth++
		case ')':
			if depth > 0 {
				depth--
			}
		case ':':
			if depth == 0 {
				changed = maskRange(masked, i, eq) || changed
				return changed
			}
		}
	}
	return changed
}

func maskCurrentConstructorReturns(source []byte) []byte {
	masked := append([]byte(nil), source...)
	for lineStart := 0; lineStart < len(source); {
		lineEnd := bytes.IndexByte(source[lineStart:], '\n')
		if lineEnd < 0 {
			lineEnd = len(source)
		} else {
			lineEnd += lineStart
		}
		cursor := lineStart
		for cursor < lineEnd && (source[cursor] == ' ' || source[cursor] == '\t') {
			cursor++
		}
		if wordAt(source, cursor, "return") {
			slash := bytes.IndexByte(source[cursor+len("return"):lineEnd], '/')
			if slash >= 0 {
				slash += cursor + len("return")
				maskRange(masked, slash, lineEnd)
			}
		}
		if lineEnd == len(source) {
			break
		}
		lineStart = lineEnd + 1
	}
	return masked
}

func topLevelCommaOrEnd(source []byte, start, end int) int {
	depth := 0
	for i := start; i < end; i++ {
		switch source[i] {
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			if depth > 0 {
				depth--
			}
		case ',':
			if depth == 0 {
				return i
			}
		}
	}
	return end
}

func headerColon(source []byte, start int) int {
	depth := 0
	for i := start; i < len(source); i++ {
		switch source[i] {
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			if depth > 0 {
				depth--
			}
		case ':':
			if depth == 0 {
				return i
			}
		case '\n':
			if depth == 0 {
				return -1
			}
		}
	}
	return -1
}

func matchingDelimiter(source []byte, start int, open, close byte) int {
	depth := 0
	for i := start; i < len(source); i++ {
		switch source[i] {
		case open:
			depth++
		case close:
			depth--
			if depth == 0 {
				return i
			}
		case '\n':
			// Headers can span lines, so newlines are intentionally retained.
		}
	}
	return -1
}

func maskRange(masked []byte, start, end int) bool {
	changed := false
	for i := start; i < end && i < len(masked); i++ {
		if masked[i] != ' ' && masked[i] != '\t' && masked[i] != '\n' && masked[i] != '\r' {
			masked[i] = ' '
			changed = true
		}
	}
	return changed
}

func wordAt(source []byte, start int, word string) bool {
	if start < 0 || start+len(word) > len(source) || !bytes.Equal(source[start:start+len(word)], []byte(word)) {
		return false
	}
	end := start + len(word)
	return (end == len(source) || !isWordByte(source[end])) && (start == 0 || !isWordByte(source[start-1]))
}

func isWordByte(b byte) bool {
	return b == '_' || b >= '0' && b <= '9' || b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z'
}
