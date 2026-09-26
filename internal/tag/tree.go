package tag

import (
	"bytes"
	"context"
	"sort"

	"github.com/chinmay-sawant/spectrePS/internal/pdf"
	"github.com/chinmay-sawant/spectrePS/internal/pdfa"
	"github.com/chinmay-sawant/spectrePS/internal/pdfout"
)

// The PDF names and the depth cap the structure parser uses. The generated
// tree stays inside the cap so it reads back.
const (
	structDepthCap = 64
	mcrType        = "MCR"

	// treeRootObjects is the root plus the PDF 2.0 namespace dictionary. The
	// default namespace dictionary is one more when a role map exists.
	treeRootObjects = 2
	// parentTreePairLen is the key and value pair count per page.
	parentTreePairLen = 2
	// pageExtraEntries is /StructParents plus /Contents.
	pageExtraEntries = 2

	keyStructTreeRoot    = "StructTreeRoot"
	keyStructElem        = "StructElem"
	keyStructParents     = "StructParents"
	keyParentTree        = "ParentTree"
	keyParentTreeNextKey = "ParentTreeNextKey"
	keyNamespaces        = "Namespaces"
	keyRoleMapNS         = "RoleMapNS"
	keyNamespaceName     = "NS"
	keyNamespaceType     = "Namespace"
	keyKids              = "K"
	keyParent            = "P"
	keyPageRef           = "Pg"
	keyType              = "Type"
	keyMCID              = "MCID"
	keyS                 = "S"
	keyAlt               = "Alt"
	keyActualText        = "ActualText"
	keyLang              = "Lang"
	keyContents          = "Contents"
	keyNums              = "Nums"
	keyAttr              = "A"
	keyScope             = "Scope"
	keyListNumbering     = "ListNumbering"

	// artifactType is the marked-content tag that wraps decorations.
	artifactType = "Artifact"

	// namespacePDF20 is the PDF 2.0 standard structure namespace. Every
	// generated element carries /NS pointing at its dictionary, open
	// decision 8.
	namespacePDF20 = "http://iso.org/pdf2/ssn"
	// namespaceDefault is the PDF 1.7 standard structure namespace. A role
	// map target lands there, because RoleMapNS maps into another namespace.
	namespaceDefault = "http://iso.org/pdf/ssn"
)

// Plan is one authored structure tree. Root must be a Document, the top
// element PDF/UA-2 requires. Roles maps a structure type that is not standard
// in the PDF 2.0 namespace to a standard type in the PDF 1.7 namespace, for
// example TOC to Div. Artifacts lists content that stays outside the tree and
// is wrapped in /Artifact BMC.
type Plan struct {
	Root      *Element
	Roles     map[string]string
	Artifacts []Claim
}

// Element is one authored structure element. Claims are the marked-content
// sequences this element owns, Kids are its child elements, Scope is the
// /Scope value of a table header cell, and ListNumbering is the /A
// /ListNumbering value of a list.
type Element struct {
	Type          string
	Alt           string
	ActualText    string
	Lang          string
	Scope         string
	ListNumbering string
	Kids          []*Element
	Claims        []Claim
}

// Claim is one generated marked-content sequence: the half-open range
// [First, Last) of recorded events on one page. The recorder's event order
// defines the range, so First is the first event the sequence wraps.
type Claim struct {
	Page  int
	First int
	Last  int
}

// builtElem is one structure element with its allocated object number.
type builtElem struct {
	num           int
	parent        int
	typeName      string
	alt           string
	actual        string
	lang          string
	scope         string
	listNumbering string
	page          int
	kids          []*builtElem
	claims        []builtClaim
}

// builtClaim is one claim with its assigned MCID.
type builtClaim struct {
	page  int
	first int
	last  int
	mcid  int
}

// object is one appended object body with its allocated number.
type object struct {
	num  int
	body []byte
}

// builder allocates the appended objects and assigns MCIDs for one write.
type builder struct {
	file         *pdf.File
	plan         *Plan
	recs         []*Recorder
	pageNums     []int
	targets      []int
	appended     []bool
	spans        [][]span
	parents      [][]*builtElem
	root         *builtElem
	elems        []*builtElem
	roles        map[string]string
	parentBase   int
	rootNum      int
	pdf2NSNum    int
	defaultNSNum int
	metaNum      int
	next         int
}

// newBuilder validates the plan, allocates every appended object number in a
// fixed order, and assigns one MCID per generated sequence. The fixed order
// is the structure tree root, the namespace dictionaries, every structure
// element in preorder, the generated content streams that need a fresh
// object, and the metadata stream.
func newBuilder(file *pdf.File, recs []*Recorder, plan *Plan) (*builder, error) {
	built, err := newBuilderState(file, recs, plan)
	if err != nil {
		return nil, err
	}
	if err := built.allocate(); err != nil {
		return nil, err
	}
	if err := built.assignMCIDs(); err != nil {
		return nil, err
	}
	if err := built.allocateContent(); err != nil {
		return nil, err
	}
	built.metaNum = built.next
	built.next++
	return built, nil
}

// newBuilderState validates the plan and returns a builder with no number
// allocated yet.
func newBuilderState(file *pdf.File, recs []*Recorder, plan *Plan) (*builder, error) {
	if file == nil || plan == nil || plan.Root == nil {
		return nil, pdf.NewError(opTag, errPlan)
	}
	if len(recs) != file.PageCount() {
		return nil, pdf.NewError(opTag, errPlan)
	}
	pageNums := file.PageObjectNums()
	if len(pageNums) != file.PageCount() {
		return nil, pdf.NewError(opTag, errPlan)
	}
	if err := rolesOK(plan.Roles); err != nil {
		return nil, err
	}
	return &builder{
		file:         file,
		plan:         plan,
		recs:         recs,
		pageNums:     pageNums,
		targets:      nil,
		appended:     nil,
		spans:        nil,
		parents:      nil,
		root:         nil,
		elems:        nil,
		roles:        plan.Roles,
		parentBase:   0,
		rootNum:      0,
		pdf2NSNum:    0,
		defaultNSNum: 0,
		metaNum:      0,
		next:         0,
	}, nil
}

// allocate reserves the root, the namespace dictionaries, and every element
// number in preorder, then walks the plan into typed elements.
func (built *builder) allocate() error {
	built.parentBase = freshParentBase(built.pageNums)
	built.next = built.file.ObjectCount() + 1
	built.rootNum = built.next
	built.next++
	built.pdf2NSNum = built.next
	built.next++
	if len(built.roles) > 0 {
		built.defaultNSNum = built.next
		built.next++
	}
	root, err := built.collect(built.plan.Root, 0, built.rootNum)
	if err != nil {
		return err
	}
	if root.typeName != documentType {
		return pdf.NewError(opTag, errPlan)
	}
	built.root = root
	return nil
}

// rolesOK rejects a role map entry with an empty name or target.
func rolesOK(roles map[string]string) error {
	for name, target := range roles {
		if name == "" || target == "" {
			return pdf.NewError(opTag, errRoleMap)
		}
	}
	return nil
}

// freshParentBase returns a /StructParents base greater than every page object
// number, so a parent tree key cannot collide with a page number. The parser
// checks page object numbers before /StructParents values.
func freshParentBase(pageNums []int) int {
	highest := 0
	for _, num := range pageNums {
		if num > highest {
			highest = num
		}
	}
	return highest + 1
}

// collect walks the plan and allocates one object number per element in
// preorder. parent is the object number of the parent element, or the
// structure tree root for the top element.
func (built *builder) collect(elem *Element, depth, parent int) (*builtElem, error) {
	if depth > structDepthCap || elem == nil || elem.Type == "" {
		return nil, pdf.NewError(opTag, errPlan)
	}
	if !pdf.StandardTypePDF20(elem.Type) && built.roles[elem.Type] == "" {
		return nil, pdf.NewError(opTag, errRoleMap)
	}
	node := &builtElem{
		num:           built.next,
		parent:        parent,
		typeName:      elem.Type,
		alt:           elem.Alt,
		actual:        elem.ActualText,
		lang:          elem.Lang,
		scope:         elem.Scope,
		listNumbering: elem.ListNumbering,
		page:          -1,
		kids:          nil,
		claims:        nil,
	}
	built.next++
	built.elems = append(built.elems, node)
	for _, claim := range elem.Claims {
		node.claims = append(node.claims, builtClaim{
			page: claim.Page, first: claim.First, last: claim.Last, mcid: -1,
		})
	}
	for _, kid := range elem.Kids {
		child, err := built.collect(kid, depth+1, node.num)
		if err != nil {
			return nil, err
		}
		node.kids = append(node.kids, child)
	}
	return node, nil
}

// claimRef pairs one claim with its owning element.
type claimRef struct {
	elem  *builtElem
	claim *builtClaim
}

// assignMCIDs validates every claim and numbers it in paint order. MCIDs are
// unique on their page and start at zero, so the parent tree array index is
// the paint order position. Artifact spans share the paint order but write no
// MCID and no parent tree entry.
func (built *builder) assignMCIDs() error {
	byPage, artifacts, err := built.claimRefs()
	if err != nil {
		return err
	}
	built.spans = make([][]span, len(built.recs))
	built.parents = make([][]*builtElem, len(built.recs))
	for page := range built.recs {
		marks, err := built.pageSpans(page, byPage[page], artifacts[page])
		if err != nil {
			return err
		}
		built.spans[page] = marks
	}
	return nil
}

// claimRefs validates every structure claim and every artifact claim and
// groups them by page.
func (built *builder) claimRefs() ([][]claimRef, [][]Claim, error) {
	byPage := make([][]claimRef, len(built.recs))
	for _, elem := range built.elems {
		for index := range elem.claims {
			claim := &elem.claims[index]
			if err := built.validRange(claim.page, claim.first, claim.last); err != nil {
				return nil, nil, err
			}
			if elem.page < 0 {
				elem.page = claim.page
			}
			byPage[claim.page] = append(byPage[claim.page], claimRef{elem: elem, claim: claim})
		}
	}
	artifacts := make([][]Claim, len(built.recs))
	for _, claim := range built.plan.Artifacts {
		if err := built.validRange(claim.Page, claim.First, claim.Last); err != nil {
			return nil, nil, err
		}
		artifacts[claim.Page] = append(artifacts[claim.Page], Claim{
			Page: claim.Page, First: claim.First, Last: claim.Last,
		})
	}
	return byPage, artifacts, nil
}

// pageSpans merges one page's structure and artifact claims in paint order
// and numbers the structure claims.
func (built *builder) pageSpans(page int, refs []claimRef, artifacts []Claim) ([]span, error) {
	marks := make([]span, 0, len(refs)+len(artifacts))
	for _, ref := range refs {
		marks = append(marks, span{
			first: ref.claim.first,
			last:  ref.claim.last,
			mcid:  -1,
			tag:   ref.elem.typeName,
			elem:  ref.elem,
			claim: ref.claim,
		})
	}
	for _, claim := range artifacts {
		marks = append(marks, span{
			first: claim.First,
			last:  claim.Last,
			mcid:  -1,
			tag:   artifactType,
			elem:  nil,
			claim: nil,
		})
	}
	sort.SliceStable(marks, func(i, j int) bool {
		return marks[i].first < marks[j].first
	})
	previous := 0
	for index := range marks {
		if marks[index].first < previous {
			return nil, pdf.NewError(opTag, errMCID)
		}
		previous = marks[index].last
		if marks[index].elem == nil {
			continue
		}
		marks[index].mcid = len(built.parents[page])
		marks[index].claim.mcid = marks[index].mcid
		built.parents[page] = append(built.parents[page], marks[index].elem)
	}
	return marks, nil
}

// validRange checks one claim against the recorded events of its page. A
// claim with no emittable event would write an empty marked-content sequence,
// so it is refused.
func (built *builder) validRange(page, first, last int) error {
	if page < 0 || page >= len(built.recs) {
		return pdf.NewError(opTag, errMCID)
	}
	events := built.recs[page].events
	if first < 0 || last > len(events) || first >= last {
		return pdf.NewError(opTag, errMCID)
	}
	for _, evt := range events[first:last] {
		if !evt.marked() {
			return nil
		}
	}
	return pdf.NewError(opTag, errMCID)
}

// allocateContent picks the object that holds each page's generated content.
// A page whose first content object is used by no other page is overridden,
// so the source body does not stay as dead weight; any further content
// objects of that page stay in the file unused. A shared or missing content
// object gets a fresh appended stream.
func (built *builder) allocateContent() error {
	counts := map[int]int{}
	refs := make([][]int, len(built.recs))
	for page := range built.recs {
		got, err := built.file.PageContentNums(page)
		if err != nil {
			return err
		}
		refs[page] = got
		for _, num := range got {
			counts[num]++
		}
	}
	built.targets = make([]int, len(built.recs))
	built.appended = make([]bool, len(built.recs))
	for page := range built.recs {
		first := 0
		if len(refs[page]) > 0 {
			first = refs[page][0]
		}
		if first > 0 && counts[first] == 1 {
			built.targets[page] = first
			continue
		}
		built.targets[page] = built.next
		built.next++
		built.appended[page] = true
	}
	return nil
}

// treeObjects returns the structure tree root, the namespace dictionaries,
// and every element body, in allocated number order.
func (built *builder) treeObjects() []object {
	objs := make([]object, 0, len(built.elems)+treeRootObjects)
	objs = append(objs, object{num: built.rootNum, body: built.rootBody()})
	objs = append(objs, object{num: built.pdf2NSNum, body: built.pdf2NSBody()})
	if built.defaultNSNum != 0 {
		objs = append(objs, object{num: built.defaultNSNum, body: defaultNSBody()})
	}
	for _, elem := range built.elems {
		objs = append(objs, object{num: elem.num, body: built.elemBody(elem)})
	}
	return objs
}

// rootBody writes /StructTreeRoot with its parent tree, namespaces, and kids.
func (built *builder) rootBody() []byte {
	entries := map[string]pdf.Value{
		keyType:              pdf.NameVal(keyStructTreeRoot),
		keyKids:              pdf.ArrayVal([]pdf.Value{pdf.RefVal(built.root.num, 0)}),
		keyParentTree:        built.parentTreeValue(),
		keyParentTreeNextKey: pdf.IntVal(int64(built.parentBase + len(built.recs))),
		keyNamespaces:        built.namespacesValue(),
	}
	return pdf.SerializeValue(pdf.DictVal(entries))
}

// pdf2NSBody writes the PDF 2.0 namespace dictionary. Every generated element
// points at it, and a role map lives here as /RoleMapNS because the element
// namespace is explicit.
func (built *builder) pdf2NSBody() []byte {
	entries := map[string]pdf.Value{
		keyType:          pdf.NameVal(keyNamespaceType),
		keyNamespaceName: pdf.StringVal(namespacePDF20),
	}
	if len(built.roles) > 0 {
		entries[keyRoleMapNS] = built.roleMapValue()
	}
	return pdf.SerializeValue(pdf.DictVal(entries))
}

// defaultNSBody writes the PDF 1.7 namespace dictionary a role map target
// lands in.
func defaultNSBody() []byte {
	return pdf.SerializeValue(pdf.DictVal(map[string]pdf.Value{
		keyType:          pdf.NameVal(keyNamespaceType),
		keyNamespaceName: pdf.StringVal(namespaceDefault),
	}))
}

// roleMapValue writes one /RoleMapNS entry per role: the target type and the
// default namespace dictionary it lives in.
func (built *builder) roleMapValue() pdf.Value {
	entries := make(map[string]pdf.Value, len(built.roles))
	for name, target := range built.roles {
		entries[name] = pdf.ArrayVal([]pdf.Value{
			pdf.NameVal(target),
			pdf.RefVal(built.defaultNSNum, 0),
		})
	}
	return pdf.DictVal(entries)
}

// namespacesValue lists the namespace dictionaries the tree uses.
func (built *builder) namespacesValue() pdf.Value {
	items := []pdf.Value{pdf.RefVal(built.pdf2NSNum, 0)}
	if built.defaultNSNum != 0 {
		items = append(items, pdf.RefVal(built.defaultNSNum, 0))
	}
	return pdf.ArrayVal(items)
}

// parentTreeValue writes one flat /Nums array. Each page gets a fresh key and
// one array slot per structure MCID, in paint order. Artifact spans carry no
// MCID, so they stay out of the parent tree.
func (built *builder) parentTreeValue() pdf.Value {
	nums := make([]pdf.Value, 0, len(built.recs)*parentTreePairLen)
	for page := range built.recs {
		refs := make([]pdf.Value, 0, len(built.parents[page]))
		for _, elem := range built.parents[page] {
			refs = append(refs, pdf.RefVal(elem.num, 0))
		}
		nums = append(nums, pdf.IntVal(int64(built.parentBase+page)), pdf.ArrayVal(refs))
	}
	return pdf.DictVal(map[string]pdf.Value{keyNums: pdf.ArrayVal(nums)})
}

// elemBody writes one structure element. A claim that sits on one page is a
// bare MCID integer; content that spans pages is a marked-content reference
// dictionary, because a bare MCID uses only the element /Pg.
func (built *builder) elemBody(elem *builtElem) []byte {
	entries := map[string]pdf.Value{
		keyType:          pdf.NameVal(keyStructElem),
		keyS:             pdf.NameVal(elem.typeName),
		keyParent:        pdf.RefVal(elem.parent, 0),
		keyNamespaceName: pdf.RefVal(built.pdf2NSNum, 0),
	}
	if elem.page >= 0 {
		entries[keyPageRef] = pdf.RefVal(built.pageNums[elem.page], 0)
	}
	if elem.alt != "" {
		entries[keyAlt] = pdf.StringVal(elem.alt)
	}
	if elem.actual != "" {
		entries[keyActualText] = pdf.StringVal(elem.actual)
	}
	if elem.lang != "" {
		entries[keyLang] = pdf.StringVal(elem.lang)
	}
	if attrs := attributeDict(elem); attrs != nil {
		entries[keyAttr] = pdf.DictVal(attrs)
	}
	if kids := built.kidsValue(elem); len(kids) > 0 {
		entries[keyKids] = pdf.ArrayVal(kids)
	}
	return pdf.SerializeValue(pdf.DictVal(entries))
}

// attributeDict writes the /A attribute object of one element: a table scope
// or a list numbering. It is nil when the element carries neither.
func attributeDict(elem *builtElem) map[string]pdf.Value {
	switch {
	case elem.scope != "":
		return map[string]pdf.Value{
			"O":      pdf.NameVal("Table"),
			keyScope: pdf.NameVal(elem.scope),
		}
	case elem.listNumbering != "":
		return map[string]pdf.Value{
			"O":              pdf.NameVal("List"),
			keyListNumbering: pdf.NameVal(elem.listNumbering),
		}
	default:
		return nil
	}
}

// kidsValue writes the /K array: claims first, then child element references.
func (built *builder) kidsValue(elem *builtElem) []pdf.Value {
	items := make([]pdf.Value, 0, len(elem.claims)+len(elem.kids))
	if samePage(elem.claims) {
		for _, claim := range elem.claims {
			items = append(items, pdf.IntVal(int64(claim.mcid)))
		}
	} else {
		for _, claim := range elem.claims {
			items = append(items, pdf.DictVal(map[string]pdf.Value{
				keyType:    pdf.NameVal(mcrType),
				keyPageRef: pdf.RefVal(built.pageNums[claim.page], 0),
				keyMCID:    pdf.IntVal(int64(claim.mcid)),
			}))
		}
	}
	for _, kid := range elem.kids {
		items = append(items, pdf.RefVal(kid.num, 0))
	}
	return items
}

// samePage reports whether every claim of one element sits on the same page.
// An element with no claim is trivially on one page.
func samePage(claims []builtClaim) bool {
	for index := 1; index < len(claims); index++ {
		if claims[index].page != claims[0].page {
			return false
		}
	}
	return true
}

// contentBody writes one page's generated stream object with its MCID
// sequences and no source marked content. The result is a complete object
// body: a stream dictionary plus the operators.
func (built *builder) contentBody(page int) []byte {
	var buf bytes.Buffer
	emitEvents(&buf, built.recs[page].events, built.spans[page], true)
	return pdf.SerializeValue(pdf.StreamVal(map[string]pdf.Value{}, buf.Bytes()))
}

// pageBody writes the page dictionary override: the fresh /StructParents
// value and the generated /Contents.
func (built *builder) pageBody(page int) ([]byte, error) {
	val, ok, err := built.file.ObjectValue(built.pageNums[page])
	if err != nil {
		return nil, err
	}
	if !ok || val.Kind != pdf.KindDict {
		return nil, pdf.NewError(opTag, errPlan)
	}
	entries := make(map[string]pdf.Value, len(val.Dict)+pageExtraEntries)
	for key, entry := range val.Dict {
		entries[key] = entry
	}
	entries[keyStructParents] = pdf.IntVal(int64(built.parentBase + page))
	entries[keyContents] = pdf.RefVal(built.targets[page], 0)
	return pdf.SerializeValue(pdf.DictVal(entries)), nil
}

// catalogBody writes the catalog override. It adds /StructTreeRoot and then
// lets pdfa.UA2Catalog set /Metadata, /MarkInfo, /Lang, and /ViewerPreferences.
func (built *builder) catalogBody(meta pdfa.UA2Metadata) ([]byte, error) {
	root := built.file.RootNum()
	if root <= 0 {
		return nil, pdf.NewError(opTag, errPlan)
	}
	catalog, ok, err := built.file.ObjectValue(root)
	if err != nil {
		return nil, err
	}
	if !ok || catalog.Kind != pdf.KindDict {
		return nil, pdf.NewError(opTag, errPlan)
	}
	entries := make(map[string]pdf.Value, len(catalog.Dict)+1)
	for key, entry := range catalog.Dict {
		entries[key] = entry
	}
	entries[keyStructTreeRoot] = pdf.RefVal(built.rootNum, 0)
	return pdfa.UA2Catalog(pdf.DictVal(entries), built.metaNum, meta), nil
}

// write builds one complete file. meta carries the XMP title and language,
// with or without the pdfuaid claim.
func (built *builder) write(ctx context.Context, meta pdfa.UA2Metadata) ([]byte, error) {
	catalog, err := built.catalogBody(meta)
	if err != nil {
		return nil, err
	}
	overrides := map[int][]byte{}
	trees := built.treeObjects()
	appended := make([][]byte, 0, len(trees)+len(built.recs)+1)
	for _, obj := range trees {
		appended = append(appended, obj.body)
	}
	for page := range built.recs {
		body := built.contentBody(page)
		if built.appended[page] {
			appended = append(appended, body)
		} else {
			overrides[built.targets[page]] = body
		}
		pageBody, err := built.pageBody(page)
		if err != nil {
			return nil, err
		}
		overrides[built.pageNums[page]] = pageBody
	}
	appended = append(appended, pdfa.UA2ExtraObjects(meta, built.metaNum)...)
	return pdfout.WriteCopy(ctx, built.file, pdfout.CopyOptions{
		Overrides:       overrides,
		PackObjects:     false,
		AppendObjects:   appended,
		CatalogOverride: catalog,
		PDFA:            false,
		Tag:             true,
	})
}
