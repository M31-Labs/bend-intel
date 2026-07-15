package intel

import (
	"fmt"

	"github.com/M31-Labs/bend-intel/bendlang"
	gotreesitter "github.com/odvcencio/gotreesitter"
)

type TextChange struct {
	Range *Range
	Text  string
}

// ApplyChanges applies LSP changes in order and reparses the document. The
// transport is incremental even when gotreesitter conservatively chooses a
// full parse for Bend's stateful indentation scanner.
func (d *Document) ApplyChanges(changes []TextChange) error {
	for _, change := range changes {
		if change.Range == nil {
			parsed, err := Parse([]byte(change.Text))
			if err != nil {
				return err
			}
			d.Source, d.Tree, d.language, d.treeSource, d.scopeGraph = parsed.Source, parsed.Tree, parsed.language, parsed.treeSource, nil
			continue
		}
		start, ok := d.Offset(change.Range.Start)
		if !ok {
			return fmt.Errorf("invalid change start: %+v", change.Range.Start)
		}
		end, ok := d.Offset(change.Range.End)
		if !ok || end < start {
			return fmt.Errorf("invalid change end: %+v", change.Range.End)
		}
		// A recovery tree is a structural hypothesis, not a safe incremental
		// reuse base. Reparse from the edited source when the old tree carried
		// an error/stop or was built from masked bytes; this avoids preserving a
		// stale GLR branch when an edit completes previously invalid syntax.
		freshRequired := d.Recovered() || d.Tree == nil || d.Tree.RootNode() == nil || d.Tree.RootNode().HasError() || d.Tree.ParseStopReason() != gotreesitter.ParseStopAccepted
		next := make([]byte, 0, len(d.Source)-(end-start)+len(change.Text))
		next = append(next, d.Source[:start]...)
		next = append(next, change.Text...)
		next = append(next, d.Source[end:]...)
		edit := gotreesitter.InputEdit{
			StartByte: uint32(start), OldEndByte: uint32(end), NewEndByte: uint32(start + len(change.Text)),
			StartPoint: pointAt(d.Source, start), OldEndPoint: pointAt(d.Source, end), NewEndPoint: pointAt(next, start+len(change.Text)),
		}
		parsedSource := d.treeSource
		if len(parsedSource) == 0 && len(d.Source) > 0 {
			parsedSource = d.Source
		}
		nextParsedSource := replaceBytes(parsedSource, start, end, []byte(change.Text))
		if freshRequired {
			parsed, err := Parse(next)
			if err != nil {
				return err
			}
			d.Source, d.Tree, d.language, d.treeSource, d.scopeGraph = parsed.Source, parsed.Tree, parsed.language, parsed.treeSource, nil
			continue
		}
		d.Tree.Edit(edit)
		parser, err := bendlang.NewParser()
		if err != nil {
			return err
		}
		tree, treeSource, err := parseIncrementalWithRecovery(parser, d.language, next, nextParsedSource, d.Tree)
		if err != nil {
			return fmt.Errorf("incrementally parse Bend: %w", err)
		}
		d.Source, d.Tree, d.treeSource, d.scopeGraph = next, tree, treeSource, nil
	}
	return nil
}

func replaceBytes(source []byte, start, end int, replacement []byte) []byte {
	next := make([]byte, 0, len(source)-(end-start)+len(replacement))
	next = append(next, source[:start]...)
	next = append(next, replacement...)
	next = append(next, source[end:]...)
	return next
}

func pointAt(source []byte, offset int) gotreesitter.Point {
	var point gotreesitter.Point
	if offset > len(source) {
		offset = len(source)
	}
	for _, b := range source[:offset] {
		if b == '\n' {
			point.Row++
			point.Column = 0
		} else {
			point.Column++
		}
	}
	return point
}
