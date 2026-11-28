package pdf

import (
	"bytes"
	"context"
	"errors"
	"image/color"
	"math"
	"runtime"
	"strings"
	"testing"
	"weak"

	"github.com/carlos7ags/folio/reader"
	"github.com/npillmayer/cords/styled"
	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/khipu/helpers"
	fontmeta "github.com/npillmayer/typography/font"
	glyphot "github.com/npillmayer/typography/glyphing/adapter/opentype"
	"github.com/npillmayer/typography/glyphing/paragraph"
	"github.com/npillmayer/typography/layout"
	"github.com/npillmayer/typography/render"
	renderot "github.com/npillmayer/typography/render/adapter/opentype"
	"github.com/npillmayer/uax/bidi"
	"golang.org/x/text/language"
)

func fragmentLayout(t *testing.T, input paragraph.Input, cfg khipu.Config, width dimen.DU, boxes []paragraph.AtomicBox) *layout.Fragment {
	t.Helper()
	options := khipu.PrepareOptions{}
	if cfg.Hyphenator != nil {
		options.Hyphenation = khipu.ExplicitHyphenation
	}
	p, err := paragraph.Prepare(input, options, cfg)
	if err != nil {
		t.Fatal(err)
	}
	out, err := p.Layout(paragraph.LayoutRequest{
		Params: paragraph.Params{Constraints: paragraph.Constraints{Width: width}, Breaker: paragraph.FirstFit,
			Assembly: paragraph.Options{Policy: paragraph.CSSLineHeight}, LineLayout: &paragraph.LineLayout{Default: paragraph.LineHeight{Kind: paragraph.NormalHeight}}},
		Horizontal: &paragraph.HorizontalLayout{}, Atomics: boxes,
	})
	if err != nil {
		t.Fatal(err)
	}
	f, err := paragraph.Fragment(out.VList)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func fragmentResult(t *testing.T, f *layout.Fragment, paint render.TextPaint) *render.FragmentResult {
	t.Helper()
	r, err := render.BuildFragmentResult(f, paint, renderot.New())
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func realConfig(face contracts.FontFace, size dimen.DU) khipu.Config {
	return khipu.Config{Streaming: &khipu.StreamingConfig{
		Resolver: helpers.DefaultStyleResolver{}, Defaults: contracts.ResolvedRunStyle{Font: face, EmSize: size, Script: language.MustParseScript("Latn"), Language: language.English},
		NewShaper: func() (contracts.StreamingShaper, error) { return glyphot.NewStream(8) },
	}}
}

func expectedFragmentGlyphs(t *testing.T, d *Document, r *render.FragmentResult, span render.IndexRange, x, y float64) []Glyph {
	t.Helper()
	var want []Glyph
	for i := span.Start; i < span.End; i++ {
		g, _ := r.Glyph(i)
		if g.Color.A == 0 {
			continue
		}
		f, _ := r.Font(g.Font)
		info, err := fontmeta.Identify(f.Instance.Face())
		if err != nil {
			t.Fatal(err)
		}
		var program *fontProgram
		for _, p := range d.fonts {
			if p.sha256 == info.SHA256 {
				program = p
				break
			}
		}
		if program == nil {
			t.Fatal("missing retained font program")
		}
		want = append(want, Glyph{Font: Font{owner: d, program: program, size: f.Instance.EmSize().Points()}, ID: g.GlyphID, X: x + g.X, Y: y + g.Y, Color: g.Color})
	}
	return want
}

func assertFragmentPage(t *testing.T, d *Document, index int, want []Glyph) {
	t.Helper()
	r := parsed(t, encoded(t, d))
	p, err := r.Page(index)
	if err != nil {
		t.Fatal(err)
	}
	stream, err := p.ContentStream()
	if err != nil {
		t.Fatal(err)
	}
	assertGlyphOps(t, stream, p.Height, want)
	var name string
	i := 0
	for _, op := range reader.ParseContentStream(stream) {
		if op.Operator == "Tf" {
			name = strings.TrimPrefix(op.Operands[0].Value, "/")
		}
		if op.Operator == "Tj" {
			if name != want[i].Font.program.name {
				t.Fatalf("glyph %d used font %s, want %s", i, name, want[i].Font.program.name)
			}
			i++
		}
	}
	if bytes.Contains(encoded(t, d), []byte("/ToUnicode")) || bytes.Contains(encoded(t, d), []byte("/ActualText")) {
		t.Fatal("introduced semantic text mapping")
	}
}

func TestFragmentPDFLigatureReuseAndTranslation(t *testing.T) {
	face, err := glyphot.LoadFile("../../opentype/testdata/fonts/ShaperTestProgress.ttf")
	if err != nil {
		t.Fatal(err)
	}
	cfg := realConfig(face, 12*dimen.BP)
	cfg.Streaming.Defaults.Features = []contracts.ShapeFeature{{Tag: [4]byte{'t', 'e', 's', 't'}, Enabled: true}}
	f := fragmentLayout(t, paragraph.Input{Text: styled.TextFromString("fi fi")}, cfg, 120*dimen.BP, nil)
	paint := render.TextPaint{Default: black, Spans: []render.SolidPaintSpan{{Start: 0, End: 1, Color: color.NRGBA{R: 255, A: 255}}, {Start: 3, End: 4, Color: color.NRGBA{B: 255, A: 255}}}}
	r := fragmentResult(t, f, paint)
	if r.GlyphCount() != 2 {
		t.Fatal("fixture must retain two ligatures", r.GlyphCount())
	}
	for i := range r.GlyphCount() {
		g, _ := r.Glyph(i)
		if g.GlyphID != 6 {
			t.Fatal("fixture did not shape ligature", g)
		}
	}
	var d, other Document
	for i := range 2 {
		p := page(t, &d, 300, 400-float64(i)*100)
		span := render.IndexRange{End: r.GlyphCount()}
		if i == 1 {
			span.Start = 1
		}
		x, y := 17.125-float64(i)*30, 28.375
		if err := p.PaintFragmentGlyphs(r, span, x, y); err != nil {
			t.Fatal(err)
		}
		assertFragmentPage(t, &d, i, expectedFragmentGlyphs(t, &d, r, span, x, y))
	}
	if len(d.fonts) != 1 {
		t.Fatal("repeated result fonts parsed twice", len(d.fonts))
	}
	// A second size and document must get their own sized binding and mutable use.
	cfg.Streaming.Defaults.EmSize = 24 * dimen.BP
	r24 := fragmentResult(t, fragmentLayout(t, paragraph.Input{Text: styled.TextFromString("fi")}, cfg, 120*dimen.BP, nil), render.TextPaint{Default: black, Spans: paint.Spans[:1]})
	if err := d.pages[0].PaintFragmentGlyphs(r24, render.IndexRange{End: 1}, 17, 70); err != nil {
		t.Fatal(err)
	}
	want := expectedFragmentGlyphs(t, &d, r, render.IndexRange{End: 2}, 17.125, 28.375)
	want = append(want, expectedFragmentGlyphs(t, &d, r24, render.IndexRange{End: 1}, 17, 70)...)
	assertFragmentPage(t, &d, 0, want)
	if len(d.fonts) != 1 {
		t.Fatal("size duplicated font program")
	}
	if err := page(t, &other, 100, 100).PaintFragmentGlyphs(r, render.IndexRange{End: 2}, 0, 0); err != nil {
		t.Fatal(err)
	}
	if d.fonts[0].embedded == other.fonts[0].embedded {
		t.Fatal("documents share mutable font usage")
	}
}

type markerHyphen struct{}

func (markerHyphen) Discretionaries(string, *khipu.Params) ([]khipu.HyphenationFragment, error) {
	return []khipu.HyphenationFragment{{PreBreak: "-"}}, nil
}

func TestFragmentPDFRTLAndSoftHyphenSuffix(t *testing.T) {
	face, err := glyphot.LoadFile("../../opentype/testdata/fonts/NotoSansHebrew-Regular.ttf")
	if err != nil {
		t.Fatal(err)
	}
	cfg := realConfig(face, 20*dimen.BP)
	cfg.Streaming.Defaults.Script = language.MustParseScript("Hebr")
	cfg.Streaming.Defaults.Language = language.Hebrew
	rtl := fragmentResult(t, fragmentLayout(t, paragraph.Input{Text: styled.TextFromString("אְב"), Direction: bidi.RightToLeft}, cfg, 150*dimen.BP, nil), render.TextPaint{Default: black})
	latin, _ := fixture(t)
	cfg = realConfig(latin, 10*dimen.BP)
	cfg.Hyphenator = markerHyphen{}
	f := fragmentLayout(t, paragraph.Input{Text: styled.TextFromString("co\u00adoperate")}, cfg, 18*dimen.BP, nil)
	if len(f.Lines()) < 2 {
		t.Fatal("fixture must break at SHY")
	}
	first, tail, err := paragraph.SplitFragment(f, 1)
	if err != nil {
		t.Fatal(err)
	}
	red := color.NRGBA{R: 255, A: 255}
	paint := render.TextPaint{Default: black, Spans: []render.SolidPaintSpan{{Start: 2, End: 4, Color: red}}}
	results := []*render.FragmentResult{rtl, fragmentResult(t, first, paint), fragmentResult(t, tail, paint)}
	redGlyphs := 0
	var d Document
	p := page(t, &d, 300, 300)
	var want []Glyph
	for i, r := range results {
		span := render.IndexRange{End: r.GlyphCount()}
		y := 20 + float64(i)*60
		if err := p.PaintFragmentGlyphs(r, span, 31.125, y); err != nil {
			t.Fatal(err)
		}
		want = append(want, expectedFragmentGlyphs(t, &d, r, span, 31.125, y)...)
		for j := range r.GlyphCount() {
			g, _ := r.Glyph(j)
			if g.Color == red {
				redGlyphs++
			}
		}
	}
	if redGlyphs != 1 || len(d.fonts) != 2 {
		t.Fatal("replacement paint or fonts", redGlyphs, len(d.fonts))
	}
	assertFragmentPage(t, &d, 0, want)
}

// Controlled shaping exercises arbitrary retained GIDs and borrowed faces;
// production PDF code must not depend on a particular shaping implementation.
type syntheticStream struct {
	*helpers.SyntheticStreamingShaper
}

func (syntheticStream) CheckFace(contracts.FontFace) error { return nil }
func (s syntheticStream) Shape(ctx context.Context, p contracts.ShapeParams, r contracts.RuneReader, o contracts.ShapeBufferOptions) error {
	p.Font, _ = contracts.NewFontInstance(helpers.NewSyntheticFont(), p.Font.EmSize())
	return s.SyntheticStreamingShaper.Shape(ctx, p, r, o)
}
func syntheticConfig(face contracts.FontFace) khipu.Config {
	cfg := realConfig(face, 12*dimen.BP)
	cfg.Streaming.NewShaper = func() (contracts.StreamingShaper, error) {
		s, e := helpers.NewSyntheticStreamingShaper(8)
		return syntheticStream{s}, e
	}
	return cfg
}

type retainedStyle struct{ contracts.ResolvedRunStyle }

func (s *retainedStyle) Equals(other styled.Style) bool {
	v, ok := other.(*retainedStyle)
	return ok && s == v
}
func (*retainedStyle) String() string { return "test font style" }

type retainedResolver struct{}

func (retainedResolver) ResolveStyle(s styled.Style, d contracts.ResolvedRunStyle) (contracts.ResolvedRunStyle, error) {
	if s == nil {
		return d, nil
	}
	return s.(*retainedStyle).ResolvedRunStyle, nil
}

func TestFragmentPDFFailureLeavesNoFontsOrPaint(t *testing.T) {
	face, _ := fixture(t)
	cfg := syntheticConfig(face)
	good := fragmentResult(t, fragmentLayout(t, paragraph.Input{Text: styled.TextFromString("ab")}, cfg, 100*dimen.BP, nil), render.TextPaint{Default: black})
	partial := fragmentResult(t, fragmentLayout(t, paragraph.Input{Text: styled.TextFromString("ab")}, cfg, 100*dimen.BP, nil), render.TextPaint{Default: black, Spans: []render.SolidPaintSpan{{Start: 1, End: 2, Color: color.NRGBA{A: 128}}}})
	badGID := fragmentResult(t, fragmentLayout(t, paragraph.Input{Text: styled.TextFromString("a\U00010000")}, cfg, 100*dimen.BP, nil), render.TextPaint{Default: black})
	var builder styled.TextBuilder
	if err := builder.AppendString("a", nil); err != nil {
		t.Fatal(err)
	}
	if err := builder.AppendString("b", &retainedStyle{contracts.ResolvedRunStyle{Font: opaqueFace{face}, EmSize: 12 * dimen.BP}}); err != nil {
		t.Fatal(err)
	}
	cfg.Streaming.Resolver = retainedResolver{}
	missingResource := fragmentResult(t, fragmentLayout(t, paragraph.Input{Text: builder.Text()}, cfg, 100*dimen.BP, nil), render.TextPaint{Default: black})
	for _, tc := range []struct {
		name  string
		r     *render.FragmentResult
		span  render.IndexRange
		x     float64
		cause error
	}{
		{"partial alpha", partial, render.IndexRange{End: 2}, 0, ErrUnsupported},
		{"invalid GID", badGID, render.IndexRange{End: 2}, 0, ErrInvalidInput},
		{"late font", missingResource, render.IndexRange{End: 2}, 0, ErrInvalidInput},
		{"invalid range", good, render.IndexRange{Start: 1, End: 3}, 0, ErrInvalidInput},
		{"negative range", good, render.IndexRange{Start: -1, End: 1}, 0, ErrInvalidInput},
		{"reversed range", good, render.IndexRange{Start: 2, End: 1}, 0, ErrInvalidInput},
		{"invalid origin", good, render.IndexRange{End: 2}, math.Inf(1), ErrInvalidInput},
		{"nil result", nil, render.IndexRange{}, 0, ErrInvalidInput},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var d Document
			p := page(t, &d, 100, 100)
			before := encoded(t, &d)
			err := p.PaintFragmentGlyphs(tc.r, tc.span, tc.x, 0)
			if !errors.Is(err, tc.cause) || len(d.fonts) != 0 || len(p.fonts) != 0 || !bytes.Equal(before, encoded(t, &d)) {
				t.Fatal("failed submission changed state", err)
			}
			if err := p.PaintFragmentGlyphs(good, render.IndexRange{End: 2}, 0, 0); err != nil {
				t.Fatal("retry", err)
			}
			if len(d.fonts) != 1 || d.fonts[0].name != "F1" {
				t.Fatal("failed call consumed font name")
			}
		})
	}
	var invalid *Page
	if err := invalid.PaintFragmentGlyphs(good, render.IndexRange{}, 0, 0); !errors.Is(err, ErrInvalidInput) {
		t.Fatal(err)
	}
	// An unsupported font outside the selected range must not block this call.
	var d Document
	if err := page(t, &d, 100, 100).PaintFragmentGlyphs(missingResource, render.IndexRange{End: 1}, 0, 0); err != nil {
		t.Fatal(err)
	}
	before := encoded(t, &d)
	if err := d.pages[0].PaintFragmentGlyphs(partial, render.IndexRange{End: 2}, 10, 20); !errors.Is(err, ErrUnsupported) || !bytes.Equal(before, encoded(t, &d)) {
		t.Fatal("failed replay changed an already used document", err)
	}
}

func TestFragmentPDFEmptyTransparentAndAtomicBoundaries(t *testing.T) {
	face, _ := fixture(t)
	cfg := syntheticConfig(face)
	f := fragmentLayout(t, paragraph.Input{Text: styled.TextFromString("a\uFFFC\uFFFCb"), Atomics: []contracts.AtomicAnchor{{ID: 7, ByteOffset: 1}, {ID: 2, ByteOffset: 4}}}, cfg, 100*dimen.BP, []paragraph.AtomicBox{
		{ID: 7, BorderWidth: 2 * dimen.BP, BorderHeight: 4 * dimen.BP, Margin: paragraph.AtomicMargins{Left: -3 * dimen.BP, Right: -4 * dimen.BP}, Alignment: paragraph.AlignBottomMarginEdge},
		{ID: 2, Alignment: paragraph.AlignBottomMarginEdge},
	})
	r := fragmentResult(t, f, render.TextPaint{Default: black})
	a, _ := r.Atomic(0)
	b, _ := r.Atomic(1)
	if a.BeforeGlyph != 1 || b.BeforeGlyph != 1 || b.X >= a.X {
		t.Fatal("fixture needs adjacent backward anchors", a, b)
	}
	var d Document
	p := page(t, &d, 100, 100)
	if err := p.PaintFragmentGlyphs(r, render.IndexRange{End: a.BeforeGlyph}, 10, 20); err != nil {
		t.Fatal(err)
	}
	// Explicit caller composition, not an atomic callback or CSS paint policy.
	if err := p.FillRect(10+a.X, 20+a.Y, 2, 4, black); err != nil {
		t.Fatal(err)
	}
	if err := p.PaintFragmentGlyphs(r, render.IndexRange{Start: b.BeforeGlyph, End: r.GlyphCount()}, 10, 20); err != nil {
		t.Fatal(err)
	}
	assertFragmentPage(t, &d, 0, expectedFragmentGlyphs(t, &d, r, render.IndexRange{End: r.GlyphCount()}, 10, 20))
	parsedPage, _ := parsed(t, encoded(t, &d)).Page(0)
	content, _ := parsedPage.ContentStream()
	var order []string
	for _, op := range reader.ParseContentStream(content) {
		if op.Operator == "Tj" || op.Operator == "re" {
			order = append(order, op.Operator)
		}
	}
	if strings.Join(order, ",") != "Tj,re,Tj" {
		t.Fatal("submission order changed or children drawn implicitly", order)
	}
	before := encoded(t, &d)
	empty, err := layout.NewFragment(f.Frozen(), layout.LineRange{}, layout.Box{}, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range []*render.FragmentResult{fragmentResult(t, empty, render.TextPaint{}), fragmentResult(t, f, render.TextPaint{})} {
		if err := p.PaintFragmentGlyphs(r, render.IndexRange{End: r.GlyphCount()}, 0, 0); err != nil {
			t.Fatal(err)
		}
	}
	if !bytes.Equal(before, encoded(t, &d)) {
		t.Fatal("empty or transparent submission changed output")
	}
	// Transparent glyphs need no font capability and create no unused resources.
	cfg = syntheticConfig(opaqueFace{face})
	invisible := fragmentResult(t, fragmentLayout(t, paragraph.Input{Text: styled.TextFromString("a")}, cfg, 100*dimen.BP, nil), render.TextPaint{})
	var transparent Document
	if err := page(t, &transparent, 100, 100).PaintFragmentGlyphs(invisible, render.IndexRange{End: 1}, 0, 0); err != nil || len(transparent.fonts) != 0 {
		t.Fatal(err)
	}
	// Skipping an invisible glyph must not advance, compact or renumber the
	// remaining retained origins or their public range/anchor coordinates.
	mixed := fragmentResult(t, f, render.TextPaint{Default: black, Spans: []render.SolidPaintSpan{{Start: 0, End: 1}}})
	span := render.IndexRange{End: mixed.GlyphCount()}
	if err := transparent.pages[0].PaintFragmentGlyphs(mixed, span, 10, 20); err != nil {
		t.Fatal(err)
	}
	assertFragmentPage(t, &transparent, 0, expectedFragmentGlyphs(t, &transparent, mixed, span, 10, 20))
}

type sealedScale struct {
	contracts.FontScale
	sealed *bool
}

func (s sealedScale) ToDU(n int32) float64 {
	if *s.sealed {
		panic("queried scale after result construction")
	}
	return s.FontScale.ToDU(n)
}

type fragmentFace struct {
	fontmeta.ResourceFace
	sealed *bool
}

func (f *fragmentFace) Metrics() (contracts.FontMetrics, error) {
	if *f.sealed {
		panic("queried metrics after construction")
	}
	return f.ResourceFace.Metrics()
}
func (f *fragmentFace) Scale(size dimen.DU) (contracts.FontScale, error) {
	if *f.sealed {
		panic("rebound font")
	}
	s, e := f.ResourceFace.Scale(size)
	return sealedScale{s, f.sealed}, e
}

func paintedLifetimeFixture(t *testing.T) (*Document, weak.Pointer[render.FragmentResult], weak.Pointer[fragmentFace]) {
	face, _ := fixture(t)
	sealed := false
	guard := &fragmentFace{face, &sealed}
	r := fragmentResult(t, fragmentLayout(t, paragraph.Input{Text: styled.TextFromString("ab")}, syntheticConfig(guard), 100*dimen.BP, nil), render.TextPaint{Default: black})
	sealed = true
	d := &Document{}
	p := page(t, d, 100, 100)
	if err := p.PaintFragmentGlyphs(r, render.IndexRange{End: r.GlyphCount()}, 0, 0); err != nil {
		t.Fatal(err)
	}
	return d, weak.Make(r), weak.Make(guard)
}

func TestFragmentPDFReleasesResultAndFontServices(t *testing.T) {
	d, result, face := paintedLifetimeFixture(t)
	for range 5 {
		runtime.GC()
	}
	if result.Value() != nil || face.Value() != nil {
		t.Fatal("document retains upstream result or services")
	}
	if parsed(t, encoded(t, d)).PageCount() != 1 {
		t.Fatal("lost output after upstream collection")
	}
	runtime.KeepAlive(d)
}
