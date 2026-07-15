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
			d.Source, d.Tree, d.language = parsed.Source, parsed.Tree, parsed.language
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
		next := make([]byte, 0, len(d.Source)-(end-start)+len(change.Text))
		next = append(next, d.Source[:start]...)
		next = append(next, change.Text...)
		next = append(next, d.Source[end:]...)
		edit := gotreesitter.InputEdit{
			StartByte: uint32(start), OldEndByte: uint32(end), NewEndByte: uint32(start + len(change.Text)),
			StartPoint: pointAt(d.Source, start), OldEndPoint: pointAt(d.Source, end), NewEndPoint: pointAt(next, start+len(change.Text)),
		}
		d.Tree.Edit(edit)
		parser, err := bendlang.NewParser()
		if err != nil {
			return err
		}
		tree, err := parser.ParseIncremental(next, d.Tree)
		if err != nil {
			return fmt.Errorf("incrementally parse Bend: %w", err)
		}
		d.Source, d.Tree = next, tree
	}
	return nil
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
