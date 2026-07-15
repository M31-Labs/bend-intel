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

//go:embed queries/locals.scm
var locals string

//go:embed queries/tags.scm
var tags string

//go:embed queries/folds.scm
var folds string

//go:embed queries/indents.scm
var indents string

var language = sync.OnceValues(func() (*gotreesitter.Language, error) {
	lang, err := gotreesitter.LoadLanguage(blob)
	if err != nil {
		return nil, fmt.Errorf("load Bend grammar: %w", err)
	}
	if err := AttachExternalScanner(lang); err != nil {
		return nil, err
	}
	return lang, nil
})

var highlightQuery = sync.OnceValues(func() (*gotreesitter.Query, error) {
	lang, err := Language()
	if err != nil {
		return nil, err
	}
	query, err := gotreesitter.NewQuery(highlights, lang)
	if err != nil {
		return nil, fmt.Errorf("compile Bend highlights query: %w", err)
	}
	return query, nil
})

// Language returns the immutable Bend language definition.
func Language() (*gotreesitter.Language, error) { return language() }

// AttachExternalScanner installs Bend's stateful scanner on a language loaded
// from a grammar blob. It is useful for grammar-generation and parity tools;
// Language and NewParser call the same validation internally.
func AttachExternalScanner(lang *gotreesitter.Language) error {
	if lang == nil {
		return fmt.Errorf("nil Bend language")
	}
	if len(lang.ExternalSymbols) != bendTokenCount {
		return fmt.Errorf("Bend grammar has %d external symbols, want %d", len(lang.ExternalSymbols), bendTokenCount)
	}
	lang.ExternalScanner = newExternalScanner(lang.ExternalSymbols)
	return nil
}

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

func LocalsQuery() string  { return locals }
func TagsQuery() string    { return tags }
func FoldsQuery() string   { return folds }
func IndentsQuery() string { return indents }

// Highlights returns the compiled Bend highlight query.
func Highlights() (*gotreesitter.Query, error) { return highlightQuery() }
