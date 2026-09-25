package pdf

// PaintOptions carries the resolved page resources for one content stream.
type PaintOptions struct {
	// Resources holds the page resources the interpreter reads.
	Resources Resources
}

// Resources is the page resource subset the content interpreter reads.
// XObjects maps each /XObject name to its resolved value.
type Resources struct {
	// XObjects holds the /XObject subdictionary by name. A value is resolved
	// through its indirect reference, so the image stream is ready to decode.
	XObjects map[string]Value
}

// PageResources returns the effective resources of a zero-based page.
// The nearest /Resources on the page or a /Pages ancestor wins, and the
// /XObject subdictionary resolves through references.
// A bad index is rangecheck.
func (file *File) PageResources(index int) (Resources, error) {
	if file == nil || index < 0 || index >= len(file.resources) {
		return Resources{XObjects: nil}, NewError(opRaster, errRange)
	}
	return file.resolveResources(file.resources[index])
}

// resolveResources reads one /Resources value into the interpreter subset.
// A missing, null, or non-dictionary value yields no XObjects.
func (file *File) resolveResources(val Value) (Resources, error) {
	out := Resources{XObjects: map[string]Value{}}
	if val.Kind == KindNull {
		return out, nil
	}
	node, err := file.deref(val)
	if err != nil {
		return Resources{XObjects: nil}, err
	}
	if node.Kind != KindDict {
		return out, nil
	}
	entry, ok := node.ValueEntry(keyXObject)
	if !ok || entry.Kind == KindNull {
		return out, nil
	}
	sub, err := file.deref(entry)
	if err != nil {
		return Resources{XObjects: nil}, err
	}
	if sub.Kind != KindDict {
		return out, nil
	}
	for name, item := range sub.Dict {
		resolved, err := file.deref(item)
		if err != nil {
			return Resources{XObjects: nil}, err
		}
		out.XObjects[name] = resolved
	}
	return out, nil
}
