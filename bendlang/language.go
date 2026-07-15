package bendlang

import (
	_ "embed"
	"fmt"
	"sync"

	gotreesitter "github.com/odvcencio/gotreesitter"
)

//go:embed grammar/bend.bin
var blob []byte

//go:embed queries/highlights.scm
var highlights string

var language = sync.OnceValues(func() (*gotreesitter.Language, error) {
	lang, err := gotreesitter.LoadLanguage(blob)
	if err != nil {
		return nil, fmt.Errorf("load Bend grammar: %w", err)
	}
	if len(lang.ExternalSymbols) != bendTokenCount {
		return nil, fmt.Errorf("Bend grammar has %d external symbols, want %d", len(lang.ExternalSymbols), bendTokenCount)
	}
	lang.ExternalScanner = newExternalScanner(lang.ExternalSymbols)
	return lang, nil
})

// Language returns the immutable Bend language definition.
func Language() (*gotreesitter.Language, error) { return language() }

// NewParser returns a parser configured for Bend.
func NewParser() (*gotreesitter.Parser, error) {
	lang, err := Language()
	if err != nil {
		return nil, err
	}
	return gotreesitter.NewParser(lang), nil
}

// HighlightsQuery returns the upstream-compatible syntax highlighting query.
func HighlightsQuery() string { return highlights }
