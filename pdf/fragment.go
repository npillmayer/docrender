package pdf

import (
	"fmt"

	"github.com/npillmayer/typography/render"
)

// PaintFragmentGlyphs paints a half-open range of a retained result's glyph
// table, in its existing order. To paint all glyphs, pass
// render.IndexRange{End: result.GlyphCount()}. x and y translate fragment-local
// origins into page-local big points, Y down. No baseline or frame offset is
// added implicitly; finite positions outside the page are allowed.
//
// The caller chooses submissions and their order. Atomic anchors, child boxes
// and line allocation are not drawn. A bridge can submit ranges separated at
// AtomicAnchor.BeforeGlyph without assigning CSS paint policy to this adapter.
//
// Fully transparent glyphs are no-ops; partial alpha is unsupported. Only visible
// selected glyphs require PDF font resources. Fonts use the captured UnitsToBP
// and existing sized instance; no shaping, ordering, metrics, scale rebinding or
// outline extraction occurs. Source text and semantic mappings are not needed.
//
// The result is borrowed for the call and never retained. Binding, range, paint
// or glyph validation failure changes neither the page nor document font state.
// Nonnil results with an empty range are no-ops. Use the Document serially.
func (p *Page) PaintFragmentGlyphs(result *render.FragmentResult, glyphRange render.IndexRange, x, y float64) error {
	if p == nil || p.owner == nil || result == nil || !finite(x) || !finite(y) {
		return fmt.Errorf("%w: fragment result, page or origin", ErrInvalidInput)
	}
	if glyphRange.Start < 0 || glyphRange.End < glyphRange.Start || glyphRange.End > result.GlyphCount() {
		return fmt.Errorf("%w: fragment glyph range [%d,%d)", ErrInvalidInput, glyphRange.Start, glyphRange.End)
	}
	bindings := p.owner.stageFonts()
	fonts := make(map[int]Font)
	glyphs := make([]Glyph, 0, glyphRange.End-glyphRange.Start)
	for i := glyphRange.Start; i < glyphRange.End; i++ {
		g, _ := result.Glyph(i)
		if g.Color.A == 0 {
			continue
		}
		if g.Color.A != 255 {
			return fmt.Errorf("%w: fragment glyph %d requires opaque RGB or transparent paint", ErrUnsupported, i)
		}
		gx, gy := x+g.X, y+g.Y
		if !finite(gx) || !finite(gy) || !finite(p.height-gy) {
			return fmt.Errorf("%w: fragment glyph %d translated origin", ErrInvalidInput, i)
		}
		font, exists := fonts[g.Font]
		if !exists {
			retained, ok := result.Font(g.Font)
			if !ok {
				return fmt.Errorf("%w: fragment glyph %d font index %d", ErrInvalidInput, i, g.Font)
			}
			var err error
			font, err = bindings.bind(retained.Instance, retained.UnitsToBP)
			if err != nil {
				return fmt.Errorf("pdf: fragment glyph %d font %d: %w", i, g.Font, err)
			}
			fonts[g.Font] = font
		}
		glyphs = append(glyphs, Glyph{Font: font, ID: g.GlyphID, X: gx, Y: gy, Color: g.Color})
	}
	if err := p.PaintGlyphs(glyphs); err != nil {
		return fmt.Errorf("pdf: fragment glyph range [%d,%d): %w", glyphRange.Start, glyphRange.End, err)
	}
	p.owner.fonts = bindings.programs
	return nil
}
