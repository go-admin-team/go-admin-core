package main

import (
	"bytes"
	"fmt"
	"go/format"
	"go/parser"
	"go/token"
	"sort"
	"strconv"
)

// change is one import line the tool would replace, reported whether or not
// the file is written.
type change struct {
	Line    int
	From    string
	To      string
	Aliased bool
}

// rewrite returns src with every deprecated import replaced. It parses rather
// than substituting text so that the path appearing in a comment, a string
// literal or a struct tag is left alone; only the import block is touched, and
// the rest of the file keeps its formatting byte for byte.
func rewrite(filename string, src []byte) ([]byte, []change, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, filename, src, parser.ImportsOnly|parser.ParseComments)
	if err != nil {
		return nil, nil, fmt.Errorf("%s: %w", filename, err)
	}

	type edit struct {
		start, end int
		text       string
	}

	var edits []edit
	var changes []change

	for _, spec := range f.Imports {
		path, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			continue
		}

		m, ok := lookup(path)
		if !ok {
			continue
		}

		// The local name has to survive the move. An alias or a dot import
		// already fixes it, so only a plain import of a renamed package needs
		// one added — and the name it needs is free by construction, because
		// the import being replaced was using it.
		added := spec.Name == nil && m.name != m.newName
		name := ""
		switch {
		case spec.Name != nil:
			name = spec.Name.Name
		case added:
			name = m.name
		}

		text := strconv.Quote(m.to)
		if name != "" {
			text = name + " " + text
		}

		edits = append(edits, edit{
			start: fset.Position(spec.Pos()).Offset,
			end:   fset.Position(spec.End()).Offset,
			text:  text,
		})
		changes = append(changes, change{
			Line:    fset.Position(spec.Pos()).Line,
			From:    path,
			To:      m.to,
			Aliased: added,
		})
	}

	if len(edits) == 0 {
		return src, nil, nil
	}

	// Applied back to front so that earlier offsets stay valid.
	sort.Slice(edits, func(i, j int) bool { return edits[i].start > edits[j].start })
	out := src
	for _, e := range edits {
		merged := make([]byte, 0, len(out)+len(e.text))
		merged = append(merged, out[:e.start]...)
		merged = append(merged, e.text...)
		merged = append(merged, out[e.end:]...)
		out = merged
	}

	// A path that changes usually sorts somewhere else, and leaving it where
	// the old one sat makes the block unsorted. gofmt sorts each group, so the
	// result is formatted — but only if the input was, because reformatting a
	// file the tool did not have to touch buries the migration in noise.
	if formatted, err := format.Source(out); err == nil {
		if clean, err := format.Source(src); err == nil && bytes.Equal(clean, src) {
			out = formatted
		}
	}

	return out, changes, nil
}

func lookup(path string) (move, bool) {
	for _, m := range moves {
		if m.from == path {
			return m, true
		}
	}
	return move{}, false
}
