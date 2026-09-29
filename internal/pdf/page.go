package pdf

import (
	"bytes"
	"context"
	"errors"
	"maps"

	"github.com/chinmay-sawant/spectrePS/internal/graphics"
)

const (
	errRange = "rangecheck"
	opRaster = "RasterizePage"

	// contentArrayDepth caps a /Contents array nested through indirect arrays.
	// A real producer nests one level; the cap turns a cycle into limitcheck.
	contentArrayDepth = 8

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
	return file.walkRefNums(pages, map[int]bool{}, map[int]bool{})
}

// walkRefNums mirrors walkRef for the content-number walk: seen skips a node a
// second /Kids entry names, and path still refuses a cycle.
func (file *File) walkRefNums(val Value, seen, path map[int]bool) ([][]int, error) {
	if val.Kind == KindRef {
		if path[val.RefNum] {
			return nil, NewError(opPDF, errLimit)
		}
		if seen[val.RefNum] {
			return nil, nil
		}
		seen[val.RefNum] = true
		path[val.RefNum] = true
		defer delete(path, val.RefNum)
	}
	node, err := file.deref(val)
	if err != nil {
		return nil, err
	}
	return file.walkNodeNums(node, seen, path)
}

func (file *File) walkNodeNums(node Value, seen, path map[int]bool) ([][]int, error) {
	typeName, _ := node.NameEntry(keyType)
	if typeName == keyPage {
		return [][]int{file.contentNumRefs(node)}, nil
	}
	kids, hasKids, err := file.kidArray(node)
	if err != nil {
		return nil, err
	}
	if !hasKids {
		if typeName != keyPages {
			return nil, NewError(opPDF, errSyntax)
		}
		return nil, nil
	}
	pages := make([][]int, 0, len(kids))
	for _, kid := range kids {
		sub, err := file.walkRefNums(kid, seen, path)
		if err != nil {
			return nil, err
		}
		pages = append(pages, sub...)
	}
	return pages, nil
}

// contentNumRefs lists the indirect content references on one page dictionary.
// A page whose /Contents names the array by an indirect reference reports the
// array's own numbers, so a consumer never takes the array for a stream.
func (file *File) contentNumRefs(node Value) []int {
	contents, ok := node.ValueEntry(keyContents)
	if !ok || contents.Kind == KindNull {
		return nil
	}
	if items, isArray := file.contentsArray(contents); isArray {
		return arrayRefNums(items)
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
	return file.walkRef(pages, map[int]bool{}, map[int]bool{}, NullVal())
}

// walkRef walks one page-tree node. seen holds every object number already
// walked, so a node that appears twice under /Kids contributes its pages once
// and the walk stays bounded by the object count. path holds the object numbers
// on the current branch, so a node that is its own ancestor is the cycle that is
// still a limitcheck.
func (file *File) walkRef(val Value, seen, path map[int]bool, resources Value) ([]pageLeaf, error) {
	if val.Kind == KindRef {
		if path[val.RefNum] {
			return nil, NewError(opPDF, errLimit)
		}
		if seen[val.RefNum] {
			return nil, nil
		}
		seen[val.RefNum] = true
		path[val.RefNum] = true
		defer delete(path, val.RefNum)
	}
	node, err := file.deref(val)
	if err != nil {
		return nil, err
	}
	return file.walkNode(node, seen, path, resources)
}

func (file *File) walkNode(node Value, seen, path map[int]bool, inherited Value) ([]pageLeaf, error) {
	resources := nearestResources(node, inherited)
	typeName, _ := node.NameEntry(keyType)
	if typeName == keyPage {
		return file.leaf(node, resources)
	}
	kids, hasKids, err := file.kidArray(node)
	if err != nil {
		return nil, err
	}
	if !hasKids {
		if typeName != keyPages {
			return nil, NewError(opPDF, errSyntax)
		}
		// A /Pages node with no /Kids is an empty subtree. Ghostscript counts
		// the pages below it as none, so the walk ends here instead of
		// refusing the file.
		return nil, nil
	}
	return file.walkKids(kids, seen, path, resources)
}

// kidArray returns the /Kids array of one page-tree node. A /Kids written as
// an indirect reference to an array is resolved, because that is what real
// producers emit and Ghostscript resolves it.
func (file *File) kidArray(node Value) ([]Value, bool, error) {
	entry, ok := node.ValueEntry(keyKids)
	if !ok || entry.Kind == KindNull {
		return nil, false, nil
	}
	if entry.Kind == KindRef {
		resolved, err := file.deref(entry)
		if err != nil {
			return nil, false, err
		}
		entry = resolved
	}
	if entry.Kind != KindArray {
		return nil, false, nil
	}
	return entry.Array, true, nil
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

func (file *File) walkKids(kids []Value, seen, path map[int]bool, resources Value) ([]pageLeaf, error) {
	pages := make([]pageLeaf, 0, len(kids))
	for _, kid := range kids {
		sub, err := file.walkRef(kid, seen, path, resources)
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
	// The reference itself goes to oneContent, which needs it to reparse a
	// stream with an indirect /Length.
	if items, isArray := file.contentsArray(contents); isArray {
		return file.joinContents(items)
	}
	return file.oneContent(contents)
}

// contentsArray returns the content stream array of one page. /Contents holds
// the array directly or names it by an indirect reference, so a reference is
// resolved to read the kind. A reference that does not resolve, or resolves to
// anything but an array, is not the array.
func (file *File) contentsArray(contents Value) ([]Value, bool) {
	if contents.Kind == KindArray {
		return contents.Array, true
	}
	if contents.Kind != KindRef {
		return nil, false
	}
	resolved, err := file.deref(contents)
	if err != nil || resolved.Kind != KindArray {
		return nil, false
	}
	return resolved.Array, true
}

func (file *File) joinContents(items []Value) ([]byte, error) {
	flat, err := file.flattenContents(items, 0)
	if err != nil {
		return nil, err
	}
	parts := make([][]byte, 0, len(flat))
	for _, item := range flat {
		part, err := file.oneContent(item)
		if err != nil {
			return nil, err
		}
		parts = append(parts, part)
	}
	return bytes.Join(parts, []byte{'\n'}), nil
}

// flattenContents splices a nested /Contents array into one stream list. The
// spec writes /Contents as one stream or one array of streams, but real
// producers point an array entry at another array and Ghostscript walks it,
// so the reader does too.
func (file *File) flattenContents(items []Value, depth int) ([]Value, error) {
	if depth > contentArrayDepth {
		return nil, NewError(opPDF, errLimit)
	}
	flat := make([]Value, 0, len(items))
	for _, item := range items {
		if item.Kind == KindRef {
			resolved, err := file.deref(item)
			if err != nil || resolved.Kind != KindArray {
				flat = append(flat, item)
				continue
			}
			item = resolved
		}
		if item.Kind != KindArray {
			flat = append(flat, item)
			continue
		}
		nested, err := file.flattenContents(item.Array, depth+1)
		if err != nil {
			return nil, err
		}
		flat = append(flat, nested...)
	}
	return flat, nil
}

func (file *File) oneContent(val Value) ([]byte, error) {
	stream, err := file.streamEntry(val)
	if err != nil {
		if file.invalidRowRef(val) {
			// The row is in use and its bytes are not the object the row claims,
			// and the object header scan did not find the object either.
			// Ghostscript reports this as an invalid xref entry, rebuilds the
			// table, and paints the page without content when the rebuild does
			// not find it. A content stream the file cannot place is not a
			// reason to refuse a document Ghostscript renders.
			return []byte{}, nil
		}
		return nil, err
	}
	decoded, err := decodeStream(file.resolvedStreamEntries(stream))
	if err != nil && recoverableContentError(err) {
		// A content stream this build has no decoder for, or one that expands
		// past the decode cap, is a page-local loss. Ghostscript reports the
		// filter and paints the rest of the page, so the document still opens.
		// A malformed stream stays a document error.
		return []byte{}, nil
	}
	return decoded, err
}

// resolvedStreamEntries returns a copy of a stream value whose /Filter and
// /DecodeParms are resolved when they are indirect. A producer may write
// /Filter 30 0 R and Ghostscript resolves it. The copy keeps the cached
// stream value untouched.
func (file *File) resolvedStreamEntries(val Value) Value {
	filter, hasFilter := val.ValueEntry(keyFilter)
	if !hasFilter {
		return val
	}
	parms, hasParms := val.ValueEntry(keyParms)
	wantFilter := filter.Kind == KindRef
	wantParms := hasParms && parms.Kind == KindRef
	if !wantFilter && !wantParms {
		return val
	}
	dict := make(map[string]Value, len(val.Dict))
	maps.Copy(dict, val.Dict)
	if wantFilter {
		if resolved, err := file.deref(filter); err == nil {
			dict[keyFilter] = resolved
		}
	}
	if wantParms {
		if resolved, err := file.deref(parms); err == nil {
			dict[keyParms] = resolved
		}
	}
	val.Dict = dict
	return val
}

// recoverableContentError reports whether a content stream decode failure is
// one Ghostscript treats as a page-local loss: a missing decoder, or a stream
// that hit the decoder's own resource cap. A malformed stream is not.
func recoverableContentError(err error) bool {
	var jobErr *Error
	if !errors.As(err, &jobErr) {
		return false
	}
	return jobErr.Name == errUndefined || jobErr.Name == errLimit
}

// invalidRowRef reports whether ref names an in-use xref row whose bytes are
// not the object the row claims. A row that is absent, free, or carries another
// generation is not this case: that reference is a dead dependency and still
// fails, because the file never carried the object at all.
func (file *File) invalidRowRef(ref Value) bool {
	if ref.Kind != KindRef {
		return false
	}
	entry, ok := file.xref[ref.RefNum]
	if !ok || !entry.InUse || !genOK(entry, ref.RefGen) {
		return false
	}
	if entry.Compressed {
		_, ok := file.objStreamValue(ref.RefNum, entry.StreamNum, entry.StreamIdx)
		return !ok
	}
	_, err := file.plainAt(ref.RefNum, entry.Offset, entry.Gen)
	return err != nil
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
