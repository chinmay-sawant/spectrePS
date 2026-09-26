package tag

import (
	"bytes"
	"errors"
	"strconv"
	"testing"

	"github.com/chinmay-sawant/spectrePS/internal/pdf"
	"github.com/chinmay-sawant/spectrePS/internal/pdfa"
)

// TestTagMCIDCoverage proves every generated sequence gets a unique MCID in
// paint order and the parent tree resolves both ways.
func TestTagMCIDCoverage(t *testing.T) {
	file, recs := openTagFixture(t, 2)
	plan := documentPlan([][]Claim{
		singleClaim(0, 0, 1),
		singleClaim(0, 1, 2),
		singleClaim(1, 0, 1),
	})
	reopened := reopen(t, buildTagged(t, file, recs, plan, Options{}))
	tree := mustTree(t, reopened)
	if got := len(tree.Keys()); got != 3 {
		t.Fatalf("claims = %d, want 3", got)
	}
	want := []pdf.StructKey{{Page: 0, MCID: 0}, {Page: 0, MCID: 1}, {Page: 1, MCID: 0}}
	for index, key := range want {
		elem, err := tree.Lookup(key)
		if err != nil {
			t.Fatal(err)
		}
		if elem.Type != "P" {
			t.Fatalf("key %+v type = %q", key, elem.Type)
		}
		back, ok := tree.Key(elem)
		if !ok || back != key {
			t.Fatalf("Key() = %+v %v, want %+v", back, ok, key)
		}
		if elem != tree.Top[0].Kids[index] {
			t.Fatalf("key %+v element mismatch", key)
		}
	}
	page0 := mustContent(t, reopened, 0)
	page1 := mustContent(t, reopened, 1)
	wantOrder(t, page0, []string{"/MCID 0", "BDC\n", "EMC\n", "/MCID 1", "BDC\n", "EMC\n"})
	wantOrder(t, page1, []string{"/MCID 0", "BDC\n", "EMC\n"})
}

// TestTagParentTree proves the parent tree agrees with every /K claim and no
// MCID is orphaned on either side.
func TestTagParentTree(t *testing.T) {
	file, recs := openTagFixture(t, 2)
	plan := documentPlan([][]Claim{singleClaim(0, 0, 1), singleClaim(1, 0, 1)})
	reopened := reopen(t, buildTagged(t, file, recs, plan, Options{}))
	tree := mustTree(t, reopened)
	checkParentTreeBothWays(t, tree)
	checkStructParents(t, reopened)
}

// checkParentTreeBothWays proves Lookup and Key agree for every claim.
func checkParentTreeBothWays(t *testing.T, tree *pdf.StructTree) {
	t.Helper()
	keys := tree.Keys()
	if len(keys) != 2 {
		t.Fatalf("keys = %+v", keys)
	}
	for _, key := range keys {
		elem, err := tree.Lookup(key)
		if err != nil {
			t.Fatal(err)
		}
		back, ok := tree.Key(elem)
		if !ok || back != key {
			t.Fatalf("Key() = %+v %v, want %+v", back, ok, key)
		}
	}
	for _, elem := range tree.Top[0].Kids {
		if _, ok := tree.Key(elem); !ok {
			t.Fatalf("element %q has no key", elem.Type)
		}
	}
}

// checkStructParents proves every page carries a fresh parent tree key.
func checkStructParents(t *testing.T, file *pdf.File) {
	t.Helper()
	for _, num := range file.PageObjectNums() {
		val, ok, err := file.ObjectValue(num)
		if err != nil || !ok {
			t.Fatalf("page %d = %v %v", num, ok, err)
		}
		if _, ok := val.IntEntry("StructParents"); !ok {
			t.Fatalf("page %d has no /StructParents", num)
		}
	}
}

// TestTagContentMCID proves the content side carries the MCIDs the parent
// tree claims, one paired sequence per claim.
func TestTagContentMCID(t *testing.T) {
	file, recs := openTagFixture(t, 2)
	plan := documentPlan([][]Claim{singleClaim(0, 0, 2), singleClaim(1, 1, 3)})
	reopened := reopen(t, buildTagged(t, file, recs, plan, Options{}))
	tree := mustTree(t, reopened)
	for _, key := range tree.Keys() {
		content := mustContent(t, reopened, key.Page)
		if !bytes.Contains(content, []byte("/MCID "+strconv.Itoa(key.MCID))) {
			t.Fatalf("page %d lacks MCID %d in %q", key.Page, key.MCID, content)
		}
	}
	for page := range 2 {
		content := mustContent(t, reopened, page)
		if got := bytes.Count(content, []byte(" BDC\n")); got != 1 {
			t.Fatalf("page %d BDC count = %d", page, got)
		}
		if got := bytes.Count(content, []byte("EMC\n")); got != 1 {
			t.Fatalf("page %d EMC count = %d", page, got)
		}
		if bytes.Contains(content, []byte("BMC\n")) {
			t.Fatalf("page %d carries a BMC", page)
		}
	}
}

// TestTagMCR proves content that spans pages serializes as a marked-content
// reference, and single-page content stays a bare MCID.
func TestTagMCR(t *testing.T) {
	file, recs := openTagFixture(t, 2)
	span := []Claim{{Page: 0, First: 0, Last: 1}, {Page: 1, First: 0, Last: 1}}
	plan := documentPlan([][]Claim{span, singleClaim(0, 1, 2)})
	out := buildTagged(t, file, recs, plan, Options{})
	if !bytes.Contains(out, []byte("/Type /MCR")) {
		t.Fatal("output carries no /Type /MCR")
	}
	reopened := reopen(t, out)
	tree := mustTree(t, reopened)
	spanElem := tree.Top[0].Kids[0]
	if len(spanElem.Items) != 2 {
		t.Fatalf("span items = %+v", spanElem.Items)
	}
	if spanElem.Items[0].Page != 0 || spanElem.Items[1].Page != 1 {
		t.Fatalf("span pages = %+v", spanElem.Items)
	}
	oneElem := tree.Top[0].Kids[1]
	if len(oneElem.Items) != 1 {
		t.Fatalf("single items = %+v", oneElem.Items)
	}
	built, err := newBuilder(file, recs, plan)
	if err != nil {
		t.Fatal(err)
	}
	objs := built.treeObjects()
	if !bytes.Contains(objs[3].body, []byte("/Type /MCR")) {
		t.Fatalf("span body = %q", objs[3].body)
	}
	if bytes.Contains(objs[4].body, []byte("/Type /MCR")) {
		t.Fatalf("single body = %q", objs[4].body)
	}
}

// TestBuildTreeObjects proves the typed builder emits the root, the namespace
// dictionary, and each element with numbers from ObjectCount()+1 in a fixed
// order, and that two runs serialize equal bytes.
func TestBuildTreeObjects(t *testing.T) {
	file, recs := openTagFixture(t, 1)
	plan := documentPlan([][]Claim{singleClaim(0, 0, 1)})
	built, err := newBuilder(file, recs, plan)
	if err != nil {
		t.Fatal(err)
	}
	objs := built.treeObjects()
	if len(objs) != 4 {
		t.Fatalf("objects = %d, want 4", len(objs))
	}
	checkObjectNumbers(t, objs, file.ObjectCount()+1)
	checkObjectBodies(t, objs)
	again := built.treeObjects()
	for index := range objs {
		if !bytes.Equal(objs[index].body, again[index].body) {
			t.Fatalf("object %d differs between runs", objs[index].num)
		}
	}
}

// checkObjectNumbers proves the allocated numbers start at first and step by
// one.
func checkObjectNumbers(t *testing.T, objs []object, first int) {
	t.Helper()
	if objs[0].num != first {
		t.Fatalf("first number = %d, want %d", objs[0].num, first)
	}
	for index := 1; index < len(objs); index++ {
		if objs[index].num != objs[index-1].num+1 {
			t.Fatalf("numbers %d then %d", objs[index-1].num, objs[index].num)
		}
	}
}

// checkObjectBodies proves the fixed order: root, namespace, elements.
func checkObjectBodies(t *testing.T, objs []object) {
	t.Helper()
	if !bytes.Contains(objs[0].body, []byte("/StructTreeRoot")) {
		t.Fatalf("root = %q", objs[0].body)
	}
	if !bytes.Contains(objs[1].body, []byte("http://iso.org/pdf2/ssn")) {
		t.Fatalf("namespace = %q", objs[1].body)
	}
	if !bytes.Contains(objs[2].body, []byte("/StructElem")) {
		t.Fatalf("document = %q", objs[2].body)
	}
}

// TestTagStable proves two builds from the same input return equal bytes.
func TestTagStable(t *testing.T) {
	file, recs := openTagFixture(t, 2)
	plan := documentPlan([][]Claim{singleClaim(0, 0, 2), singleClaim(1, 0, 1)})
	opt := Options{Title: "Annual report", Lang: "en-US", Claim: true}
	first := buildTagged(t, file, recs, plan, opt)
	second := buildTagged(t, file, recs, plan, opt)
	if !bytes.Equal(first, second) {
		t.Fatal("two builds differ")
	}
}

// TestTagCatalog proves the catalog override carries the structure tree root,
// /Marked true, /Lang, /DisplayDocTitle, and /Metadata.
func TestTagCatalog(t *testing.T) {
	file, recs := openTagFixture(t, 1)
	plan := documentPlan([][]Claim{singleClaim(0, 0, 1)})
	out := buildTagged(t, file, recs, plan, Options{Title: "Annual report", Lang: "en-US"})
	reopened := reopen(t, out)
	if !reopened.HasStructTree() {
		t.Fatal("HasStructTree() = false")
	}
	checkCatalogInfo(t, reopened)
	tree := mustTree(t, reopened)
	root := tree.Root()
	if root == nil || root.Type != documentType || root.RoleNS != pdf.NamespacePDF20 {
		t.Fatalf("root = %+v", root)
	}
}

// checkCatalogInfo proves /Metadata, /Marked, /DisplayDocTitle, /Lang, and
// dc:title survive the write.
func checkCatalogInfo(t *testing.T, file *pdf.File) {
	t.Helper()
	info, err := pdfa.ReadUA2(file)
	if err != nil {
		t.Fatal(err)
	}
	if !info.Metadata || !info.Marked || !info.DisplayDocTitle {
		t.Fatalf("catalog flags = %+v", info)
	}
	if info.Lang != "en-US" || info.Title != "Annual report" {
		t.Fatalf("catalog text = %+v", info)
	}
}

// TestTagClaim proves the positive claim case: a passing build writes
// pdfuaid:part 2 and pdfuaid:rev 2024 only after the preflight passes.
func TestTagClaim(t *testing.T) {
	file, recs := openTagFixture(t, 1)
	plan := documentPlan([][]Claim{singleClaim(0, 0, 1)})
	claimed := buildTagged(t, file, recs, plan, Options{
		Title: "Annual report", Lang: "en-US", Claim: true,
	})
	reopened := reopen(t, claimed)
	info, err := pdfa.ReadUA2(reopened)
	if err != nil {
		t.Fatal(err)
	}
	if info.Part != "2" || info.Rev != "2024" {
		t.Fatalf("claim = %+v", info)
	}
	if err := pdfa.PreflightUA2(t.Context(), reopened); err != nil {
		t.Fatal(err)
	}
	unclaimed := buildTagged(t, file, recs, plan, Options{Title: "Annual report", Lang: "en-US"})
	if got := mustUA2(t, reopen(t, unclaimed)); got.Part != "" {
		t.Fatalf("unclaimed part = %q", got.Part)
	}
}

// TestTagTitleRequired proves the negative claim case: with no title from the
// caller or the source, the build returns ua2-title and writes no claim.
func TestTagTitleRequired(t *testing.T) {
	file, recs := openTagFixture(t, 1)
	plan := documentPlan([][]Claim{singleClaim(0, 0, 1)})
	out, err := Build(t.Context(), file, recs, plan, Options{Lang: "en-US", Claim: true})
	var job *pdf.Error
	if !errors.As(err, &job) || job.Op != "PDFUA" || job.Name != "ua2-title" {
		t.Fatalf("error = %v", err)
	}
	if out == nil {
		t.Fatal("refusal dropped the tree")
	}
	reopened := reopen(t, out)
	if got := mustUA2(t, reopened); got.Part != "" {
		t.Fatalf("part = %q after a refusal", got.Part)
	}
	if err := pdfa.PreflightUA2(t.Context(), reopened); !errors.As(err, &job) || job.Name != "ua2-title" {
		t.Fatalf("preflight = %v", err)
	}
}

// TestTagRoleNamespace proves /NS sits on every generated element, TOC and
// TOCI keep the ISO 32000-2 spelling and role-map to a standard PDF 2.0 type,
// and the tree round-trips through internal/pdf.
func TestTagRoleNamespace(t *testing.T) {
	file, recs := openTagFixture(t, 1)
	toci := &Element{
		Type: "TOCI", Alt: "", ActualText: "", Lang: "",
		Kids: nil, Claims: singleClaim(0, 1, 2),
	}
	toc := &Element{
		Type: "TOC", Alt: "", ActualText: "", Lang: "",
		Kids: []*Element{toci}, Claims: singleClaim(0, 0, 1),
	}
	plan := &Plan{
		Root: &Element{
			Type: "Document", Alt: "", ActualText: "", Lang: "",
			Kids: []*Element{toc}, Claims: nil,
		},
		Roles: map[string]string{"TOC": "Div", "TOCI": "Div"},
	}
	out := buildTagged(t, file, recs, plan, Options{})
	tree := mustTree(t, reopen(t, out))
	root := tree.Root()
	if root == nil || root.Type != documentType || root.Namespace != pdf.NamespacePDF20 {
		t.Fatalf("root = %+v", root)
	}
	checkNamespaceWalk(t, tree, root)
	if root.Kids[0].Type != "TOC" || root.Kids[0].Role != "Div" {
		t.Fatalf("TOC = %+v", root.Kids[0])
	}
	if root.Kids[0].Kids[0].Type != "TOCI" || root.Kids[0].Kids[0].Role != "Div" {
		t.Fatalf("TOCI = %+v", root.Kids[0].Kids[0])
	}
	if pdf.StandardTypePDF20("TOC") {
		t.Fatal("TOC counts as a PDF 2.0 standard type")
	}
	checkTOCSpelling(t, out)
}

// checkNamespaceWalk proves every generated element carries /NS in the PDF
// 2.0 namespace and resolves to a standard type.
func checkNamespaceWalk(t *testing.T, tree *pdf.StructTree, root *pdf.StructElem) {
	t.Helper()
	for _, elem := range walkElems(root) {
		if elem.Namespace != pdf.NamespacePDF20 {
			t.Fatalf("element %q namespace = %q", elem.Type, elem.Namespace)
		}
		if _, _, err := tree.Roles.Resolve(elem.Type, elem.Namespace); err != nil {
			t.Fatalf("Resolve(%q) = %v", elem.Type, err)
		}
		if !pdf.StandardTypePDF20(elem.Type) && elem.Role == elem.Type {
			t.Fatalf("element %q is unmapped", elem.Type)
		}
	}
}

// checkTOCSpelling proves the output spells TOC and TOCI the ISO 32000-2 way.
func checkTOCSpelling(t *testing.T, out []byte) {
	t.Helper()
	if !bytes.Contains(out, []byte("/TOC")) || !bytes.Contains(out, []byte("/TOCI")) {
		t.Fatal("output lacks the TOC spelling")
	}
	if bytes.Contains(out, []byte("/Toc")) || bytes.Contains(out, []byte("/Toci")) {
		t.Fatal("output carries a mixed-case TOC spelling")
	}
}

// buildTagged builds and fails the test on error.
func buildTagged(t *testing.T, file *pdf.File, recs []*Recorder, plan *Plan, opt Options) []byte {
	t.Helper()
	out, err := Build(t.Context(), file, recs, plan, opt)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// reopen opens built bytes or fails the test.
func reopen(t *testing.T, src []byte) *pdf.File {
	t.Helper()
	file, err := pdf.Open(t.Context(), src)
	if err != nil {
		t.Fatal(err)
	}
	return file
}

// mustTree parses the structure tree or fails the test.
func mustTree(t *testing.T, file *pdf.File) *pdf.StructTree {
	t.Helper()
	tree, err := file.StructTree()
	if err != nil {
		t.Fatal(err)
	}
	if tree == nil {
		t.Fatal("StructTree() = nil")
	}
	return tree
}

// mustContent returns one page's decoded content or fails the test.
func mustContent(t *testing.T, file *pdf.File, page int) []byte {
	t.Helper()
	content, err := file.Content(page)
	if err != nil {
		t.Fatal(err)
	}
	return content
}

// mustUA2 reads the metadata or fails the test.
func mustUA2(t *testing.T, file *pdf.File) pdfa.UA2Info {
	t.Helper()
	info, err := pdfa.ReadUA2(file)
	if err != nil {
		t.Fatal(err)
	}
	return info
}

// walkElems returns one element and its descendants in preorder.
func walkElems(root *pdf.StructElem) []*pdf.StructElem {
	if root == nil {
		return nil
	}
	out := []*pdf.StructElem{root}
	for _, kid := range root.Kids {
		out = append(out, walkElems(kid)...)
	}
	return out
}
