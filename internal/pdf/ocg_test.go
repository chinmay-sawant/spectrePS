package pdf

import (
	"testing"

	"github.com/chinmay-sawant/spectrePS/internal/graphics"
)

// ocgCatalog builds a catalog whose default configuration carries an OFF
// list. off is the /OFF body text, for example "5 0 R" or "".
func ocgCatalog(off string) string {
	offEntry := ""
	if off != "" {
		offEntry = " /OFF [" + off + "]"
	}
	return "<< /Type /Catalog /Pages 2 0 R /OCProperties " +
		"<< /OCGs [5 0 R] /D << /OCGs [5 0 R]" + offEntry + " >> >> >>"
}

// TestOCGVisibility proves content under an OFF group is skipped, content
// under an ON group paints, and a property dictionary and an inline
// membership dictionary are both honored. Alternate configurations and the
// /AS usage map stay out.
func TestOCGVisibility(t *testing.T) { //nolint:cyclop,funlen // one subtest per rule
	const properties = "<< /Properties << /OC1 5 0 R >> >>"
	t.Run("off group skipped", func(t *testing.T) {
		file := ocgPage(t, ocgCatalog("5 0 R"), properties,
			"/OC /OC1 BDC 1 0 0 rg 0 0 10 20 re f EMC 1 0 0 rg 10 0 10 20 re f")
		img := paintPage(t, file)
		wantPixel(t, img, 5, 10, whiteByte, whiteByte, whiteByte)
		wantPixel(t, img, 15, 10, 255, 0, 0)
	})
	t.Run("on group paints", func(t *testing.T) {
		file := ocgPage(t, ocgCatalog(""), properties,
			"/OC /OC1 BDC 1 0 0 rg 0 0 10 20 re f EMC")
		img := paintPage(t, file)
		wantPixel(t, img, 5, 10, 255, 0, 0)
	})
	t.Run("property dictionary", func(t *testing.T) {
		resources := "<< /Properties << /MC1 6 0 R >> >>"
		file := ocgPage(t, ocgCatalog("5 0 R"), resources,
			"/OC /MC1 BDC 1 0 0 rg 0 0 10 20 re f EMC",
			"<< /OC 5 0 R >>")
		img := paintPage(t, file)
		wantPixel(t, img, 5, 10, whiteByte, whiteByte, whiteByte)
	})
	t.Run("inline membership", func(t *testing.T) {
		file := ocgPage(t, ocgCatalog("5 0 R"), properties,
			"/OC << /OCGs [5 0 R] >> BDC 1 0 0 rg 0 0 10 20 re f EMC")
		img := paintPage(t, file)
		wantPixel(t, img, 5, 10, whiteByte, whiteByte, whiteByte)
	})
	t.Run("all on policy", func(t *testing.T) {
		file := ocgPage(t, ocgCatalog("5 0 R"), properties,
			"/OC << /OCGs [5 0 R] /P /AllOn >> BDC "+
				"1 0 0 rg 0 0 10 20 re f EMC")
		img := paintPage(t, file)
		wantPixel(t, img, 5, 10, whiteByte, whiteByte, whiteByte)
	})
	t.Run("no optional content", func(t *testing.T) {
		file := ocgPage(t, ocgCatalog("5 0 R"), "",
			"/Span BMC 1 0 0 rg 0 0 20 20 re f EMC")
		img := paintPage(t, file)
		wantPixel(t, img, 10, 10, 255, 0, 0)
	})
	t.Run("off group sink paired", func(t *testing.T) {
		file := ocgPage(t, ocgCatalog("5 0 R"), properties,
			"/OC /OC1 BDC 1 0 0 rg 0 0 20 20 re f EMC")
		content, err := file.Content(0)
		if err != nil {
			t.Fatal(err)
		}
		res, err := file.PageResources(0)
		if err != nil {
			t.Fatal(err)
		}
		log := &markedLog{}
		pixmap := graphics.NewPixmap(pageSide, pageSide)
		if err := PaintWith(t.Context(), content, pixmap, 1,
			PaintOptions{Resources: res, MarkedContent: log}); err != nil {
			t.Fatal(err)
		}
		img := shown(t, pixmap)
		if !rowWhite(img, 10) {
			t.Fatal("an OFF group painted pixels")
		}
		if len(log.events) != 2 || log.events[0].kind != "begin" || log.events[1].kind != "end" {
			t.Fatalf("events = %+v", log.events)
		}
		if log.events[0].tag != "OC" || log.events[0].depth != 1 || log.events[1].depth != 1 {
			t.Fatalf("events = %+v", log.events)
		}
	})
}

// TestOCGXObject proves /OC on an image XObject is honored before decode.
func TestOCGXObject(t *testing.T) {
	const xobjects = "<< /XObject << /Im0 6 0 R >> >>"
	t.Run("off image skipped", func(t *testing.T) {
		file := ocgPage(t, ocgCatalog("5 0 R"), xobjects,
			"2 0 0 2 0 0 cm /Im0 Do", ocImageBody(t))
		img := paintSmallPage(t, file)
		if !rowWhite(img, 0) || !rowWhite(img, 1) {
			t.Fatal("an OFF image XObject painted pixels")
		}
	})
	t.Run("on image paints", func(t *testing.T) {
		file := ocgPage(t, ocgCatalog(""), xobjects,
			"2 0 0 2 0 0 cm /Im0 Do", ocImageBody(t))
		img := paintSmallPage(t, file)
		wantPixel(t, img, 0, 0, 255, 0, 0)
		wantPixel(t, img, 1, 0, 0, 255, 0)
		wantPixel(t, img, 0, 1, 0, 0, 255)
		wantPixel(t, img, 1, 1, whiteByte, whiteByte, whiteByte)
	})
}

// ocImageBody builds one 2 by 2 RGB image whose /OC names group 5.
func ocImageBody(t *testing.T) string {
	t.Helper()
	body := "/Subtype /Image /Width 2 /Height 2 /ColorSpace /DeviceRGB " +
		"/BitsPerComponent 8 /Filter /FlateDecode /OC 5 0 R"
	return streamBody(body, flateRaw(t, rgbPixels()))
}

// ocgPage builds one page over an OCG fixture. Objects 1 through 5 are the
// catalog, pages, page, content, and one /Type /OCG group. The bodies start at
// object 6, so an /OC reference to group 5 always resolves.
func ocgPage(t *testing.T, catalog, resources, content string, bodies ...string) *File {
	t.Helper()
	doc := newDoc()
	doc.object(catalog)
	doc.object("<< /Type /Pages /Kids [3 0 R] /Count 1 >>")
	page := "<< /Type /Page /Parent 2 0 R /Contents 4 0 R"
	if resources != "" {
		page += " /Resources " + resources
	}
	doc.object(page + " >>")
	doc.object(streamBody("", []byte(content)))
	doc.object("<< /Type /OCG /Name (Off) >>")
	for _, body := range bodies {
		doc.object(body)
	}
	return mustOpen(t, doc.classic(""))
}
