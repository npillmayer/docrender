package pdf

import (
	"errors"
	"fmt"
	"image/color"
	"io"
	"math"

	"github.com/carlos7ags/folio/content"
	"github.com/carlos7ags/folio/core"
	"github.com/carlos7ags/folio/document"
)

var (
	// ErrInvalidInput identifies invalid geometry, GIDs or resource bindings.
	ErrInvalidInput = errors.New("pdf: invalid input")
	// ErrUnsupported identifies input outside the initial font/paint profile.
	ErrUnsupported = errors.New("pdf: unsupported by initial backend profile")
)

// Document owns page streams and font embedding state. Its zero value is ready
// to use. Do not copy a Document after first use. A Document and its Pages must
// be used serially; separate Documents have independent mutable state.
type Document struct {
	pages []*Page
	fonts []*fontProgram
}

// Page is created by Document.AddPage. Its zero value is invalid. Do not copy it.
type Page struct {
	owner         *Document
	width, height float64
	stream        *content.Stream
	fonts         map[*fontProgram]bool
}

// Glyph is one selected GID and its final baseline origin in page coordinates.
// Font and Color are explicit for every glyph; neither has an implicit default.
// Color.A must be 255 in this first profile. ID 0 explicitly paints .notdef.
type Glyph struct {
	Font  Font
	ID    uint32
	X, Y  float64
	Color color.NRGBA
}

// AddPage appends a page. Dimensions must be finite and within [0.000001, 14400]
// bp: the serialization precision through the PDF 1.7 default user-space limit.
func (d *Document) AddPage(width, height float64) (*Page, error) {
	if d == nil || !finite(width) || !finite(height) || width < 1e-6 || height < 1e-6 || width > 14400 || height > 14400 {
		return nil, fmt.Errorf("%w: page dimensions", ErrInvalidInput)
	}
	p := &Page{owner: d, width: width, height: height, stream: content.NewStream(), fonts: make(map[*fontProgram]bool)}
	d.pages = append(d.pages, p)
	return p, nil
}

// PaintGlyphs paints glyphs in the supplied order, retaining none of the slice.
// It validates the entire call before changing the page or used-glyph sets.
// Every origin is absolute: nominal PDF font widths never advance the next
// glyph. Positions may lie outside the page, but must be finite.
func (p *Page) PaintGlyphs(glyphs []Glyph) error {
	if p == nil || p.owner == nil {
		return fmt.Errorf("%w: page", ErrInvalidInput)
	}
	for i, g := range glyphs {
		if g.Font.owner != p.owner || g.Font.program == nil {
			return fmt.Errorf("%w: glyph %d has no font binding in this document", ErrInvalidInput, i)
		}
		if g.ID > math.MaxUint16 || int(g.ID) >= g.Font.program.embedded.Face().NumGlyphs() {
			return fmt.Errorf("%w: glyph %d ID %d outside bound font", ErrInvalidInput, i, g.ID)
		}
		if !finite(g.X) || !finite(g.Y) || !finite(p.height-g.Y) {
			return fmt.Errorf("%w: glyph %d origin", ErrInvalidInput, i)
		}
		if g.Color.A != 255 {
			return fmt.Errorf("%w: glyph %d paint requires opaque RGB", ErrUnsupported, i)
		}
	}
	if len(glyphs) == 0 {
		return nil
	}
	s := p.stream
	s.SaveState()
	s.BeginText()
	for _, g := range glyphs {
		program := g.Font.program
		program.used = true
		p.fonts[program] = true
		s.SetFillColorRGB(float64(g.Color.R)/255, float64(g.Color.G)/255, float64(g.Color.B)/255)
		s.SetFont(program.name, g.Font.size)
		s.SetTextMatrix(1, 0, 0, 1, g.X, p.height-g.Y)
		// Empty text registers exact GIDs without introducing a source-text
		// dependency. WriteTo removes Folio's resulting placeholder ToUnicode.
		s.ShowTextHex(program.embedded.EncodeGIDs([]uint16{uint16(g.ID)}, ""))
	}
	s.EndText()
	s.RestoreState()
	return nil
}

// FillRect paints an opaque rectangle with a top-left origin. Its extent must
// be finite and nonnegative. A zero width or height is a no-op.
func (p *Page) FillRect(x, y, width, height float64, paint color.NRGBA) error {
	if p == nil || p.owner == nil || !finite(x) || !finite(y) || !finite(width) || !finite(height) || width < 0 || height < 0 {
		return fmt.Errorf("%w: rectangle", ErrInvalidInput)
	}
	bottom := p.height - y - height
	if !finite(bottom) || !finite(x+width) || !finite(y+height) {
		return fmt.Errorf("%w: rectangle extent", ErrInvalidInput)
	}
	if paint.A != 255 {
		return fmt.Errorf("%w: rectangle paint requires opaque RGB", ErrUnsupported)
	}
	if width == 0 || height == 0 {
		return nil
	}
	p.stream.SaveState()
	p.stream.SetFillColorRGB(float64(paint.R)/255, float64(paint.G)/255, float64(paint.B)/255)
	p.stream.Rectangle(x, bottom, width, height)
	p.stream.Fill()
	p.stream.RestoreState()
	return nil
}

// WriteTo serializes the current document as PDF 1.7. At least one page is
// required. Repeated calls and later painting are supported; each call builds
// a fresh PDF object graph. Font resources include use across all current pages;
// the initial adapter embeds the original font programs in full.
//
// The caller owns the writer. An I/O failure may leave partial output there;
// the Document remains usable for retry with a fresh destination. No files
// are opened or closed here. No source text or usable ToUnicode mapping is
// emitted; the backend-created placeholder CMap is left unreferenced.
func (d *Document) WriteTo(out io.Writer) (int64, error) {
	if d == nil || len(d.pages) == 0 || out == nil {
		return 0, fmt.Errorf("%w: document needs pages and an output writer", ErrInvalidInput)
	}
	w := document.NewWriter("1.7")
	fontRefs := make(map[*fontProgram]*core.PdfIndirectReference)
	for _, f := range d.fonts {
		if !f.used {
			continue
		}
		fontRefs[f] = w.AddObject(f.buildObjects(w.AddObject))
	}
	pages := core.NewPdfDictionary()
	pages.Set("Type", core.NewPdfName("Pages"))
	pages.Set("Count", core.NewPdfInteger(len(d.pages)))
	pagesRef := w.AddObject(pages)
	kids := core.NewPdfArray()
	for _, p := range d.pages {
		fonts := core.NewPdfDictionary()
		for _, f := range d.fonts {
			if p.fonts[f] {
				fonts.Set(f.name, fontRefs[f])
			}
		}
		resources := core.NewPdfDictionary()
		resources.Set("Font", fonts)
		page := core.NewPdfDictionary()
		page.Set("Type", core.NewPdfName("Page"))
		page.Set("Parent", pagesRef)
		page.Set("MediaBox", core.NewPdfArray(core.NewPdfInteger(0), core.NewPdfInteger(0), core.NewPdfReal(p.width), core.NewPdfReal(p.height)))
		page.Set("Resources", resources)
		page.Set("Contents", w.AddObject(p.stream.ToPdfStream()))
		kids.Add(w.AddObject(page))
	}
	pages.Set("Kids", kids)
	catalog := core.NewPdfDictionary()
	catalog.Set("Type", core.NewPdfName("Catalog"))
	catalog.Set("Pages", pagesRef)
	w.SetRoot(w.AddObject(catalog))
	n, err := w.WriteTo(checkedWriter{out})
	if err != nil {
		return n, fmt.Errorf("pdf: serialize: %w", err)
	}
	return n, nil
}

func normalizeDescriptor(desc *core.PdfDictionary, f *fontProgram) {
	face := f.embedded.Face()
	scale := 1000 / float64(face.UnitsPerEm())
	for _, key := range []string{"Ascent", "Descent", "CapHeight"} {
		if v, ok := desc.Get(key).(*core.PdfNumber); ok {
			desc.Set(key, core.NewPdfReal(v.FloatValue()*scale))
		}
	}
	b := face.BBox()
	desc.Set("FontBBox", core.NewPdfArray(core.NewPdfReal(float64(b[0])*scale), core.NewPdfReal(float64(b[1])*scale), core.NewPdfReal(float64(b[2])*scale), core.NewPdfReal(float64(b[3])*scale)))
}

type checkedWriter struct{ io.Writer }

func (w checkedWriter) Write(p []byte) (int, error) {
	n, err := w.Writer.Write(p)
	if err == nil && n != len(p) {
		err = io.ErrShortWrite
	}
	return n, err
}

func finite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }
