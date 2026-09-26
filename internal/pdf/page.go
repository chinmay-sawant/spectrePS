package pdf

import (
	"bytes"
	"context"

	"github.com/chinmay-sawant/spectrePS/internal/graphics"
)

const (
	errRange = "rangecheck"
	opRaster = "RasterizePage"

	keyPage      = "Page"
	keyPages     = "Pages"
	keyKids      = "Kids"
	keyContents  = "Contents"
	keyResources = "Resources"
	keyXObject   = "XObject"
)

// pageLeaf is one page leaf from the tree walk: the decoded content bytes and
// the nearest /Resources, either the page's own or an ancestor's.
type pageLeaf struct {
	content   []byte
	resources Value
}

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

// PageContentNums returns the object numbers of the content streams for a
// zero-based page, in /Contents order. A content entry that is not an indirect
// reference contributes no number. A bad index is rangecheck.
func (file *File) PageContentNums(pageIndex int) ([]int, error) {
	if file == nil || pageIndex < 0 || pageIndex >= len(file.pages) {
		return nil, NewError(opPDF, errRange)
	}
	groups, err := file.contentNumPages()
	if err != nil {
		return nil, err
	}
	if pageIndex >= len(groups) {
		return nil, NewError(opPDF, errSyntax)
	}
	return groups[pageIndex], nil
}

// contentNumPages walks the page tree and lists the content reference numbers
// per page, in page order.
func (file *File) contentNumPages() ([][]int, error) {
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
		return nil, NewError(opPDF, errUndefined)
	}
	return file.walkRefNums(pages, map[int]bool{})
}

func (file *File) walkRefNums(val Value, seen map[int]bool) ([][]int, error) {
	if val.Kind == KindRef {
		if seen[val.RefNum] {
			return nil, NewError(opPDF, errLimit)
		}
		seen[val.RefNum] = true
	}
	node, err := file.deref(val)
	if err != nil {
		return nil, err
	}
	return file.walkNodeNums(node, seen)
}

func (file *File) walkNodeNums(node Value, seen map[int]bool) ([][]int, error) {
	typeName, _ := node.NameEntry(keyType)
	if typeName == keyPage {
		return [][]int{contentNumRefs(node)}, nil
	}
	kids, hasKids := node.ArrayEntry(keyKids)
	if typeName != keyPages && !hasKids {
		return nil, NewError(opPDF, errSyntax)
	}
	if !hasKids {
		return nil, NewError(opPDF, errSyntax)
	}
	pages := make([][]int, 0, len(kids))
	for _, kid := range kids {
		sub, err := file.walkRefNums(kid, seen)
		if err != nil {
			return nil, err
		}
		pages = append(pages, sub...)
	}
	return pages, nil
}

// contentNumRefs lists the indirect content references on one page dictionary.
func contentNumRefs(node Value) []int {
	contents, ok := node.ValueEntry(keyContents)
	if !ok || contents.Kind == KindNull {
		return nil
	}
	if contents.Kind == KindArray {
		return arrayRefNums(contents.Array)
	}
	if contents.Kind == KindRef {
		return []int{contents.RefNum}
	}
	return nil
}

func arrayRefNums(items []Value) []int {
	nums := make([]int, 0, len(items))
	for _, item := range items {
		if item.Kind == KindRef {
			nums = append(nums, item.RefNum)
		}
	}
	return nums
}

// PaintPage paints one page onto marker. It does not call ShowPage.
func (file *File) PaintPage(ctx context.Context, index int, marker graphics.Marker, scale float64) error {
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
	res, err := file.PageResources(index)
	if err != nil {
		return err
	}
	return PaintWith(ctx, content, marker, scale, PaintOptions{
		Resources:     res,
		Text:          TextOptions{Fonts: nil, Sink: nil, Runs: nil},
		MarkedContent: nil,
	})
}

func (file *File) walkRoot() ([]pageLeaf, error) {
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
		return nil, NewError(opPDF, errUndefined)
	}
	return file.walkRef(pages, map[int]bool{}, NullVal())
}

func (file *File) walkRef(val Value, seen map[int]bool, resources Value) ([]pageLeaf, error) {
	if val.Kind == KindRef {
		if seen[val.RefNum] {
			return nil, NewError(opPDF, errLimit)
		}
		seen[val.RefNum] = true
	}
	node, err := file.deref(val)
	if err != nil {
		return nil, err
	}
	return file.walkNode(node, seen, resources)
}

func (file *File) walkNode(node Value, seen map[int]bool, inherited Value) ([]pageLeaf, error) {
	resources := nearestResources(node, inherited)
	typeName, _ := node.NameEntry(keyType)
	if typeName == keyPage {
		return file.leaf(node, resources)
	}
	kids, hasKids := node.ArrayEntry(keyKids)
	if typeName != keyPages && !hasKids {
		return nil, NewError(opPDF, errSyntax)
	}
	if !hasKids {
		return nil, NewError(opPDF, errSyntax)
	}
	return file.walkKids(kids, seen, resources)
}

// nearestResources returns the node's own /Resources, or the nearest ancestor's
// when the node has none.
func nearestResources(node Value, inherited Value) Value {
	entry, ok := node.ValueEntry(keyResources)
	if !ok || entry.Kind == KindNull {
		return inherited
	}
	return entry
}

func (file *File) leaf(node Value, resources Value) ([]pageLeaf, error) {
	content, err := file.pageBytes(node)
	if err != nil {
		return nil, err
	}
	return []pageLeaf{{content: content, resources: resources}}, nil
}

func (file *File) walkKids(kids []Value, seen map[int]bool, resources Value) ([]pageLeaf, error) {
	pages := make([]pageLeaf, 0, len(kids))
	for _, kid := range kids {
		sub, err := file.walkRef(kid, seen, resources)
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
	stream, err := file.streamEntry(val)
	if err != nil {
		return nil, err
	}
	return decodeStream(stream)
}

// streamEntry resolves one stream entry. A parsed stream whose /Length is a
// direct integer is returned as it stands. An indirect /Length, or a plain
// parse that failed, goes through the xref-aware reparse, so a body that
// contains the bytes endstream still reads its declared span.
func (file *File) streamEntry(val Value) (Value, error) {
	stream, err := file.deref(val)
	if err == nil && !streamIndirectLength(stream) {
		return stream, nil
	}
	if fixed, ok := file.resolvedStream(val); ok {
		return fixed, nil
	}
	return stream, err
}

// streamIndirectLength reports whether a parsed stream still needs an indirect
// /Length resolved. A direct length was already read by the plain parse.
func streamIndirectLength(stream Value) bool {
	if stream.Kind != KindStream {
		return false
	}
	length, ok := stream.ValueEntry(wordLength)
	return ok && length.Kind == KindRef
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
