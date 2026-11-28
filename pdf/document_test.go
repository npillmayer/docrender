package pdf

import (
	"bytes"
	"errors"
	"image/color"
	"io"
	"math"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/carlos7ags/folio/core"
	"github.com/carlos7ags/folio/reader"
	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/dimen"
	fontmeta "github.com/npillmayer/typography/font"
	"github.com/npillmayer/typography/fontresource"
	otfont "github.com/npillmayer/typography/glyphing/adapter/opentype"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
)

var black = color.NRGBA{A: 255}

func fixture(t *testing.T) (*otfont.Face, []byte) {
	t.Helper()
	data, err := os.ReadFile("../../opentype/testdata/fonts/GentiumPlus-R.ttf")
	if err != nil {
		t.Fatal(err)
	}
	// This origin does not exist. Binding and serialization must use the snapshot.
	r, err := fontresource.Read(bytes.NewReader(data), fontresource.Origin{Location: "absent/font.ttf"})
	if err != nil {
		t.Fatal(err)
	}
	f, err := otfont.LoadResource(r)
	if err != nil {
		t.Fatal(err)
	}
	return f, data
}

func bind(t *testing.T, d *Document, face contracts.FontFace, size dimen.DU) Font {
	t.Helper()
	i, err := contracts.NewFontInstance(face, size)
	if err != nil {
		t.Fatal(err)
	}
	f, err := d.BindFont(i)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func page(t *testing.T, d *Document, width, height float64) *Page {
	t.Helper()
	p, err := d.AddPage(width, height)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func encoded(t *testing.T, d *Document) []byte {
	t.Helper()
	var b bytes.Buffer
	n, err := d.WriteTo(&b)
	if err != nil || n != int64(b.Len()) {
		t.Fatalf("write: %d bytes, buffer %d, error %v", n, b.Len(), err)
	}
	return b.Bytes()
}

func parsed(t *testing.T, data []byte) *reader.PdfReader {
	t.Helper()
	r, err := reader.ParseWithOptions(data, reader.ReadOptions{Strictness: reader.StrictnessStrict})
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func resolve(t *testing.T, r *reader.PdfReader, obj core.PdfObject) core.PdfObject {
	t.Helper()
	result, err := r.ResolveObject(obj)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestTwoPagePositionsAndSharedFont(t *testing.T) {
	face, original := fixture(t)
	var d Document
	f12, f24 := bind(t, &d, face, 12*dimen.BP), bind(t, &d, face, 24*dimen.BP)
	a, err := face.GlyphIndex('A')
	if err != nil || a == 0 {
		t.Fatalf("fixture A: %d, %v", a, err)
	}
	accent, err := face.GlyphIndex('é')
	if err != nil || accent == 0 {
		t.Fatalf("fixture composite: %d, %v", accent, err)
	}
	p1, p2 := page(t, &d, 600, 800), page(t, &d, 600, 400)
	// Deliberately use nonnominal spacing, decreasing X and differing Y origins.
	want := [][]Glyph{
		{{f12, a, 42.125, 63.5, black}, {f12, a, 42.25, 73.75, black}, {f24, a, 12, 200, color.NRGBA{R: 255, A: 255}}},
		{{f12, accent, 17.5, 31.25, black}, {f12, 0, 80, 31.25, black}},
	}
	for i, p := range []*Page{p1, p2} {
		if err := p.FillRect(10, 20, 30, 40, black); err != nil {
			t.Fatal(err)
		}
		if err := p.PaintGlyphs(want[i]); err != nil {
			t.Fatal(err)
		}
	}
	data := encoded(t, &d)
	if !bytes.Equal(data, encoded(t, &d)) {
		t.Fatal("repeated serialization changed the PDF")
	}
	r := parsed(t, data)
	if r.PageCount() != 2 {
		t.Fatalf("pages: %d", r.PageCount())
	}
	var firstFont *core.PdfIndirectReference
	for i, expected := range want {
		p, err := r.Page(i)
		if err != nil {
			t.Fatal(err)
		}
		content, err := p.ContentStream()
		if err != nil {
			t.Fatal(err)
		}
		assertGlyphOps(t, content, p.Height, expected)
		ops := reader.ParseContentStream(content)
		if ops[2].Operator != "re" || number(t, ops[2].Operands[1]) != p.Height-60 {
			t.Fatal("rectangle order or top-left coordinate conversion changed")
		}
		res, err := p.Resources()
		if err != nil {
			t.Fatal(err)
		}
		fonts := resolve(t, r, res.Get("Font")).(*core.PdfDictionary)
		if fonts.Len() != 1 {
			t.Fatalf("expected one shared font program, got %d", fonts.Len())
		}
		ref := fonts.Get("F1").(*core.PdfIndirectReference)
		if i == 0 {
			firstFont = ref
		} else if ref.Num() != firstFont.Num() {
			t.Fatal("pages did not share the font program")
		}
	}
	type0 := resolve(t, r, firstFont).(*core.PdfDictionary)
	if type0.Get("ToUnicode") != nil {
		t.Fatal("placeholder Unicode mapping advertised")
	}
	cid := resolve(t, r, type0.Get("DescendantFonts").(*core.PdfArray).At(0)).(*core.PdfDictionary)
	if cid.Get("CIDToGIDMap").(*core.PdfName).Value != "Identity" {
		t.Fatal("GIDs were remapped")
	}
	descriptor := resolve(t, r, cid.Get("FontDescriptor")).(*core.PdfDictionary)
	backendFace := f12.program.embedded.Face()
	ascent := descriptor.Get("Ascent").(*core.PdfNumber).FloatValue()
	if math.Abs(ascent-float64(backendFace.Ascent())*1000/float64(backendFace.UnitsPerEm())) > 1e-6 {
		t.Fatalf("descriptor ascent not in PDF units: %g", ascent)
	}
	fontStream := resolve(t, r, descriptor.Get("FontFile2")).(*core.PdfStream)
	if !bytes.Equal(fontStream.Data, original) {
		t.Fatal("embedded font differs from the authoritative retained program")
	}
	if strings.Contains(type0.Get("BaseFont").(*core.PdfName).Value, "+") ||
		type0.Get("BaseFont").(*core.PdfName).Value != descriptor.Get("FontName").(*core.PdfName).Value ||
		type0.Get("BaseFont").(*core.PdfName).Value != cid.Get("BaseFont").(*core.PdfName).Value {
		t.Fatal("full-font resource names retain a subset tag or disagree")
	}
	// Check actual embedded outlines at the original GIDs, including a composite
	// used only on page two. This also guards a future subsetting implementation.
	orig, err := sfnt.Parse(original)
	if err != nil {
		t.Fatal(err)
	}
	embedded, err := sfnt.Parse(fontStream.Data)
	if err != nil {
		t.Fatal(err)
	}
	for _, gid := range []uint32{a, accent} {
		var b1, b2 sfnt.Buffer
		scale := fixed.Int26_6(orig.UnitsPerEm()) * 64
		before, err1 := orig.LoadGlyph(&b1, sfnt.GlyphIndex(gid), scale, nil)
		after, err2 := embedded.LoadGlyph(&b2, sfnt.GlyphIndex(gid), scale, nil)
		if err1 != nil || err2 != nil || len(before) == 0 || !reflect.DeepEqual(before, after) {
			t.Fatalf("embedded outline changed for GID %d: %v, %v", gid, err1, err2)
		}
	}
}

func number(t *testing.T, token reader.Token) float64 {
	t.Helper()
	v, err := strconv.ParseFloat(token.Value, 64)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func assertGlyphOps(t *testing.T, data []byte, height float64, want []Glyph) {
	t.Helper()
	var x, y, size float64
	var rgb [3]float64
	i, matrices := 0, 0
	for _, op := range reader.ParseContentStream(data) {
		switch op.Operator {
		case "Tm":
			if number(t, op.Operands[0]) != 1 || number(t, op.Operands[3]) != 1 {
				t.Fatal("glyph outline was reflected or scaled by text matrix")
			}
			x, y = number(t, op.Operands[4]), number(t, op.Operands[5])
			matrices++
		case "Tf":
			size = number(t, op.Operands[1])
		case "rg":
			for j := range rgb {
				rgb[j] = number(t, op.Operands[j])
			}
		case "Tj":
			if i >= len(want) || matrices != i+1 {
				t.Fatal("glyphs need one absolute matrix each")
			}
			g := want[i]
			if math.Abs(x-g.X) > 1e-6 || math.Abs(y-(height-g.Y)) > 1e-6 || math.Abs(size-g.Font.size) > 1e-6 {
				t.Fatalf("glyph %d placement (%g,%g,%g), want (%g,%g,%g)", i, x, y, size, g.X, height-g.Y, g.Font.size)
			}
			code := op.Operands[0].Value
			if len(code) != 2 || uint32(code[0])<<8|uint32(code[1]) != g.ID {
				t.Fatalf("glyph %d code %x, want %04x", i, code, g.ID)
			}
			for j, channel := range []uint8{g.Color.R, g.Color.G, g.Color.B} {
				if math.Abs(rgb[j]-float64(channel)/255) > 1e-6 {
					t.Fatalf("glyph %d color changed", i)
				}
			}
			i++
		case "TJ", "Td", "TD":
			t.Fatalf("unexpected pen-dependent operator %s", op.Operator)
		}
	}
	if i != len(want) {
		t.Fatalf("got %d glyphs, want %d", i, len(want))
	}
}

func TestPaintValidationIsAtomic(t *testing.T) {
	face, _ := fixture(t)
	var d, other Document
	f := bind(t, &d, face, 12*dimen.BP)
	foreign := bind(t, &other, face, 12*dimen.BP)
	p := page(t, &d, 100, 100)
	good := Glyph{Font: f, ID: 1, X: 10, Y: 20, Color: black}
	bad := []Glyph{good, good, good, good, good}
	bad[0].ID = 1 << 16
	bad[1].ID = uint32(f.program.embedded.Face().NumGlyphs())
	bad[2].X = math.NaN()
	bad[3].Font = foreign
	bad[4].Color.A = 128
	before := encoded(t, &d)
	for _, g := range bad {
		if err := p.PaintGlyphs([]Glyph{good, g}); err == nil {
			t.Fatal("invalid batch accepted")
		}
		if !bytes.Equal(before, encoded(t, &d)) {
			t.Fatal("failed batch changed page or font usage")
		}
	}
	if err := p.PaintGlyphs([]Glyph{good}); err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(before, encoded(t, &d)) {
		t.Fatal("valid retry did not paint")
	}
}

type failWriter struct{ err error }

func (w failWriter) Write([]byte) (int, error) { return 0, w.err }

type shortWriter struct{}

func (shortWriter) Write(p []byte) (int, error) { return len(p) - 1, nil }

func TestWriteFailureAndRetry(t *testing.T) {
	var d Document
	page(t, &d, 100, 100)
	failure := errors.New("destination failed")
	if _, err := d.WriteTo(failWriter{failure}); !errors.Is(err, failure) {
		t.Fatalf("lost output error: %v", err)
	}
	if _, err := d.WriteTo(shortWriter{}); !errors.Is(err, io.ErrShortWrite) {
		t.Fatalf("short write: %v", err)
	}
	if parsed(t, encoded(t, &d)).PageCount() != 1 {
		t.Fatal("failed output poisoned retry")
	}
	page(t, &d, 200, 200)
	if parsed(t, encoded(t, &d)).PageCount() != 2 {
		t.Fatal("later page omitted")
	}
}

// The slice makes this face non-comparable. Resource identity must never depend
// on comparing arbitrary FontFace interface values.
type guardedFace struct {
	fontmeta.ResourceFace
	nonComparable []byte
	locked        *bool
}

func (f guardedFace) Metrics() (contracts.FontMetrics, error) { panic("metrics queried") }
func (f guardedFace) Scale(size dimen.DU) (contracts.FontScale, error) {
	if *f.locked {
		panic("face scale rebound")
	}
	return f.ResourceFace.Scale(size)
}

func TestBindingUsesRetainedResourceAndBoundScale(t *testing.T) {
	face, _ := fixture(t)
	locked := false
	g := guardedFace{ResourceFace: face, locked: &locked}
	i, err := contracts.NewFontInstance(g, 12*dimen.BP)
	if err != nil {
		t.Fatal(err)
	}
	locked = true
	var d Document
	f, err := d.BindFont(i)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.BindFont(i); err != nil {
		t.Fatal(err)
	}
	if len(d.fonts) != 1 {
		t.Fatal("same program parsed again")
	}
	p := page(t, &d, 100, 100)
	if err := p.PaintGlyphs([]Glyph{{Font: f, ID: 1, X: 10, Y: 20, Color: black}}); err != nil {
		t.Fatal(err)
	}
	encoded(t, &d)
}

func TestInvalidInputsAndFontProfile(t *testing.T) {
	var d Document
	if _, err := d.BindFont(contracts.FontInstance{}); !errors.Is(err, ErrInvalidInput) {
		t.Fatal(err)
	}
	if _, err := d.WriteTo(io.Discard); !errors.Is(err, ErrInvalidInput) {
		t.Fatal(err)
	}
	for _, n := range []float64{0, 1e-8, -1, math.Inf(1), math.NaN(), 14401} {
		if _, err := d.AddPage(n, 100); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("page size %g: %v", n, err)
		}
	}
	p := page(t, &d, 100, 100)
	for _, n := range []float64{-1, math.Inf(1), math.NaN()} {
		if err := p.FillRect(0, 0, n, 20, black); !errors.Is(err, ErrInvalidInput) {
			t.Fatal(err)
		}
	}
	if err := p.FillRect(0, 0, 20, 20, color.NRGBA{}); !errors.Is(err, ErrUnsupported) {
		t.Fatal(err)
	}
	if err := checkFontProfile([]byte("truncated")); !errors.Is(err, ErrInvalidInput) {
		t.Fatal(err)
	}
	for _, signature := range []string{"OTTO", "ttcf", "wOFF", "wOF2"} {
		if err := checkFontProfile([]byte(signature + strings.Repeat("\x00", 8))); !errors.Is(err, ErrUnsupported) {
			t.Fatal(err)
		}
	}
	_, data := fixture(t)
	if err := checkFontProfile(data); err != nil {
		t.Fatal(err)
	}
	// Insert an unsupported variation table tag into an otherwise valid directory.
	copy(data[12:16], "fvar")
	if err := checkFontProfile(data); !errors.Is(err, ErrUnsupported) {
		t.Fatal(err)
	}
}
