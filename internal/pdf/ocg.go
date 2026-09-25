package pdf

// Optional content keys. The default configuration /OCProperties /D carries
// the /OCGs list and the /OFF list this subset reads. Alternate
// configurations, the /AS usage map, and the visibility flag stay out.
const (
	keyOC           = "OC"
	keyOCProperties = "OCProperties"
	keyOCGs         = "OCGs"
	keyOFF          = "OFF"
	keyD            = "D"

	policyAllOn  = "AllOn"
	policyAnyOn  = "AnyOn"
	policyAllOff = "AllOff"
	policyAnyOff = "AnyOff"

	opOCG = "OCProperties"
)

// ocOffGroups returns the object numbers whose optional content group is OFF
// in the default configuration. A document without /OCProperties returns an
// empty map, and every group is then visible.
func (res Resources) ocOffGroups() (map[int]bool, error) {
	if res.file == nil {
		return map[int]bool{}, nil
	}
	return res.file.ocOffGroups()
}

// ocOffGroups reads /OCProperties /D /OFF from the catalog. The /OFF entries
// are indirect references to optional content groups. A malformed
// /OCProperties is syntaxerror in OCProperties, so a page that uses the
// document still refuses by name.
func (file *File) ocOffGroups() (map[int]bool, error) {
	catalog, err := file.catalogNode()
	if err != nil {
		return nil, err
	}
	properties, err := file.ocProperties(catalog)
	if err != nil || properties.Kind == KindNull {
		return map[int]bool{}, err
	}
	def, err := file.ocDefault(properties)
	if err != nil || def.Kind == KindNull {
		return map[int]bool{}, err
	}
	return ocOffList(def), nil
}

// catalogNode returns the resolved document catalog. A missing /Root returns
// null, and the optional content stays empty.
func (file *File) catalogNode() (Value, error) {
	root, ok := file.trailer.ValueEntry(keyRoot)
	if !ok || root.Kind == KindNull {
		return NullVal(), nil
	}
	return file.deref(root)
}

// ocProperties returns the resolved /OCProperties dictionary, or null when
// the catalog has none. Any other kind is syntaxerror in OCProperties.
func (file *File) ocProperties(catalog Value) (Value, error) {
	entry, ok := catalog.ValueEntry(keyOCProperties)
	if !ok || entry.Kind == KindNull {
		return NullVal(), nil
	}
	node, err := file.deref(entry)
	if err != nil {
		return NullVal(), err
	}
	if node.Kind != KindDict {
		return NullVal(), NewError(opOCG, errSyntax)
	}
	return node, nil
}

// ocDefault returns the resolved /OCProperties /D dictionary, or null when it
// is absent. Any other kind is syntaxerror in OCProperties.
func (file *File) ocDefault(properties Value) (Value, error) {
	entry, ok := properties.ValueEntry(keyD)
	if !ok || entry.Kind == KindNull {
		return NullVal(), nil
	}
	node, err := file.deref(entry)
	if err != nil {
		return NullVal(), err
	}
	if node.Kind != KindDict {
		return NullVal(), NewError(opOCG, errSyntax)
	}
	return node, nil
}

// ocOffList collects the object numbers in one /OFF array.
func ocOffList(def Value) map[int]bool {
	off, ok := def.ArrayEntry(keyOFF)
	if !ok {
		return map[int]bool{}
	}
	out := make(map[int]bool, len(off))
	for _, item := range off {
		if item.Kind == KindRef {
			out[item.RefNum] = true
		}
	}
	return out
}

// ocHidden reports whether one BDC properties operand names an OFF group. raw
// is the operand as popped, so an indirect OCG keeps its object number. props
// is the resolved properties dictionary, where a nested /OC entry still holds
// its reference.
func (run *runner) ocHidden(props Value, raw item) (bool, error) {
	if len(run.ocOff) == 0 {
		return false, nil
	}
	if raw.kind == itemName {
		if entry, ok := run.properties[raw.name]; ok {
			hidden, err := run.ocEntryHidden(entry)
			if err != nil || hidden {
				return hidden, err
			}
		}
	}
	if entry, ok := props.ValueEntry(keyOC); ok && entry.Kind != KindNull {
		return run.ocEntryHidden(entry)
	}
	return run.ocNodeHidden(props), nil
}

// ocEntryHidden reports whether one /OC value is OFF. A direct reference
// compares against the OFF list; an indirect membership dictionary resolves
// and evaluates its /OCGs list.
func (run *runner) ocEntryHidden(entry Value) (bool, error) {
	if entry.Kind == KindRef && run.ocOff[entry.RefNum] {
		return true, nil
	}
	node, err := run.derefValue(entry, "BDC")
	if err != nil {
		return false, err
	}
	return run.ocNodeHidden(node), nil
}

// ocNodeHidden evaluates one resolved optional content membership dictionary
// through its /OCGs list and /P policy. A dictionary with no /OCGs list is an
// optional content group with no OFF entry, so it stays visible.
func (run *runner) ocNodeHidden(node Value) bool {
	ocgs, ok := node.ArrayEntry(keyOCGs)
	if !ok || len(ocgs) == 0 {
		return false
	}
	allOff, anyOff := true, false
	for _, item := range ocgs {
		if item.Kind == KindRef && run.ocOff[item.RefNum] {
			anyOff = true
			continue
		}
		allOff = false
	}
	policy, _ := node.NameEntry(keyP)
	return policyHides(policy, allOff, anyOff)
}

// policyHides maps one /P policy and the member states to hidden or visible.
// /AnyOn is the default: a group is hidden only when every member is off.
func policyHides(policy string, allOff, anyOff bool) bool {
	switch policy {
	case policyAllOn:
		return anyOff
	case policyAnyOff:
		return !anyOff
	case policyAllOff:
		return !allOff
	case policyAnyOn, "":
		return allOff
	default:
		return allOff
	}
}

// xobjectHidden reports whether one XObject dictionary names an OFF group
// through /OC. An absent entry keeps the XObject visible.
func (run *runner) xobjectHidden(val Value) (bool, error) {
	if len(run.ocOff) == 0 {
		return false, nil
	}
	entry, ok := val.ValueEntry(keyOC)
	if !ok || entry.Kind == KindNull {
		return false, nil
	}
	return run.ocEntryHidden(entry)
}
