//go:build ignore

// Command gen rebuilds the generated tables in this package from their
// upstream sources. Run it from anywhere in the module:
//
//	go run internal/font/gen.go
//
// The command needs the network. The product does not: package font reads the
// checked-in .go tables and opens nothing.
//
// # Sources and licenses
//
// The advance widths come from the Adobe Core 14 AFM files. The fetch source
// is the mirror at
//
//	https://github.com/tecnickcom/tc-font-core14-afms
//
// which carries the archive Adobe published at
//
//	https://www.adobe.com/devnet/font/pdfs/Core14_AFMs.zip
//
// The AFM files are copyright Adobe Systems Incorporated. The license that
// accompanies them reads:
//
//	This file and the 14 PostScript(R) AFM files it accompanies may be used,
//	copied, and distributed for any purpose and without charge, with or
//	without modification, provided that all copyright notices are retained;
//	that the AFM files are not distributed without this file; that all
//	modifications to this file or any of the AFM files are prominently noted
//	in the modified file(s); and that this paragraph is not modified. Adobe
//	Systems has no responsibility or obligation to support the use of the
//	AFM files.
//
// The generated header repeats every Adobe copyright notice found in the AFM
// files, and documentation/fonts.md carries the license text.
//
// The encoding tables come from Mozilla pdf.js:
//
//	https://github.com/mozilla/pdf.js/blob/master/src/core/encodings.js
//
// pdf.js is Apache License 2.0. Its arrays transcribe the code-to-name tables
// in ISO 32000-1 Annex D.2. Spectre follows Annex D.2 where Ghostscript's
// gs_mro_e.ps writes .notdef for MacRoman codes that have no glyph in its
// substitute fonts.
//
// The glyph names come from the Adobe Glyph List 2.0, glyphlist.txt and
// zapfdingbats.txt:
//
//	https://github.com/adobe-type-tools/agl-aglfn
//
// That repository carries the BSD-3-Clause style Adobe license.
//
// Every fetched byte is checked against a SHA-256 digest pinned below, and
// every URL is pinned to a commit. The generated files are deterministic:
// keys are sorted and no timestamp is written.
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"go/format"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	afmCommit  = "0675784d24b28a55c607cad6b74596ce19ce333c"
	jsCommit   = "d52fdf411a6e4d338180687456e0df019e28475e"
	aglCommit  = "4036a9ca80a62f64f9de4f7321a9a045ad0ecfd6"
	afmURLBase = "https://raw.githubusercontent.com/tecnickcom/tc-font-core14-afms/" + afmCommit + "/"
	jsURL      = "https://raw.githubusercontent.com/mozilla/pdf.js/" + jsCommit + "/src/core/encodings.js"
	aglURLBase = "https://raw.githubusercontent.com/adobe-type-tools/agl-aglfn/" + aglCommit + "/"
)

// The digests pin the fetched bytes. Regeneration fails on a mismatch instead
// of quietly writing different tables.
var (
	afmSHA256 = map[string]string{
		"Courier.afm":               "521e0d7c7521efd4be78a5a9c5398e4c67d0771e396115b0346bc4ef74ada53d",
		"Courier-Bold.afm":          "ad0150d4bedcc8877742bf94251fcec13e348dd599d4603f679d92027d1e6e99",
		"Courier-Oblique.afm":       "b27103b2a2ef6030c110626597e2ab47bb8279075a039ab9170facb0aa1f70e1",
		"Courier-BoldOblique.afm":   "cb82e69ef5f6d421e8f404fe00bb0d993425aae79725331d0ebf847e94e97e92",
		"Helvetica.afm":             "da33f1870474c8e68bfe3e2353ff107ab6c6eea1f9836ce2aaf1e1a07b17982f",
		"Helvetica-Bold.afm":        "b880d96baf56d0cc059f258f60b4d764ef49b555ab9db294b959c0016dee41f2",
		"Helvetica-Oblique.afm":     "b4609b71b660a392ac09df35060271a876c2ce66617dd83bf826f742bb9d9721",
		"Helvetica-BoldOblique.afm": "69984a35ca26973a39f261cf83e0d367ea2e6517c590b0b030d4d4a219d9c269",
		"Times-Roman.afm":           "768e1cabea085d489a63da3e80b96bc5abf0ec98d3073c9b4d6ba76e7bccba64",
		"Times-Bold.afm":            "b4a000ed85cb22c6cdd985aa0fd3f6f78ed5079b7c6860dc4f0234e0d0e3c522",
		"Times-Italic.afm":          "ed37fa2e6a67b5b17dfd47f36fc7e90df32891a4408860dbc8d4cbbe9959e242",
		"Times-BoldItalic.afm":      "93c4744ba955215de02c4aae0b777133442ada2f7ab5a2af30a040b792b3c55d",
		"Symbol.afm":                "3d2128a820375a10de9bc8bf6cfb15ded482c01ca0f95cc0b3277f37ec8bde66",
		"ZapfDingbats.afm":          "a32565c90afd1b57a7008fc567b78d95cf1c22adff5e086094d666d88b039859",
	}
	jsSHA256 = "eee7b0b49fbf0c27fd1765abeea43621c23da3d1a8a592ad0e5ddfe6743658e3"
	aglFiles = map[string]string{
		"glyphlist.txt":    "a3b2f61ced9f3644cc0d4ecde5c59df34ca286c689d9484a43a710a81c466789",
		"zapfdingbats.txt": "f6394e3cb8a447e84a1dad75d4baaf2aa7f45dc104faf369f4720e1a774ef2dc",
	}
)

// standard14 lists the fonts in AFM order. vars is the Go identifier prefix
// used by the generated tables.
var standard14 = []struct {
	file string
	vars string
}{
	{"Courier", "courier"},
	{"Courier-Bold", "courierBold"},
	{"Courier-Oblique", "courierOblique"},
	{"Courier-BoldOblique", "courierBoldOblique"},
	{"Helvetica", "helvetica"},
	{"Helvetica-Bold", "helveticaBold"},
	{"Helvetica-Oblique", "helveticaOblique"},
	{"Helvetica-BoldOblique", "helveticaBoldOblique"},
	{"Times-Roman", "timesRoman"},
	{"Times-Bold", "timesBold"},
	{"Times-Italic", "timesItalic"},
	{"Times-BoldItalic", "timesBoldItalic"},
	{"Symbol", "symbol"},
	{"ZapfDingbats", "zapfDingbats"},
}

// encodingNames are the three PDF simple-font encodings in the generated table.
var encodingNames = []string{"StandardEncoding", "WinAnsiEncoding", "MacRomanEncoding"}

type afmData struct {
	name    string
	widths  map[string]int
	codes   map[byte]int
	notices []string
}

func main() {
	client := &http.Client{Timeout: 60 * time.Second}
	fonts := fetchFonts(client)
	encodings := fetchEncodings(client)
	glyphs := fetchGlyphs(client)
	checkAnchors(fonts)

	dir := sourceDir()
	writeGo(filepath.Join(dir, "standard14_data.go"), standard14Source(fonts))
	writeGo(filepath.Join(dir, "widths_data.go"), widthsSource(fonts))
	writeGo(filepath.Join(dir, "codes_data.go"), codesSource(fonts))
	writeGo(filepath.Join(dir, "encodings_data.go"), encodingsSource(encodings))
	writeGo(filepath.Join(dir, "agl_data.go"), aglSource(glyphs))
	fmt.Println("wrote generated tables to", dir)
}

// sourceDir is the directory that holds this file, so the command can run
// from any working directory.
func sourceDir() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		fail("cannot locate the generator source")
	}
	return filepath.Dir(file)
}

func fetchFonts(client *http.Client) []*afmData {
	fonts := make([]*afmData, 0, len(standard14))
	for _, spec := range standard14 {
		file := spec.file + ".afm"
		data := fetch(client, afmURLBase+file, afmSHA256[file])
		font, err := parseAFM(data)
		if err != nil {
			fail("parse %s: %v", file, err)
		}
		if font.name != spec.file {
			fail("%s: FontName %q", file, font.name)
		}
		fonts = append(fonts, font)
	}
	return fonts
}

func fetchEncodings(client *http.Client) map[string][]string {
	data := fetch(client, jsURL, jsSHA256)
	out := make(map[string][]string, len(encodingNames))
	for _, name := range encodingNames {
		table, err := parseJSArray(data, name)
		if err != nil {
			fail("parse %s: %v", name, err)
		}
		out[name] = table
	}
	return out
}

func fetchGlyphs(client *http.Client) map[string]string {
	glyphs := map[string]string{}
	for file, digest := range aglFiles {
		data := fetch(client, aglURLBase+file, digest)
		if err := parseAGL(data, glyphs); err != nil {
			fail("parse %s: %v", file, err)
		}
	}
	return glyphs
}

func checkAnchors(fonts []*afmData) {
	byName := map[string]*afmData{}
	for _, font := range fonts {
		byName[font.name] = font
	}
	for _, anchor := range []struct {
		font  string
		glyph string
		want  int
	}{
		{"Helvetica", "A", 667},
		{"Helvetica", "space", 278},
		{"Times-Roman", "A", 722},
		{"Times-Roman", "space", 250},
		{"Symbol", "space", 250},
		{"ZapfDingbats", "space", 278},
	} {
		got, ok := byName[anchor.font].widths[anchor.glyph]
		if !ok || got != anchor.want {
			fail("anchor %s %s = %d (present %t), want %d", anchor.font, anchor.glyph, got, ok, anchor.want)
		}
	}
	for name, width := range byName["Courier"].widths {
		if width != 600 {
			fail("Courier %s = %d, want 600", name, width)
		}
	}
}

func fetch(client *http.Client, url, want string) []byte {
	resp, err := client.Get(url)
	if err != nil {
		fail("get %s: %v", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		fail("get %s: %s", url, resp.Status)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		fail("read %s: %v", url, err)
	}
	sum := sha256.Sum256(data)
	if got := hex.EncodeToString(sum[:]); got != want {
		fail("%s: sha256 %s, want %s", url, got, want)
	}
	return data
}

func parseAFM(data []byte) (*afmData, error) {
	font := &afmData{widths: map[string]int{}, codes: map[byte]int{}}
	seenNotice := map[string]bool{}
	for _, raw := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(raw)
		switch {
		case strings.HasPrefix(line, "FontName "):
			font.name = strings.TrimSpace(strings.TrimPrefix(line, "FontName "))
		case strings.HasPrefix(line, "Comment Copyright") && !seenNotice[line]:
			seenNotice[line] = true
			font.notices = append(font.notices, strings.TrimSpace(strings.TrimPrefix(line, "Comment ")))
		case strings.HasPrefix(line, "C "):
			if err := font.addMetric(line); err != nil {
				return nil, err
			}
		}
	}
	if font.name == "" {
		return nil, errors.New("no FontName line")
	}
	return font, nil
}

func (font *afmData) addMetric(line string) error {
	code, width := -1, -1
	name := ""
	for _, raw := range strings.Split(line, ";") {
		field := strings.TrimSpace(raw)
		switch {
		case strings.HasPrefix(field, "C "):
			code, _ = strconv.Atoi(strings.TrimSpace(field[2:]))
		case strings.HasPrefix(field, "WX "):
			width, _ = strconv.Atoi(strings.TrimSpace(field[3:]))
		case strings.HasPrefix(field, "N "):
			name = strings.TrimSpace(field[2:])
		}
	}
	if name == "" || width < 0 {
		return fmt.Errorf("bad char metric %q", line)
	}
	font.widths[name] = width
	if code < 0 {
		return nil
	}
	if code > 255 {
		return fmt.Errorf("code %d out of byte range in %q", code, line)
	}
	font.codes[byte(code)] = width
	return nil
}

// parseJSArray reads one "const Name = [...];" string array from the pdf.js
// source. The file is ahead of the Go module, so a regular expression is
// enough; a JavaScript parser would be more code than the job needs.
func parseJSArray(data []byte, name string) ([]string, error) {
	text := string(data)
	marker := "const " + name + " = ["
	start := strings.Index(text, marker)
	if start < 0 {
		return nil, fmt.Errorf("%s not found", name)
	}
	text = text[start+len(marker):]
	end := strings.Index(text, "];")
	if end < 0 {
		return nil, fmt.Errorf("%s is not closed", name)
	}
	matches := regexp.MustCompile(`"([^"]*)"`).FindAllStringSubmatch(text[:end], -1)
	table := make([]string, 0, len(matches))
	for _, match := range matches {
		table = append(table, match[1])
	}
	if len(table) != 256 {
		return nil, fmt.Errorf("%s has %d entries", name, len(table))
	}
	return table, nil
}

// parseAGL reads "name;XXXX YYYY" lines. The value is the UTF-8 string of the
// listed code points, so a ligature keeps all of its characters.
func parseAGL(data []byte, glyphs map[string]string) error {
	for i, raw := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, ";", 2)
		if len(parts) != 2 {
			return fmt.Errorf("line %d: %q", i+1, line)
		}
		name := strings.TrimSpace(parts[0])
		var value strings.Builder
		for _, field := range strings.Fields(parts[1]) {
			code, err := strconv.ParseUint(field, 16, 32)
			if err != nil {
				return fmt.Errorf("line %d: %v", i+1, err)
			}
			value.WriteRune(rune(code))
		}
		if old, ok := glyphs[name]; ok && old != value.String() {
			return fmt.Errorf("glyph name %q listed twice", name)
		}
		glyphs[name] = value.String()
	}
	return nil
}

func standard14Source(fonts []*afmData) []byte {
	var b bytes.Buffer
	generated(&b, "The standard 14 font list and the metrics lookup.")
	b.WriteString("package font\n\n")
	b.WriteString("var standard14Order = [14]string{\n")
	for _, font := range fonts {
		fmt.Fprintf(&b, "\t%q,\n", font.name)
	}
	b.WriteString("}\n\n")
	b.WriteString("var standard14Data = map[string]*Metrics{\n")
	for _, font := range fonts {
		vars := varsOf(font.name)
		fmt.Fprintf(&b, "\t%q: {name: %q, byName: %sByName, byCode: %sByCode},\n",
			font.name, font.name, vars, vars)
	}
	b.WriteString("}\n")
	return b.Bytes()
}

func widthsSource(fonts []*afmData) []byte {
	var b bytes.Buffer
	b.WriteString("// Code generated by gen.go from the Adobe Core 14 AFM files; DO NOT EDIT.\n")
	b.WriteString("//\n")
	b.WriteString("// Source: https://github.com/tecnickcom/tc-font-core14-afms\n")
	b.WriteString("// Original: https://www.adobe.com/devnet/font/pdfs/Core14_AFMs.zip\n")
	for _, notice := range allNotices(fonts) {
		fmt.Fprintf(&b, "// %s\n", notice)
	}
	b.WriteString("// License: see documentation/fonts.md.\n\n")
	b.WriteString("package font\n\n")
	for _, font := range fonts {
		names := sortedKeys(font.widths)
		items := make([]string, 0, len(names))
		for _, name := range names {
			items = append(items, fmt.Sprintf("%q: %d", name, font.widths[name]))
		}
		fmt.Fprintf(&b, "// %s advances by glyph name.\n", font.name)
		fmt.Fprintf(&b, "var %sByName = map[string]Width{\n", varsOf(font.name))
		pack(&b, items, 3)
		b.WriteString("}\n\n")
	}
	return b.Bytes()
}

func codesSource(fonts []*afmData) []byte {
	var b bytes.Buffer
	b.WriteString("// Code generated by gen.go from the Adobe Core 14 AFM files; DO NOT EDIT.\n")
	b.WriteString("//\n")
	b.WriteString("// The codes are the font's own encoding, as the AFM files list them.\n")
	b.WriteString("// Source and license: see documentation/fonts.md.\n\n")
	b.WriteString("package font\n\n")
	for _, font := range fonts {
		codes := make([]int, 0, len(font.codes))
		for code := range font.codes {
			codes = append(codes, int(code))
		}
		sort.Ints(codes)
		items := make([]string, 0, len(codes))
		for _, code := range codes {
			items = append(items, fmt.Sprintf("%d: %d", code, font.codes[byte(code)]))
		}
		fmt.Fprintf(&b, "// %s advances by character code.\n", font.name)
		fmt.Fprintf(&b, "var %sByCode = map[byte]Width{\n", varsOf(font.name))
		pack(&b, items, 6)
		b.WriteString("}\n\n")
	}
	return b.Bytes()
}

func encodingsSource(encodings map[string][]string) []byte {
	var b bytes.Buffer
	b.WriteString("// Code generated by gen.go from Mozilla pdf.js; DO NOT EDIT.\n")
	b.WriteString("//\n")
	b.WriteString("// Source: " + jsURL + "\n")
	b.WriteString("// License: Apache License 2.0. The tables transcribe ISO 32000-1 Annex D.2.\n\n")
	b.WriteString("package font\n\n")
	for _, name := range encodingNames {
		items := make([]string, 0, 256)
		for code, glyph := range encodings[name] {
			if glyph != "" {
				items = append(items, fmt.Sprintf("%d: %q", code, glyph))
			}
		}
		fmt.Fprintf(&b, "var %sNames = [256]string{\n", lowerFirst(name))
		pack(&b, items, 4)
		b.WriteString("}\n\n")
	}
	return b.Bytes()
}

func aglSource(glyphs map[string]string) []byte {
	var b bytes.Buffer
	b.WriteString("// Code generated by gen.go from the Adobe Glyph List; DO NOT EDIT.\n")
	b.WriteString("//\n")
	b.WriteString("// Source: https://github.com/adobe-type-tools/agl-aglfn\n")
	b.WriteString("// Files: glyphlist.txt and zapfdingbats.txt.\n")
	b.WriteString("// License: BSD-3-Clause style Adobe license; see documentation/fonts.md.\n\n")
	b.WriteString("package font\n\n")
	b.WriteString("var aglNames = map[string]string{\n")
	names := sortedKeys(glyphs)
	items := make([]string, 0, len(names))
	for _, name := range names {
		items = append(items, fmt.Sprintf("%q: %q", name, glyphs[name]))
	}
	pack(&b, items, 3)
	b.WriteString("}\n")
	return b.Bytes()
}

// generated writes the standard generated-file header. data files that derive
// from the AFM files add their own source and notices.
func generated(b *bytes.Buffer, summary string) {
	b.WriteString("// Code generated by gen.go; DO NOT EDIT.\n")
	b.WriteString("//\n")
	fmt.Fprintf(b, "// %s\n\n", summary)
}

// allNotices dedupes the Adobe copyright notices, sorted for determinism.
func allNotices(fonts []*afmData) []string {
	seen := map[string]bool{}
	notices := []string{}
	for _, font := range fonts {
		for _, notice := range font.notices {
			if !seen[notice] {
				seen[notice] = true
				notices = append(notices, notice)
			}
		}
	}
	sort.Strings(notices)
	return notices
}

func varsOf(name string) string {
	for _, spec := range standard14 {
		if spec.file == name {
			return spec.vars
		}
	}
	fail("no variable prefix for %s", name)
	return ""
}

func lowerFirst(name string) string {
	return strings.ToLower(name[:1]) + name[1:]
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// pack writes items, perLine to a line, indented with a tab. Every item ends
// in a comma, so gofmt leaves the packing alone.
func pack(b *bytes.Buffer, items []string, perLine int) {
	for start := 0; start < len(items); start += perLine {
		end := min(start+perLine, len(items))
		b.WriteString("\t")
		for i := start; i < end; i++ {
			if i > start {
				b.WriteString(" ")
			}
			b.WriteString(items[i])
			b.WriteString(",")
		}
		b.WriteString("\n")
	}
}

func writeGo(path string, source []byte) {
	formatted, err := format.Source(source)
	if err != nil {
		fail("format %s: %v\n%s", path, err, source)
	}
	if err := os.WriteFile(path, formatted, 0o644); err != nil {
		fail("write %s: %v", path, err)
	}
}

func fail(format string, args ...any) {
	log.Fatalf("gen: "+format, args...)
}
