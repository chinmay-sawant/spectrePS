package pdf

import (
	"bytes"
	"fmt"
	"testing"
)

// taggedHeader is a PDF 2.0 header with the binary marker on the next line.
const taggedHeader = "%PDF-2.0\n%\xe2\xe3\xcf\xd3\n"

// figureType is the standard type the fixture's namespaced role maps to.
const figureType = "Figure"

// taggedDoc builds a classic-xref PDF with a PDF 2.0 header.
type taggedDoc struct {
	buf     bytes.Buffer
	offsets []int
}

func newTaggedDoc() *taggedDoc {
	doc := &taggedDoc{offsets: []int{0}}
	doc.buf.WriteString(taggedHeader)
	return doc
}

func (doc *taggedDoc) object(body string) {
	num := len(doc.offsets)
	doc.offsets = append(doc.offsets, doc.buf.Len())
	fmt.Fprintf(&doc.buf, "%d 0 obj\n%s\nendobj\n", num, body)
}

func (doc *taggedDoc) classic() []byte {
	xrefAt := doc.buf.Len()
	size := len(doc.offsets)
	fmt.Fprintf(&doc.buf, "xref\n0 %d\n", size)
	doc.buf.WriteString(xrefLine(0, freeGen, false))
	for num := 1; num < size; num++ {
		doc.buf.WriteString(xrefLine(doc.offsets[num], 0, true))
	}
	fmt.Fprintf(&doc.buf, "trailer\n<< /Size %d /Root 1 0 R >>\n", size)
	fmt.Fprintf(&doc.buf, "startxref\n%d\n%%%%EOF\n", xrefAt)
	return doc.buf.Bytes()
}

func TestStructTreeParse(t *testing.T) {
	t.Run("entries", checkStructTreeEntries)
	t.Run("cycle", checkStructTreeCycle)
	t.Run("depth", checkStructTreeDepth)
}

func checkStructTreeEntries(t *testing.T) {
	t.Helper()
	file := mustOpen(t, structTreeFixture(t))
	if !file.HasStructTree() {
		t.Fatal("HasStructTree() = false")
	}
	checkStructMarkInfo(t, file)
	checkStructHeader(t, file)
	tree, err := file.StructTree()
	if err != nil {
		t.Fatal(err)
	}
	if tree == nil {
		t.Fatal("StructTree() = nil")
	}
	checkStructLang(t, tree)
	checkStructNamespaces(t, tree)
	checkStructTreeKids(t, tree)
}

func checkStructMarkInfo(t *testing.T, file *File) {
	t.Helper()
	info := file.MarkInfo()
	if !info.Marked || info.Suspects {
		t.Fatalf("MarkInfo = %+v", info)
	}
}

func checkStructHeader(t *testing.T, file *File) {
	t.Helper()
	if !bytes.Equal(file.Header(), []byte(taggedHeader)) {
		t.Fatalf("Header() = %q", file.Header())
	}
}

func checkStructLang(t *testing.T, tree *StructTree) {
	t.Helper()
	if tree.Lang != "en-US" {
		t.Fatalf("Lang = %q", tree.Lang)
	}
}

func checkStructNamespaces(t *testing.T, tree *StructTree) {
	t.Helper()
	if len(tree.Namespaces) != 2 {
		t.Fatalf("Namespaces = %d", len(tree.Namespaces))
	}
	namespace := tree.Namespaces["http://example.com/schema"]
	if namespace == nil {
		t.Fatal("missing custom namespace")
	}
	if role, ok := namespace.Roles["Widget"]; !ok || role.Type != figureType {
		t.Fatalf("RoleMapNS = %+v", namespace.Roles)
	}
}

func checkStructTreeKids(t *testing.T, tree *StructTree) {
	t.Helper()
	root := tree.Root()
	if root == nil {
		t.Fatal("Root() = nil")
	}
	if root.Type != documentType || root.Role != documentType || root.Object != structTreeDocNum {
		t.Fatalf("root = %+v", root)
	}
	if root.ParentNum != structTreeRootNum || root.Parent != nil {
		t.Fatalf("root parent = %d %v", root.ParentNum, root.Parent)
	}
	if len(root.Kids) != 2 {
		t.Fatalf("kids = %d", len(root.Kids))
	}
	checkStructTreePara(t, root)
	checkStructTreeFigure(t, root)
}

func checkStructTreePara(t *testing.T, root *StructElem) {
	t.Helper()
	para := root.Kids[0]
	if para.Type != "Para" || para.Role != "P" {
		t.Fatalf("para = %+v", para)
	}
	if para.Parent != root || para.Page != 0 {
		t.Fatalf("para parent/page = %v %d", para.Parent, para.Page)
	}
	if len(para.Items) != 1 || para.Items[0] != (StructKey{Page: 0, MCID: 0}) {
		t.Fatalf("para items = %v", para.Items)
	}
}

func checkStructTreeFigure(t *testing.T, root *StructElem) {
	t.Helper()
	figure := root.Kids[1]
	checkFigureEntries(t, figure)
	checkFigureContent(t, figure)
}

func checkFigureEntries(t *testing.T, figure *StructElem) {
	t.Helper()
	if figure.Type != "Widget" || figure.Role != figureType ||
		figure.RoleNS != nsPDF20SSN || figure.Namespace != "http://example.com/schema" {
		t.Fatalf("figure = %+v", figure)
	}
	if figure.Alt != "A figure" || figure.ActualText != figureType || figure.Lang != "fr" {
		t.Fatalf("figure text = %+v", figure)
	}
}

func checkFigureContent(t *testing.T, figure *StructElem) {
	t.Helper()
	if figure.Page != 1 || len(figure.Items) != 1 || figure.Items[0] != (StructKey{Page: 1, MCID: 0}) {
		t.Fatalf("figure page/items = %d %v", figure.Page, figure.Items)
	}
	if got, ok := figure.TextCover(); !ok || got != figureType {
		t.Fatalf("TextCover() = %q %v", got, ok)
	}
}

// The rich fixture numbers fixed objects for the assertions.
const (
	structTreeRootNum = 7
	structTreeDocNum  = 8
)

// structTreeFixture is two pages, a Document with a Para and a namespaced
// Widget, and a parent tree keyed by /StructParents on page 1 and the page
// object number on page 2.
func structTreeFixture(t *testing.T) []byte {
	t.Helper()
	doc := newTaggedDoc()
	doc.object("<< /Type /Catalog /Pages 2 0 R /MarkInfo << /Marked true /Suspects false >> " +
		"/StructTreeRoot 7 0 R /Lang (en-US) >>")
	doc.object("<< /Type /Pages /Kids [3 0 R 5 0 R] /Count 2 >>")
	doc.object("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 20 20] /Contents 4 0 R " +
		"/Resources << >> /StructParents 0 >>")
	doc.object(streamBody("", []byte("/P <</MCID 0>> BDC 0 0 m 10 0 l S EMC")))
	doc.object("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 20 20] /Contents 6 0 R " +
		"/Resources << >> >>")
	doc.object(streamBody("", []byte("/Figure <</MCID 0>> BDC 0 0 m 10 0 l S EMC")))
	doc.object("<< /Type /StructTreeRoot /K [8 0 R] /ParentTree 11 0 R " +
		"/ParentTreeNextKey 1 /RoleMap << /Para /P >> /Namespaces [12 0 R 13 0 R] >>")
	doc.object("<< /Type /StructElem /S /Document /P 7 0 R /K [9 0 R 10 0 R] >>")
	doc.object("<< /Type /StructElem /S /Para /P 8 0 R /K " +
		"<< /Type /MCR /Pg 3 0 R /MCID 0 >> >>")
	doc.object("<< /Type /StructElem /S /Widget /P 8 0 R /NS 12 0 R /Pg 5 0 R /K 0 " +
		"/Alt (A figure) /ActualText (Figure) /Lang (fr) >>")
	doc.object("<< /Nums [0 [9 0 R] 5 [10 0 R]] >>")
	doc.object("<< /Type /Namespace /NS (http://example.com/schema) " +
		"/RoleMapNS << /Widget [ /Figure 13 0 R ] >> >>")
	doc.object("<< /Type /Namespace /NS (http://iso.org/pdf2/ssn) >>")
	return doc.classic()
}

func checkStructTreeCycle(t *testing.T) {
	t.Helper()
	file := mustOpen(t, structTreeCycle(t))
	_, err := file.StructTree()
	wantJob(t, err, opStructTree, errLimit)
}

// structTreeCycle points one element /K at its own parent.
func structTreeCycle(t *testing.T) []byte {
	t.Helper()
	doc := newTaggedDoc()
	doc.object("<< /Type /Catalog /Pages 2 0 R /StructTreeRoot 4 0 R >>")
	doc.object("<< /Type /Pages /Kids [3 0 R] /Count 1 >>")
	doc.object("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 20 20] >>")
	doc.object("<< /Type /StructTreeRoot /K [5 0 R] >>")
	doc.object("<< /Type /StructElem /S /Document /K [6 0 R] >>")
	doc.object("<< /Type /StructElem /S /P /K [5 0 R] >>")
	return doc.classic()
}

func checkStructTreeDepth(t *testing.T) {
	t.Helper()
	file := mustOpen(t, structTreeDepth())
	_, err := file.StructTree()
	wantJob(t, err, opStructTree, errLimit)
}

// structTreeDepth builds a chain one element deeper than the cap.
func structTreeDepth() []byte {
	doc := newTaggedDoc()
	doc.object("<< /Type /Catalog /Pages 2 0 R /StructTreeRoot 4 0 R >>")
	doc.object("<< /Type /Pages /Kids [3 0 R] /Count 1 >>")
	doc.object("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 20 20] >>")
	doc.object("<< /Type /StructTreeRoot /K [5 0 R] >>")
	last := structDepthCap + 6
	for num := 5; num <= last; num++ {
		body := "<< /Type /StructElem /S /P"
		if num < last {
			body += fmt.Sprintf(" /K [%d 0 R]", num+1)
		}
		doc.object(body + " >>")
	}
	return doc.classic()
}

func TestParentTree(t *testing.T) {
	t.Run("both ways", checkParentTreeLookup)
	t.Run("missing parent", checkParentTreeMissing)
}

const (
	parentDocNum  = 8
	parentParaNum = 10
	parentFigNum  = 11
)

func checkParentTreeLookup(t *testing.T) {
	t.Helper()
	file := mustOpen(t, parentTreePDF(t, "0 [10 0 R] 5 [11 0 R] 42 8 0 R"))
	tree, err := file.StructTree()
	if err != nil {
		t.Fatal(err)
	}
	checkParentTreePage(t, tree, StructKey{Page: 0, MCID: 0}, parentParaNum)
	checkParentTreePage(t, tree, StructKey{Page: 1, MCID: 0}, parentFigNum)
	if _, err := tree.Lookup(StructKey{Page: 0, MCID: 1}); err == nil {
		t.Fatal("lookup of a missing MCID succeeded")
	} else {
		wantJob(t, err, opParentTree, errUndefined)
	}
	if parent, ok := tree.Parents[42]; !ok || parent.Object != parentDocNum {
		t.Fatalf("structure parent = %+v %v", parent, ok)
	}
}

// checkParentTreePage proves one key resolves both ways.
func checkParentTreePage(t *testing.T, tree *StructTree, key StructKey, object int) {
	t.Helper()
	elem, err := tree.Lookup(key)
	if err != nil {
		t.Fatal(err)
	}
	if elem.Object != object {
		t.Fatalf("lookup %v = %+v", key, elem)
	}
	if got, ok := tree.Key(elem); !ok || got != key {
		t.Fatalf("key of %d = %v %v", object, got, ok)
	}
}

func checkParentTreeMissing(t *testing.T) {
	t.Helper()
	file := mustOpen(t, parentTreePDF(t, "0 [10 0 R]"))
	_, err := file.StructTree()
	wantJob(t, err, opParentTree, errUndefined)
}

// parentTreePDF is two pages with a Document and two elements. nums is the
// /Nums array of the parent tree.
func parentTreePDF(t *testing.T, nums string) []byte {
	t.Helper()
	doc := newTaggedDoc()
	doc.object("<< /Type /Catalog /Pages 2 0 R /StructTreeRoot 7 0 R >>")
	doc.object("<< /Type /Pages /Kids [3 0 R 5 0 R] /Count 2 >>")
	doc.object("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 20 20] /Contents 4 0 R " +
		"/Resources << >> /StructParents 0 >>")
	doc.object(streamBody("", []byte("0 0 m 10 0 l S")))
	doc.object("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 20 20] /Contents 6 0 R " +
		"/Resources << >> >>")
	doc.object(streamBody("", []byte("0 0 m 10 0 l S")))
	doc.object("<< /Type /StructTreeRoot /K [8 0 R] /ParentTree 9 0 R >>")
	doc.object("<< /Type /StructElem /S /Document /K [10 0 R 11 0 R] >>")
	doc.object("<< /Nums [" + nums + "] >>")
	doc.object("<< /Type /StructElem /S /P /P 8 0 R /Pg 3 0 R /K 0 >>")
	doc.object("<< /Type /StructElem /S /Figure /P 8 0 R /Pg 5 0 R /K 0 /Alt (Two) >>")
	return doc.classic()
}

func TestRoleMap(t *testing.T) {
	t.Run("resolve", checkRoleResolve)
	t.Run("parse", checkRoleParse)
}

func checkRoleResolve(t *testing.T) {
	t.Helper()
	maps := &RoleMaps{
		defaultRoles: map[string]string{
			"Para":  "P",
			"A":     "B",
			"B":     "P",
			"Loop1": "Loop2",
			"Loop2": "Loop1",
			"Same":  "Same",
		},
		namespaces: map[string]map[string]RoleNS{
			"custom": {"Widget": {Type: "Figure"}},
			"self":   {"Bad": {Type: "Other", Namespace: "self"}},
			"left":   {"A2": {Type: "B2", Namespace: "right"}},
			"right":  {"B2": {Type: "A2", Namespace: "left"}},
		},
	}
	cases := []struct {
		name      string
		typ       string
		namespace string
		role      string
		roleNS    string
		op        string
		errName   string
	}{
		{name: "standard", typ: documentType, role: documentType},
		{name: "mapped", typ: "Para", role: "P"},
		{name: "chain", typ: "A", role: "P"},
		{name: "cycle", typ: "Loop1", op: opRoleMap, errName: errLimit},
		{name: "same", typ: "Same", op: opRoleMap, errName: errSyntax},
		{name: "unmapped", typ: "Nope", op: opRoleMap, errName: errUndefined},
		{name: "namespace", typ: "Widget", namespace: "custom", role: figureType},
		{name: "same namespace", typ: "Bad", namespace: "self", op: opRoleMap, errName: errSyntax},
		{name: "namespace cycle", typ: "A2", namespace: "left", op: opRoleMap, errName: errLimit},
		{name: "pdf2 standard", typ: "P", namespace: nsPDF20SSN, role: "P", roleNS: nsPDF20SSN},
		{name: "pdf2 unmapped", typ: "Custom", namespace: nsPDF20SSN, op: opRoleMap, errName: errUndefined},
		{name: "unknown namespace", typ: "P", namespace: "other", op: opRoleMap, errName: errUndefined},
		{name: "mathml", typ: "msup", namespace: nsMathML, role: "msup", roleNS: nsMathML},
		{name: "default url", typ: "P", namespace: nsDefaultSSN, role: "P"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			role, roleNS, err := maps.Resolve(testCase.typ, testCase.namespace)
			if testCase.errName != "" {
				wantJob(t, err, testCase.op, testCase.errName)
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if role != testCase.role || roleNS != testCase.roleNS {
				t.Fatalf("Resolve() = %q %q, want %q %q", role, roleNS, testCase.role, testCase.roleNS)
			}
		})
	}
}

func checkRoleParse(t *testing.T) {
	t.Helper()
	role, err := roleTree(t, "<< /Para /P >>", "Para", "")
	if err != nil {
		t.Fatal(err)
	}
	if role != "P" {
		t.Fatalf("Role = %q", role)
	}
	role, err = roleNamespacedTree(t)
	if err != nil {
		t.Fatal(err)
	}
	if role != figureType {
		t.Fatalf("namespaced role = %q", role)
	}
	for _, testCase := range []struct {
		name    string
		roleMap string
		typ     string
		errName string
	}{
		{name: "cycle", roleMap: "<< /A /B /B /A >>", typ: "A", errName: errLimit},
		{name: "same", roleMap: "<< /Same /Same >>", typ: "Same", errName: errSyntax},
		{name: "unmapped", roleMap: "<< /Para /P >>", typ: "Nope", errName: errUndefined},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := roleTree(t, testCase.roleMap, testCase.typ, "")
			wantJob(t, err, opRoleMap, testCase.errName)
		})
	}
}

// roleTree builds one element with a root /RoleMap.
func roleTree(t *testing.T, roleMap, typ, nsRef string) (string, error) {
	t.Helper()
	doc := newTaggedDoc()
	doc.object("<< /Type /Catalog /Pages 2 0 R /StructTreeRoot 3 0 R >>")
	doc.object("<< /Type /Pages /Kids [] /Count 0 >>")
	doc.object("<< /Type /StructTreeRoot /K [4 0 R] /RoleMap " + roleMap + " >>")
	element := "<< /Type /StructElem /S /" + typ
	if nsRef != "" {
		element += " /NS " + nsRef
	}
	doc.object(element + " >>")
	file := mustOpen(t, doc.classic())
	tree, err := file.StructTree()
	if err != nil {
		return "", err
	}
	root := tree.Root()
	if root == nil {
		t.Fatal("Root() = nil")
	}
	return root.Role, nil
}

// roleNamespacedTree builds one element that maps through /RoleMapNS.
func roleNamespacedTree(t *testing.T) (string, error) {
	t.Helper()
	doc := newTaggedDoc()
	doc.object("<< /Type /Catalog /Pages 2 0 R /StructTreeRoot 3 0 R >>")
	doc.object("<< /Type /Pages /Kids [] /Count 0 >>")
	doc.object("<< /Type /StructTreeRoot /K [4 0 R] /Namespaces [5 0 R] >>")
	doc.object("<< /Type /StructElem /S /Widget /NS 5 0 R >>")
	doc.object("<< /Type /Namespace /NS (http://example.com/schema) " +
		"/RoleMapNS << /Widget [ /Figure 6 0 R ] >> >>")
	doc.object("<< /Type /Namespace /NS (http://iso.org/pdf2/ssn) >>")
	file := mustOpen(t, doc.classic())
	tree, err := file.StructTree()
	if err != nil {
		return "", err
	}
	root := tree.Root()
	if root == nil {
		t.Fatal("Root() = nil")
	}
	if root.RoleNS != nsPDF20SSN {
		t.Fatalf("RoleNS = %q", root.RoleNS)
	}
	return root.Role, nil
}

func TestTaggedFontCheck(t *testing.T) {
	t.Run("dictionaries", checkTaggedFontOK)
	t.Run("runs", checkTaggedTextOK)
}

func checkTaggedFontOK(t *testing.T) {
	t.Helper()
	checkFontAccepted(t)
	checkFontRejected(t)
}

// checkFontAccepted covers the two dictionary-level acceptance rules.
func checkFontAccepted(t *testing.T) {
	t.Helper()
	cases := map[string]Value{
		"tounicode": DictVal(map[string]Value{
			"Type":      NameVal("Font"),
			"Subtype":   NameVal("Type0"),
			"ToUnicode": RefVal(9, 0),
		}),
		"winansi": DictVal(map[string]Value{
			"Subtype":  NameVal("Type1"),
			"Encoding": NameVal("WinAnsiEncoding"),
		}),
		"base encoding": DictVal(map[string]Value{
			"Subtype": NameVal("TrueType"),
			"Encoding": DictVal(map[string]Value{
				"BaseEncoding": NameVal("MacRomanEncoding"),
				"Differences":  ArrayVal([]Value{IntVal(65), NameVal("A")}),
			}),
		}),
		"type0 tounicode": DictVal(map[string]Value{
			"Subtype":   NameVal("Type0"),
			"ToUnicode": StreamVal(nil, []byte("cmap")),
		}),
		"type3": DictVal(map[string]Value{
			"Subtype":  NameVal("Type3"),
			"Encoding": NameVal("StandardEncoding"),
		}),
	}
	for name, font := range cases {
		t.Run(name, func(t *testing.T) {
			if !TaggedFontOK(font) {
				t.Fatal("TaggedFontOK() = false")
			}
		})
	}
}

// checkFontRejected covers fonts that carry no dictionary-level mapping.
func checkFontRejected(t *testing.T) {
	t.Helper()
	cases := map[string]Value{
		"type0 no tounicode": DictVal(map[string]Value{
			"Subtype":  NameVal("Type0"),
			"Encoding": NameVal("Identity-H"),
		}),
		"simple no encoding": DictVal(map[string]Value{
			"Subtype": NameVal("Type1"),
		}),
		"custom differences only": DictVal(map[string]Value{
			"Subtype": NameVal("Type1"),
			"Encoding": DictVal(map[string]Value{
				"Differences": ArrayVal([]Value{IntVal(65), NameVal("A")}),
			}),
		}),
	}
	for name, font := range cases {
		t.Run(name, func(t *testing.T) {
			if TaggedFontOK(font) {
				t.Fatal("TaggedFontOK() = true")
			}
		})
	}
}

func checkTaggedTextOK(t *testing.T) {
	t.Helper()
	good := DictVal(map[string]Value{
		"Subtype":  NameVal("Type1"),
		"Encoding": NameVal("WinAnsiEncoding"),
	})
	bad := DictVal(map[string]Value{
		"Subtype": NameVal("Type0"),
	})
	if !TaggedTextOK(nil, []Value{good}) {
		t.Fatal("good fonts are not covered")
	}
	if TaggedTextOK(nil, []Value{bad}) {
		t.Fatal("bad font is covered")
	}
	if !TaggedTextOK(nil, nil) {
		t.Fatal("an empty run is not covered")
	}
	covered := &StructElem{ActualText: "text"}
	if !TaggedTextOK(covered, []Value{bad}) {
		t.Fatal("ActualText on the element does not cover")
	}
	parent := &StructElem{ActualText: "text"}
	child := &StructElem{Parent: parent}
	if !TaggedTextOK(child, []Value{bad}) {
		t.Fatal("ActualText on an ancestor does not cover")
	}
}
