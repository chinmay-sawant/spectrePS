package pdf

import (
	"bytes"
	"context"

	"github.com/chinmay-sawant/spectrePS/internal/graphics"
)

const (
	errRange = "rangecheck"
	opRaster = "RasterizePage"

	keyPage     = "Page"
	keyPages    = "Pages"
	keyKids     = "Kids"
	keyContents = "Contents"
)

// PageCount returns the number of page leaves walked from the page tree.
func (file *File) PageCount() int {
	if file == nil {
		return 0
	}
	return len(file.pages)
}

// Content returns the decoded content bytes for a zero-based page.
// A bad index is rangecheck.
func (file *File) Content(index int) ([]byte, error) {
	if file == nil || index < 0 || index >= len(file.pages) {
		return nil, NewError(opRaster, errRange)
	}
	return cloneBytes(file.pages[index]), nil
}

// PaintPage paints one page onto pixmap. It does not call ShowPage.
func (file *File) PaintPage(ctx context.Context, index int, pixmap *graphics.Pixmap, scale float64) error {
	if ctx == nil {
		panic(panicNilCtx)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	content, err := file.Content(index)
	if err != nil {
		return err
	}
	return Paint(ctx, content, pixmap, scale)
}

func (file *File) walkRoot() ([][]byte, error) {
	root, ok := file.trailer.ValueEntry(keyRoot)
	if !ok || root.Kind == KindNull {
		return nil, NewError(opPDF, errSyntax)
	}
	catalog, err := file.deref(root)
	if err != nil {
		return nil, err
	}
	pages, ok := catalog.ValueEntry(keyPages)
	if !ok || pages.Kind == KindNull {
		return nil, NewError(opPDF, errSyntax)
	}
	return file.walkRef(pages, map[int]bool{})
}

func (file *File) walkRef(val Value, seen map[int]bool) ([][]byte, error) {
	if val.Kind == KindRef {
		if seen[val.RefNum] {
			return nil, NewError(opPDF, errSyntax)
		}
		seen[val.RefNum] = true
	}
	node, err := file.deref(val)
	if err != nil {
		return nil, err
	}
	return file.walkNode(node, seen)
}

func (file *File) walkNode(node Value, seen map[int]bool) ([][]byte, error) {
	typeName, _ := node.NameEntry(keyType)
	if typeName == keyPage {
		return file.leaf(node)
	}
	kids, hasKids := node.ArrayEntry(keyKids)
	if typeName != keyPages && !hasKids {
		return nil, NewError(opPDF, errSyntax)
	}
	if !hasKids {
		return nil, NewError(opPDF, errSyntax)
	}
	return file.walkKids(kids, seen)
}

func (file *File) leaf(node Value) ([][]byte, error) {
	content, err := file.pageBytes(node)
	if err != nil {
		return nil, err
	}
	return [][]byte{content}, nil
}

func (file *File) walkKids(kids []Value, seen map[int]bool) ([][]byte, error) {
	pages := make([][]byte, 0, len(kids))
	for _, kid := range kids {
		sub, err := file.walkRef(kid, seen)
		if err != nil {
			return nil, err
		}
		pages = append(pages, sub...)
	}
	return pages, nil
}

func (file *File) pageBytes(page Value) ([]byte, error) {
	contents, ok := page.ValueEntry(keyContents)
	if !ok || contents.Kind == KindNull {
		return []byte{}, nil
	}
	if contents.Kind == KindArray {
		return file.joinContents(contents.Array)
	}
	return file.oneContent(contents)
}

func (file *File) joinContents(items []Value) ([]byte, error) {
	parts := make([][]byte, 0, len(items))
	for _, item := range items {
		part, err := file.oneContent(item)
		if err != nil {
			return nil, err
		}
		parts = append(parts, part)
	}
	return bytes.Join(parts, []byte{'\n'}), nil
}

func (file *File) oneContent(val Value) ([]byte, error) {
	stream, err := file.deref(val)
	if err != nil {
		return nil, err
	}
	return decodeStream(stream)
}

func decodeStream(val Value) ([]byte, error) {
	if val.Kind != KindStream {
		return nil, NewError(opPDF, errType)
	}
	filter, ok := val.ValueEntry(keyFilter)
	if !ok || filter.Kind == KindNull {
		return cloneBytes(val.Stream), nil
	}
	parms := NullVal()
	if entry, found := val.ValueEntry(keyParms); found {
		parms = entry
	}
	if filter.Kind == KindName {
		return Decode(filter.Name, paramAt(parms, 0, false), val.Stream)
	}
	if filter.Kind == KindArray {
		return decodeChain(filter.Array, parms, val.Stream)
	}
	return nil, NewError(opPDF, errType)
}

func decodeChain(filters []Value, parms Value, raw []byte) ([]byte, error) {
	current := raw
	for idx, filter := range filters {
		if filter.Kind != KindName {
			return nil, NewError(opPDF, errType)
		}
		decoded, err := Decode(filter.Name, paramAt(parms, idx, true), current)
		if err != nil {
			return nil, err
		}
		current = decoded
	}
	return current, nil
}

func paramAt(parms Value, index int, many bool) Value {
	if parms.Kind == KindNull {
		return NullVal()
	}
	if parms.Kind == KindArray {
		return arrayParam(parms.Array, index)
	}
	if many {
		return NullVal()
	}
	return parms
}

func arrayParam(items []Value, index int) Value {
	if index < 0 || index >= len(items) || items[index].Kind == KindNull {
		return NullVal()
	}
	return items[index]
}

func cloneBytes(raw []byte) []byte {
	out := make([]byte, len(raw))
	copy(out, raw)
	return out
}
