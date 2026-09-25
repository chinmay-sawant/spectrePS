package pdf

// This file holds the tagged PDF model: /MarkInfo, the structure tree under
// /StructTreeRoot, the /ParentTree lookup, the role maps, and the
// dictionary-level font check. Nothing here paints, extracts, or rewrites.

const (
	opStructTree = "StructTree"
	opParentTree = "ParentTree"
	opRoleMap    = "RoleMap"

	structDepthCap = 64
	roleChainCap   = 32

	keyMarkInfo       = "MarkInfo"
	keyMarked         = "Marked"
	keySuspects       = "Suspects"
	keyStructTreeRoot = "StructTreeRoot"
	keyParentTree     = "ParentTree"
	keyNums           = "Nums"
	keyRoleMap        = "RoleMap"
	keyNamespaces     = "Namespaces"
	keyRoleMapNS      = "RoleMapNS"
	keyNamespaceName  = "NS"
	keyK              = "K"
	keyS              = "S"
	keyP              = "P"
	keyPg             = "Pg"
	keyMCID           = "MCID"
	keyAlt            = "Alt"
	keyActualText     = "ActualText"
	keyLang           = "Lang"
	keyID             = "ID"
	keyStructParents  = "StructParents"
	keyMCR            = "MCR"
	keyToUnicode      = "ToUnicode"
	keyEncoding       = "Encoding"
	keyBaseEncoding   = "BaseEncoding"

	// The standard structure namespace names. NamespaceDefault is the
	// default (PDF 1.7) namespace and NamespacePDF20 is the PDF 2.0 namespace.
	NamespaceDefault = "http://iso.org/pdf/ssn"
	NamespacePDF20   = "http://iso.org/pdf2/ssn"
	NamespaceMathML  = "http://www.w3.org/1998/Math/MathML"

	nsDefaultSSN = NamespaceDefault
	nsPDF20SSN   = NamespacePDF20
	nsMathML     = NamespaceMathML

	documentType = "Document"
)

// defaultStandardType reports whether one structure type is standard in the
// default (PDF 1.7) namespace, ISO 32000-1 Table 333 and Table 334.
func defaultStandardType(name string) bool {
	switch name {
	case documentType, "Part", "Art", "Sect", "Div",
		"BlockQuote", "Caption", "TOC", "TOCI",
		"Index", "NonStruct", "Private",
		"P", "H", "H1", "H2", "H3", "H4", "H5", "H6",
		"L", "LI", "Lbl", "LBody",
		"Table", "TR", "TH", "TD", "THead", "TBody", "TFoot",
		"Span", "Quote", "Note", "Reference", "BibEntry", "Code",
		"Link", "Annot", "Ruby", "RB", "RT", "RP",
		"Warichu", "WT", "WP", "Figure", "Formula", "Form":
		return true
	default:
		return false
	}
}

// pdf20StandardType reports whether one structure type is standard in the
// PDF 2.0 namespace, ISO 32000-2 Table 368 and the PDF Association cheat
// sheet. It is the default set with the PDF 2.0 additions.
func pdf20StandardType(name string) bool {
	switch name {
	case "DocumentFragment", "Aside", "Title", "Toc", "Toci",
		"FENote", "Sub", "Em", "Strong", "Artifact":
		return true
	default:
		return defaultStandardType(name)
	}
}

// standardEncodingName reports whether one simple-font encoding maps character
// codes to glyph names without a /ToUnicode CMap.
func standardEncodingName(name string) bool {
	switch name {
	case "StandardEncoding", "WinAnsiEncoding", "MacRomanEncoding",
		"MacExpertEncoding":
		return true
	default:
		return false
	}
}

// MarkInfo is the catalog /MarkInfo dictionary.
type MarkInfo struct {
	Marked   bool
	Suspects bool
}

// StructKey identifies one marked-content item by page index and MCID. The
// page index is zero-based and follows the page tree.
type StructKey struct {
	Page int
	MCID int
}

// RoleNS is one structure type mapping from /RoleMapNS. The empty namespace
// is the default structure namespace.
type RoleNS struct {
	Type      string
	Namespace string
}

// Namespace is one PDF 2.0 structure namespace dictionary.
type Namespace struct {
	Name  string
	Roles map[string]RoleNS
}

// RoleMaps holds the root /RoleMap and the /RoleMapNS of every declared
// namespace.
type RoleMaps struct {
	defaultRoles map[string]string
	namespaces   map[string]map[string]RoleNS
}

// StructElem is one node of a structure tree. An element that owns marked
// content lists it in Items; an element that owns other elements lists them in
// Kids. Page is the /Pg page index, or the nearest inherited page, and -1 when
// the content page is unknown.
type StructElem struct {
	Object     int
	ParentNum  int
	Type       string
	Role       string
	RoleNS     string
	Namespace  string
	ID         string
	Alt        string
	ActualText string
	Lang       string
	Page       int
	Kids       []*StructElem
	Parent     *StructElem
	Items      []StructKey
}

// StructTree is one parsed structure tree.
type StructTree struct {
	MarkInfo      MarkInfo
	Lang          string
	Top           []*StructElem
	Roles         *RoleMaps
	Namespaces    map[string]*Namespace
	Parents       map[int]*StructElem
	pageNums      []int
	structParents map[int]int
	elems         map[int]*StructElem
	byKey         map[StructKey]*StructElem
	byElem        map[*StructElem]StructKey
}

// structClaim is one marked-content item a structure element claims through
// /K. The parent tree has to agree with every claim.
type structClaim struct {
	elem *StructElem
	key  StructKey
}

// structParser walks one structure tree and the parent tree behind it.
type structParser struct {
	file   *File
	tree   *StructTree
	claims []structClaim
}

// Root returns the first top-level structure element. A tagged document
// normally makes it /Document. It is nil when the tree has no kid.
func (tree *StructTree) Root() *StructElem {
	if tree == nil || len(tree.Top) == 0 {
		return nil
	}
	return tree.Top[0]
}

// Lookup returns the structure element that owns one marked-content item.
// A missing /ParentTree entry is undefined in ParentTree.
func (tree *StructTree) Lookup(key StructKey) (*StructElem, error) {
	if tree == nil {
		return nil, NewError(opParentTree, errUndefined)
	}
	elem, ok := tree.byKey[key]
	if !ok {
		return nil, NewError(opParentTree, errUndefined)
	}
	return elem, nil
}

// Key returns the marked-content item one structure element owns. The bool is
// false when the element owns none.
func (tree *StructTree) Key(elem *StructElem) (StructKey, bool) {
	if tree == nil || elem == nil {
		return StructKey{Page: 0, MCID: 0}, false
	}
	key, ok := tree.byElem[elem]
	return key, ok
}

// TextCover returns the nearest /ActualText on the element or an ancestor. It
// is false when none carries one.
func (elem *StructElem) TextCover() (string, bool) {
	for node := elem; node != nil; node = node.Parent {
		if node.ActualText != "" {
			return node.ActualText, true
		}
	}
	return "", false
}

// MarkInfo reads the catalog /MarkInfo dictionary. A missing catalog or entry
// returns the zero value.
func (file *File) MarkInfo() MarkInfo {
	entry, ok := file.catalogEntry(keyMarkInfo)
	if !ok {
		return MarkInfo{Marked: false, Suspects: false}
	}
	return MarkInfo{
		Marked:   boolEntry(entry, keyMarked),
		Suspects: boolEntry(entry, keySuspects),
	}
}

// HasStructTree reports whether the catalog carries a /StructTreeRoot entry or
// a true /MarkInfo /Marked.
func (file *File) HasStructTree() bool {
	if file == nil {
		return false
	}
	if _, ok := file.catalogEntry(keyStructTreeRoot); ok {
		return true
	}
	return file.MarkInfo().Marked
}

// StructTree parses the structure tree the catalog /StructTreeRoot names.
// A catalog without the entry returns nil and no error. A cycle or a depth
// past the cap is limitcheck.
func (file *File) StructTree() (*StructTree, error) {
	if file == nil {
		return nil, NewError(opStructTree, errType)
	}
	root, found, err := file.structRoot()
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, nil //nolint:nilnil // a catalog without /StructTreeRoot is not an error
	}
	return file.parseStructTree(root)
}

// structRoot resolves the catalog /StructTreeRoot dictionary. The bool is
// false when the catalog has no entry.
func (file *File) structRoot() (Value, bool, error) {
	catalog, err := file.catalogValue()
	if err != nil {
		return NullVal(), false, err
	}
	rootVal, ok := catalog.ValueEntry(keyStructTreeRoot)
	if !ok || rootVal.Kind == KindNull {
		return NullVal(), false, nil
	}
	root, err := file.deref(rootVal)
	if err != nil {
		return NullVal(), false, err
	}
	if root.Kind != KindDict {
		return NullVal(), false, NewError(opStructTree, errType)
	}
	return root, true, nil
}

// parseStructTree builds and walks one structure tree.
func (file *File) parseStructTree(root Value) (*StructTree, error) {
	catalog, err := file.catalogValue()
	if err != nil {
		return nil, err
	}
	pages, parents, err := file.pageTable()
	if err != nil {
		return nil, err
	}
	tree := &StructTree{
		MarkInfo:      file.MarkInfo(),
		Lang:          structureLang(catalog, root),
		Top:           nil,
		Roles:         nil,
		Namespaces:    nil,
		Parents:       map[int]*StructElem{},
		pageNums:      pages,
		structParents: parents,
		elems:         map[int]*StructElem{},
		byKey:         map[StructKey]*StructElem{},
		byElem:        map[*StructElem]StructKey{},
	}
	tree.Roles, tree.Namespaces, err = file.parseRoles(root)
	if err != nil {
		return nil, err
	}
	parser := &structParser{file: file, tree: tree, claims: nil}
	if err := parser.walk(root); err != nil {
		return nil, err
	}
	return tree, nil
}

// walk parses the root kids, the parent tree, and the claims between them.
func (parser *structParser) walk(root Value) error {
	top, err := parser.topKids(root)
	if err != nil {
		return err
	}
	parser.tree.Top = top
	if err := parser.buildParentTree(root); err != nil {
		return err
	}
	return parser.checkClaims()
}

// structureLang reads the structure tree language, then the catalog language.
func structureLang(catalog, root Value) string {
	if lang := stringEntry(root, keyLang); lang != "" {
		return lang
	}
	return stringEntry(catalog, keyLang)
}

// topKids parses the direct kids of the structure tree root.
func (parser *structParser) topKids(root Value) ([]*StructElem, error) {
	kids, ok := root.ValueEntry(keyK)
	if !ok || kids.Kind == KindNull {
		return nil, nil
	}
	items := kidItems(kids)
	top := make([]*StructElem, 0, len(items))
	busy := map[int]bool{}
	for _, item := range items {
		if item.Kind == KindNull {
			continue
		}
		if item.Kind != KindRef && item.Kind != KindDict {
			return nil, NewError(opStructTree, errType)
		}
		elem, err := parser.element(item, nil, 0, -1, busy)
		if err != nil {
			return nil, err
		}
		top = append(top, elem)
	}
	return top, nil
}

// element parses one structure element. A reference that is already parsed
// returns the stored node, so the parent tree and /K agree on identity.
func (parser *structParser) element(
	val Value,
	parent *StructElem,
	depth int,
	inheritPage int,
	busy map[int]bool,
) (*StructElem, error) {
	if depth > structDepthCap {
		return nil, NewError(opStructTree, errLimit)
	}
	objNum, done, err := parser.enterElement(val, parent, busy)
	if err != nil {
		return nil, err
	}
	if done != nil {
		return done, nil
	}
	if objNum != 0 {
		defer delete(busy, objNum)
	}
	return parser.readElement(val, objNum, parent, depth, inheritPage, busy)
}

// readElement parses the dictionary of one structure element and its kids.
func (parser *structParser) readElement(
	val Value,
	objNum int,
	parent *StructElem,
	depth int,
	inheritPage int,
	busy map[int]bool,
) (*StructElem, error) {
	node, err := parser.file.deref(val)
	if err != nil {
		return nil, err
	}
	if node.Kind != KindDict {
		return nil, NewError(opStructTree, errType)
	}
	elem, err := parser.newElem(node, objNum, parent, inheritPage)
	if err != nil {
		return nil, err
	}
	if objNum != 0 {
		parser.tree.elems[objNum] = elem
	}
	if err := parser.elementKids(node, elem, depth, busy); err != nil {
		return nil, err
	}
	parser.inheritPage(elem)
	return elem, nil
}

// inheritPage fills an element page from its first content item when no /Pg
// names one.
func (parser *structParser) inheritPage(elem *StructElem) {
	if elem.Page < 0 && len(elem.Items) > 0 {
		elem.Page = elem.Items[0].Page
	}
}

// enterElement marks a referenced element as in progress. It returns the
// stored element when the walk already parsed it, and object number 0 for a
// direct dictionary.
func (parser *structParser) enterElement(
	val Value,
	parent *StructElem,
	busy map[int]bool,
) (int, *StructElem, error) {
	if val.Kind != KindRef {
		return 0, nil, nil
	}
	objNum := val.RefNum
	if busy[objNum] {
		return 0, nil, NewError(opStructTree, errLimit)
	}
	if elem, ok := parser.tree.elems[objNum]; ok {
		if parent != nil {
			elem.Parent = parent
		}
		return 0, elem, nil
	}
	busy[objNum] = true
	return objNum, nil, nil
}

// newElem reads the entries of one structure element dictionary and resolves
// its type through the role maps.
func (parser *structParser) newElem(
	node Value,
	objNum int,
	parent *StructElem,
	inheritPage int,
) (*StructElem, error) {
	name, ok := node.NameEntry(keyS)
	if !ok || name == "" {
		return nil, NewError(opStructTree, errType)
	}
	namespace, err := parser.elementNamespace(node)
	if err != nil {
		return nil, err
	}
	role, roleNS, err := parser.tree.Roles.Resolve(name, namespace)
	if err != nil {
		return nil, err
	}
	elem := &StructElem{
		Object:     objNum,
		ParentNum:  0,
		Type:       name,
		Role:       role,
		RoleNS:     roleNS,
		Namespace:  namespace,
		ID:         stringEntry(node, keyID),
		Alt:        stringEntry(node, keyAlt),
		ActualText: stringEntry(node, keyActualText),
		Lang:       stringEntry(node, keyLang),
		Page:       inheritPage,
		Kids:       nil,
		Parent:     parent,
		Items:      nil,
	}
	if p, ok := node.ValueEntry(keyP); ok && p.Kind == KindRef {
		elem.ParentNum = p.RefNum
	}
	if pg, ok := node.ValueEntry(keyPg); ok && pg.Kind != KindNull {
		index, err := parser.pageIndex(pg)
		if err != nil {
			return nil, err
		}
		elem.Page = index
	}
	return elem, nil
}

// elementNamespace resolves the /NS entry of one structure element to a
// namespace name. An absent entry is the default namespace.
func (parser *structParser) elementNamespace(node Value) (string, error) {
	entry, ok := node.ValueEntry(keyNamespaceName)
	if !ok || entry.Kind == KindNull {
		return "", nil
	}
	name := parser.namespaceName(entry)
	if name == "" {
		return "", NewError(opStructTree, errType)
	}
	return name, nil
}

// namespaceName resolves a namespace reference, name, string, or array to the
// namespace name.
func (parser *structParser) namespaceName(val Value) string {
	switch val.Kind {
	case KindName:
		return val.Name
	case KindString:
		return val.String
	case KindRef:
		node, err := parser.file.deref(val)
		if err != nil || node.Kind != KindDict {
			return ""
		}
		return stringEntry(node, keyNamespaceName)
	case KindArray:
		for _, item := range val.Array {
			if name := parser.namespaceName(item); name != "" {
				return name
			}
		}
	case KindNull, KindBool, KindInt, KindReal, KindDict, KindStream:
		return ""
	}
	return ""
}

// elementKids walks the /K entry of one structure element.
func (parser *structParser) elementKids(
	node Value,
	elem *StructElem,
	depth int,
	busy map[int]bool,
) error {
	kids, ok := node.ValueEntry(keyK)
	if !ok || kids.Kind == KindNull {
		return nil
	}
	for _, item := range kidItems(kids) {
		if err := parser.kid(item, elem, depth, busy); err != nil {
			return err
		}
	}
	return nil
}

// kid parses one /K item: a structure element, a marked-content reference, or
// an integer MCID that uses the page of its element.
func (parser *structParser) kid(
	val Value,
	parent *StructElem,
	depth int,
	busy map[int]bool,
) error {
	if val.Kind == KindNull {
		return nil
	}
	if val.Kind == KindInt || val.Kind == KindReal {
		return parser.addContent(parent, val)
	}
	if val.Kind != KindRef && val.Kind != KindDict {
		return NewError(opStructTree, errType)
	}
	node, err := parser.file.deref(val)
	if err != nil {
		return err
	}
	if node.Kind != KindDict {
		return NewError(opStructTree, errType)
	}
	if markedContentRef(node) {
		return parser.addContent(parent, node)
	}
	child, err := parser.element(val, parent, depth+1, parent.Page, busy)
	if err != nil {
		return err
	}
	child.Parent = parent
	parent.Kids = append(parent.Kids, child)
	return nil
}

// addContent records one content item on its structure element.
func (parser *structParser) addContent(parent *StructElem, val Value) error {
	key, err := parser.contentKey(val, parent.Page)
	if err != nil {
		return err
	}
	parser.addItem(parent, key)
	return nil
}

// contentKey reads the /MCID and /Pg of one content item.
func (parser *structParser) contentKey(val Value, page int) (StructKey, error) {
	mcid, ok := mcidOf(val)
	if !ok {
		return StructKey{}, NewError(opStructTree, errType)
	}
	if mcid < 0 {
		return StructKey{}, NewError(opStructTree, errRange)
	}
	if val.Kind == KindDict {
		if pg, found := val.ValueEntry(keyPg); found && pg.Kind != KindNull {
			index, err := parser.pageIndex(pg)
			if err != nil {
				return StructKey{}, err
			}
			page = index
		}
	}
	if page < 0 {
		return StructKey{}, NewError(opStructTree, errSyntax)
	}
	return StructKey{Page: page, MCID: mcid}, nil
}

// pageIndex maps a /Pg reference to a page index.
func (parser *structParser) pageIndex(val Value) (int, error) {
	if val.Kind != KindRef {
		return -1, NewError(opStructTree, errType)
	}
	for index, num := range parser.tree.pageNums {
		if num == val.RefNum {
			return index, nil
		}
	}
	return -1, NewError(opStructTree, errSyntax)
}

// addItem records one marked-content item and the claim behind it.
func (parser *structParser) addItem(elem *StructElem, key StructKey) {
	elem.Items = append(elem.Items, key)
	parser.claims = append(parser.claims, structClaim{elem: elem, key: key})
}

// buildParentTree parses the /ParentTree of the structure tree root.
func (parser *structParser) buildParentTree(root Value) error {
	entry, ok := root.ValueEntry(keyParentTree)
	if !ok || entry.Kind == KindNull {
		return nil
	}
	node, err := parser.file.deref(entry)
	if err != nil {
		return err
	}
	if node.Kind != KindDict {
		return NewError(opParentTree, errType)
	}
	return parser.parentNode(node, 0)
}

// parentNode walks one number tree node of the parent tree.
func (parser *structParser) parentNode(node Value, depth int) error {
	if depth > structDepthCap {
		return NewError(opParentTree, errLimit)
	}
	if nums, ok := node.ArrayEntry(keyNums); ok {
		if err := parser.parentNums(nums); err != nil {
			return err
		}
	}
	return parser.parentKids(node, depth)
}

// parentNums records one /Nums array of the parent tree.
func (parser *structParser) parentNums(nums []Value) error {
	if len(nums)%2 != 0 {
		return NewError(opParentTree, errSyntax)
	}
	for i := 0; i+1 < len(nums); i += 2 {
		key, ok := intOf(nums[i])
		if !ok {
			return NewError(opParentTree, errSyntax)
		}
		if err := parser.parentEntry(key, nums[i+1]); err != nil {
			return err
		}
	}
	return nil
}

// parentKids walks the /Kids array of one parent tree node.
func (parser *structParser) parentKids(node Value, depth int) error {
	kids, ok := node.ArrayEntry(keyKids)
	if !ok {
		return nil
	}
	for _, kid := range kids {
		child, err := parser.file.deref(kid)
		if err != nil {
			return err
		}
		if child.Kind != KindDict {
			return NewError(opParentTree, errType)
		}
		if err := parser.parentNode(child, depth+1); err != nil {
			return err
		}
	}
	return nil
}

// parentEntry records one /Nums pair. An array value is the MCID-indexed list
// for one page; a single value is a structure parent.
func (parser *structParser) parentEntry(key int, val Value) error {
	node, err := parser.file.deref(val)
	if err != nil {
		return err
	}
	if node.Kind != KindArray {
		elem, err := parser.ensureElem(val)
		if err != nil {
			return err
		}
		parser.tree.Parents[key] = elem
		return nil
	}
	index := parser.pageForKey(key)
	if index < 0 {
		return nil
	}
	for mcid, item := range node.Array {
		if item.Kind == KindNull {
			continue
		}
		elem, err := parser.ensureElem(item)
		if err != nil {
			return err
		}
		sk := StructKey{Page: index, MCID: mcid}
		parser.tree.byKey[sk] = elem
		if _, seen := parser.tree.byElem[elem]; !seen {
			parser.tree.byElem[elem] = sk
		}
	}
	return nil
}

// pageForKey maps a parent tree key to a page index. The key is either the
// page object number or the /StructParents value of the page.
func (parser *structParser) pageForKey(key int) int {
	for index, num := range parser.tree.pageNums {
		if num == key {
			return index
		}
	}
	if index, ok := parser.tree.structParents[key]; ok {
		return index
	}
	return -1
}

// ensureElem returns a parsed structure element, and parses it when the tree
// walk has not seen it yet.
func (parser *structParser) ensureElem(val Value) (*StructElem, error) {
	if val.Kind == KindRef {
		if elem, ok := parser.tree.elems[val.RefNum]; ok {
			return elem, nil
		}
	}
	return parser.element(val, nil, 0, -1, map[int]bool{})
}

// checkClaims proves the parent tree agrees with every /K claim. A missing or
// different parent is undefined in ParentTree.
func (parser *structParser) checkClaims() error {
	for _, claim := range parser.claims {
		elem, ok := parser.tree.byKey[claim.key]
		if !ok || elem != claim.elem {
			return NewError(opParentTree, errUndefined)
		}
	}
	return nil
}

// parseRoles reads the root /RoleMap and every /Namespaces entry with its
// /RoleMapNS.
func (file *File) parseRoles(root Value) (*RoleMaps, map[string]*Namespace, error) {
	maps := &RoleMaps{
		defaultRoles: map[string]string{},
		namespaces:   map[string]map[string]RoleNS{},
	}
	namespaces := map[string]*Namespace{}
	if err := file.defaultRoles(root, maps); err != nil {
		return nil, nil, err
	}
	if err := file.roleNamespaces(root, maps, namespaces); err != nil {
		return nil, nil, err
	}
	return maps, namespaces, nil
}

// defaultRoles reads the root /RoleMap into maps.
func (file *File) defaultRoles(root Value, maps *RoleMaps) error {
	entry, ok := root.ValueEntry(keyRoleMap)
	if !ok || entry.Kind == KindNull {
		return nil
	}
	dict, err := file.deref(entry)
	if err != nil {
		return err
	}
	if dict.Kind != KindDict {
		return NewError(opRoleMap, errType)
	}
	for name, val := range dict.Dict {
		if val.Kind != KindName {
			return NewError(opRoleMap, errType)
		}
		maps.defaultRoles[name] = val.Name
	}
	return nil
}

// roleNamespaces reads the /Namespaces array with every /RoleMapNS.
func (file *File) roleNamespaces(
	root Value,
	maps *RoleMaps,
	namespaces map[string]*Namespace,
) error {
	entry, ok := root.ValueEntry(keyNamespaces)
	if !ok || entry.Kind == KindNull {
		return nil
	}
	items, err := file.valueArray(entry)
	if err != nil {
		return err
	}
	for _, item := range items {
		dict, err := file.deref(item)
		if err != nil {
			return err
		}
		if dict.Kind != KindDict {
			return NewError(opRoleMap, errType)
		}
		name := stringEntry(dict, keyNamespaceName)
		if name == "" {
			return NewError(opRoleMap, errType)
		}
		namespace := &Namespace{Name: name, Roles: map[string]RoleNS{}}
		if err := file.namespaceRoles(dict, namespace); err != nil {
			return err
		}
		namespaces[name] = namespace
		maps.namespaces[name] = namespace.Roles
	}
	return nil
}

// namespaceRoles reads one /RoleMapNS dictionary into a namespace.
func (file *File) namespaceRoles(dict Value, namespace *Namespace) error {
	entry, ok := dict.ValueEntry(keyRoleMapNS)
	if !ok || entry.Kind == KindNull {
		return nil
	}
	node, err := file.deref(entry)
	if err != nil {
		return err
	}
	if node.Kind != KindDict {
		return NewError(opRoleMap, errType)
	}
	for name, val := range node.Dict {
		role, err := file.roleEntry(val)
		if err != nil {
			return err
		}
		namespace.Roles[name] = role
	}
	return nil
}

// roleEntry reads one /RoleMapNS value: a target type, then the target
// namespace.
func (file *File) roleEntry(val Value) (RoleNS, error) {
	items, err := file.valueArray(val)
	if err != nil {
		return RoleNS{Type: "", Namespace: ""}, err
	}
	if len(items) == 0 || items[0].Kind != KindName {
		return RoleNS{Type: "", Namespace: ""}, NewError(opRoleMap, errType)
	}
	role := RoleNS{Type: items[0].Name, Namespace: ""}
	if len(items) > 1 {
		role.Namespace = file.namespaceValue(items[1])
	}
	return role, nil
}

// namespaceValue resolves one namespace target to a namespace name. A null
// target is the default namespace.
func (file *File) namespaceValue(val Value) string {
	switch val.Kind {
	case KindName:
		return val.Name
	case KindString:
		return val.String
	case KindRef:
		node, err := file.deref(val)
		if err != nil || node.Kind != KindDict {
			return ""
		}
		return stringEntry(node, keyNamespaceName)
	case KindArray:
		for _, item := range val.Array {
			if name := file.namespaceValue(item); name != "" {
				return name
			}
		}
	case KindNull, KindBool, KindInt, KindReal, KindDict, KindStream:
		return ""
	}
	return ""
}

// valueArray resolves a value that should be an array. A single dictionary
// becomes a one-item slice.
func (file *File) valueArray(val Value) ([]Value, error) {
	node, err := file.deref(val)
	if err != nil {
		return nil, err
	}
	if node.Kind == KindArray {
		return node.Array, nil
	}
	if node.Kind == KindDict {
		return []Value{node}, nil
	}
	return nil, NewError(opRoleMap, errType)
}

// Resolve maps one structure type used in one namespace to a standard type.
// The empty namespace is the default structure namespace. An unmapped custom
// type is undefined in RoleMap, a chain past the cap or a revisited pair is
// limitcheck, and a mapping that targets its own namespace is syntaxerror.
func (maps *RoleMaps) Resolve(name, namespace string) (string, string, error) {
	if maps == nil || name == "" {
		return "", "", NewError(opRoleMap, errType)
	}
	current, currentNS := name, namespaceKey(namespace)
	visited := map[string]bool{}
	for hop := 0; hop <= roleChainCap; hop++ {
		if standardType(current, currentNS) {
			return current, currentNS, nil
		}
		mark := currentNS + "\x00" + current
		if visited[mark] {
			return "", "", NewError(opRoleMap, errLimit)
		}
		visited[mark] = true
		target, targetNS, found, err := maps.step(current, currentNS)
		if err != nil {
			return "", "", err
		}
		if !found {
			return "", "", NewError(opRoleMap, errUndefined)
		}
		current, currentNS = target, targetNS
	}
	return "", "", NewError(opRoleMap, errLimit)
}

// step takes one role map hop. The bool is false when no mapping exists.
func (maps *RoleMaps) step(name, namespace string) (string, string, bool, error) {
	if namespace == "" {
		target, ok := maps.defaultRoles[name]
		if !ok {
			return "", "", false, nil
		}
		if target == name {
			return "", "", false, NewError(opRoleMap, errSyntax)
		}
		return target, "", true, nil
	}
	roles, ok := maps.namespaces[namespace]
	if !ok {
		return "", "", false, nil
	}
	role, ok := roles[name]
	if !ok {
		return "", "", false, nil
	}
	targetNS := namespaceKey(role.Namespace)
	if targetNS == namespace {
		return "", "", false, NewError(opRoleMap, errSyntax)
	}
	return role.Type, targetNS, true, nil
}

// namespaceKey folds the documented default namespace URL onto the empty
// namespace, so an explicit default declaration resolves like an absent one.
func namespaceKey(name string) string {
	if name == nsDefaultSSN {
		return ""
	}
	return name
}

// standardType reports whether one structure type is standard in one
// namespace. The default namespace is the empty string.
func standardType(name, namespace string) bool {
	switch namespace {
	case "":
		return defaultStandardType(name)
	case nsPDF20SSN:
		return pdf20StandardType(name)
	case nsMathML:
		return true
	default:
		return false
	}
}

// TaggedFontOK reports whether one font dictionary carries a Unicode mapping
// at the dictionary level. It is true when /ToUnicode is present, or when the
// font is a simple font with a standard /Encoding. It does not decode glyphs.
// font is a resolved font dictionary.
func TaggedFontOK(font Value) bool {
	if entry, ok := font.ValueEntry(keyToUnicode); ok && entry.Kind != KindNull {
		return true
	}
	if !simpleFont(font) {
		return false
	}
	return standardFontEncoding(font)
}

// simpleFont reports whether one font is a simple font: Type1, MMType1,
// TrueType, or Type3.
func simpleFont(font Value) bool {
	subtype, ok := font.NameEntry(keySubtype)
	if !ok {
		return false
	}
	switch subtype {
	case "Type1", "MMType1", "TrueType", "Type3":
		return true
	default:
		return false
	}
}

// standardFontEncoding reports whether /Encoding is a standard simple-font
// encoding, directly or through /BaseEncoding.
func standardFontEncoding(font Value) bool {
	entry, ok := font.ValueEntry(keyEncoding)
	if !ok || entry.Kind == KindNull {
		return false
	}
	if entry.Kind == KindName {
		return standardEncodingName(entry.Name)
	}
	if entry.Kind == KindDict {
		base, ok := entry.NameEntry(keyBaseEncoding)
		return ok && standardEncodingName(base)
	}
	return false
}

// TaggedTextOK reports whether one text run can map to Unicode without glyph
// decoding. An /ActualText on the element or an ancestor covers the run, or
// every font in the run passes TaggedFontOK.
func TaggedTextOK(elem *StructElem, fonts []Value) bool {
	if elem != nil {
		if _, ok := elem.TextCover(); ok {
			return true
		}
	}
	for _, font := range fonts {
		if !TaggedFontOK(font) {
			return false
		}
	}
	return true
}

// mcidOf reads an /MCID from a marked-content reference dictionary or an
// integer MCID kid.
func mcidOf(val Value) (int, bool) {
	if val.Kind == KindDict {
		return val.IntEntry(keyMCID)
	}
	return intOf(val)
}

// markedContentRef reports whether one /K dictionary is a marked-content
// reference rather than a structure element.
func markedContentRef(node Value) bool {
	if typeName, ok := node.NameEntry(keyType); ok && typeName == keyMCR {
		return true
	}
	_, ok := node.ValueEntry(keyMCID)
	return ok
}

// kidItems returns the items of one /K value. A single object becomes a
// one-item slice.
func kidItems(kids Value) []Value {
	if kids.Kind == KindArray {
		return kids.Array
	}
	return []Value{kids}
}

// catalogEntry resolves one catalog dictionary entry. The bool is false when
// the trailer, the catalog, or the entry is missing.
func (file *File) catalogEntry(key string) (Value, bool) {
	if file == nil {
		return NullVal(), false
	}
	catalog, err := file.catalogValue()
	if err != nil {
		return NullVal(), false
	}
	entry, ok := catalog.ValueEntry(key)
	if !ok || entry.Kind == KindNull {
		return NullVal(), false
	}
	node, err := file.deref(entry)
	if err != nil {
		return NullVal(), false
	}
	return node, true
}

// catalogValue returns the resolved catalog dictionary.
func (file *File) catalogValue() (Value, error) {
	root, ok := file.trailer.ValueEntry(keyRoot)
	if !ok || root.Kind == KindNull {
		return NullVal(), NewError(opPDF, errSyntax)
	}
	node, err := file.deref(root)
	if err != nil {
		return NullVal(), err
	}
	if node.Kind != KindDict {
		return NullVal(), NewError(opPDF, errSyntax)
	}
	return node, nil
}

// pageTable walks the page tree and returns the page object numbers in page
// order plus a /StructParents value to page index map.
func (file *File) pageTable() ([]int, map[int]int, error) {
	catalog, err := file.catalogValue()
	if err != nil {
		return nil, nil, err
	}
	pages, ok := catalog.ValueEntry(keyPages)
	if !ok || pages.Kind == KindNull {
		return nil, nil, NewError(opPDF, errSyntax)
	}
	nums := []int{}
	if err := file.collectPageNums(pages, map[int]bool{}, &nums); err != nil {
		return nil, nil, err
	}
	marks := map[int]int{}
	for index, num := range nums {
		val, ok, err := file.ObjectValue(num)
		if err != nil {
			return nil, nil, err
		}
		if !ok {
			continue
		}
		if parents, ok := val.IntEntry(keyStructParents); ok {
			marks[parents] = index
		}
	}
	return nums, marks, nil
}

// collectPageNums appends the page object numbers under one page tree node.
func (file *File) collectPageNums(val Value, seen map[int]bool, nums *[]int) error {
	if val.Kind == KindRef {
		if seen[val.RefNum] {
			return NewError(opPDF, errSyntax)
		}
		seen[val.RefNum] = true
	}
	node, err := file.deref(val)
	if err != nil {
		return err
	}
	return file.collectPageNode(val, node, seen, nums)
}

// collectPageNode appends one resolved page tree node.
func (file *File) collectPageNode(
	val Value,
	node Value,
	seen map[int]bool,
	nums *[]int,
) error {
	if node.Kind != KindDict {
		return NewError(opPDF, errSyntax)
	}
	typeName, _ := node.NameEntry(keyType)
	if typeName == keyPage {
		if val.Kind != KindRef {
			return NewError(opPDF, errSyntax)
		}
		*nums = append(*nums, val.RefNum)
		return nil
	}
	kids, hasKids := node.ArrayEntry(keyKids)
	if !hasKids {
		return NewError(opPDF, errSyntax)
	}
	for _, kid := range kids {
		if err := file.collectPageNums(kid, seen, nums); err != nil {
			return err
		}
	}
	return nil
}

// stringEntry returns a text string dictionary entry, or an empty string.
func stringEntry(val Value, key string) string {
	entry, ok := val.ValueEntry(key)
	if !ok || entry.Kind != KindString {
		return ""
	}
	return entry.String
}

// boolEntry returns a true boolean dictionary entry.
func boolEntry(val Value, key string) bool {
	entry, ok := val.ValueEntry(key)
	return ok && entry.Kind == KindBool && entry.Bool
}
