package pdf

// This file holds the reader seams the tag writer needs: page object numbers,
// a loaded font's dictionary, the PDF 2.0 standard type predicate, and the
// parent tree key list. Nothing here paints or writes.

import "sort"

// PageObjectNums returns the object numbers of the page dictionaries in
// PageCount order. A nil file, or a page tree the reader cannot walk, returns
// nil.
func (file *File) PageObjectNums() []int {
	if file == nil {
		return nil
	}
	nums, _, err := file.pageTable()
	if err != nil {
		return nil
	}
	return nums
}

// PageFontDict returns the resolved font dictionary for one /Font resource
// name on a zero-based page. It pairs with PageResources(...).Font(name): the
// loaded font comes from the resources, and TaggedFontOK reads the dictionary
// this returns. The bool is false when the page has no such name. A bad page
// index is rangecheck.
func (file *File) PageFontDict(index int, name string) (Value, bool, error) {
	if file == nil || index < 0 || index >= len(file.resources) {
		return NullVal(), false, NewError(opRaster, errRange)
	}
	node, err := file.deref(file.resources[index])
	if err != nil || node.Kind != KindDict {
		return NullVal(), false, err
	}
	sub, err := file.fontSubdict(node)
	if err != nil || sub.Kind != KindDict {
		return NullVal(), false, err
	}
	return file.namedFont(sub, name)
}

// namedFont resolves one /Font entry to its font dictionary.
func (file *File) namedFont(sub Value, name string) (Value, bool, error) {
	entry, ok := sub.ValueEntry(name)
	if !ok || entry.Kind == KindNull {
		return NullVal(), false, nil
	}
	font, err := file.deref(entry)
	if err != nil {
		return NullVal(), false, err
	}
	if font.Kind != KindDict {
		return NullVal(), false, nil
	}
	return font, true, nil
}

// fontSubdict resolves the /Font subdictionary of one resources dictionary.
func (file *File) fontSubdict(node Value) (Value, error) {
	entry, ok := node.ValueEntry(keyFont)
	if !ok || entry.Kind == KindNull {
		return NullVal(), nil
	}
	return file.deref(entry)
}

// StandardTypePDF20 reports whether one structure type name is standard in
// the PDF 2.0 structure namespace, ISO 32000-2 Table 365 through Table 375.
// TOC and TOCI are PDF 1.7 types that ISO 32000-2 Annex M removes, so they
// are not standard here.
func StandardTypePDF20(name string) bool {
	return pdf20StandardType(name)
}

// Keys returns every marked-content claim the parent tree carries, sorted by
// page index and then MCID. A nil tree returns nil. The result is a copy, so
// the caller can sort or keep it.
func (tree *StructTree) Keys() []StructKey {
	if tree == nil {
		return nil
	}
	keys := make([]StructKey, 0, len(tree.byKey))
	for key := range tree.byKey {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].Page != keys[j].Page {
			return keys[i].Page < keys[j].Page
		}
		return keys[i].MCID < keys[j].MCID
	})
	return keys
}
