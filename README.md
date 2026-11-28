# docrender

Bottom-up rendering work for the typesetting pipeline. The first implemented
layer is [`pdf`](pdf): a small Folio adapter for already selected, positioned
glyphs and opaque rectangle fills. The page-layout bridge, immutable fragment
result and CSS paint traversal remain under design.

## Current boundary

```go
var doc pdf.Document // zero value is ready
font, err := doc.BindFont(instance) // contracts.FontInstance, already sized
if err != nil { return err }
page, err := doc.AddPage(595, 842) // big points, 1/72 inch
if err != nil { return err }
err = page.PaintGlyphs([]pdf.Glyph{
    {Font: font, ID: selectedGID, X: finalX, Y: finalY,
        Color: color.NRGBA{A: 255}},
})
if err != nil { return err }
_, err = doc.WriteTo(destination) // caller-owned io.Writer
```

Origins are page-local, top-left, Y down; glyph origins already include all
shaping offsets and layout adjustments. Submission order is paint order. Each
glyph receives an absolute PDF text matrix, so Folio's nominal widths cannot
change the next origin. This immediate backend API does not select a display-list
design or paint-traversal owner.

Font faces must provide `typography/font.ResourceFace`. The adapter parses the
same retained bytes into a private Folio model and validates the bound scale.
It never reopens a font path, reshapes text or asks the upstream face for new
metrics. Identical programs share document-local embedding state across sizes;
font handles belong to their originating document. This first adapter embeds
the original programs in full: Folio v0.11.0's subsetter needs a further local
adaptation for glyph-only input. See the backend audit for details.

The initial profile supports static, single-face TrueType outlines and opaque
`color.NRGBA` paint. CFF, collections, variable/color fonts and transparency are
rejected. Semantic text, tagging and extraction guarantees are deferred.
Use a document serially; separate documents own separate mutable font state.
Painting validates each complete batch before mutation. A failed write may leave
partial bytes at the destination; retry with a fresh writer.

## Build and proof

Folio is pinned to **v0.11.0**, which requires **Go 1.26**. `go.mod` uses local
replacements for the sibling development checkouts: `typography`, `khipu`,
`opentype`, `cords`, `uax` and `schuko`. The tests and proof use retained font
fixtures from `../opentype/testdata/fonts`; they do not discover installed fonts.

From this module directory:

```sh
go test ./...
go test -race ./...
go vet ./...
go run ./cmd/folio-proof
```

The proof writes `output/pdf/folio-proof.pdf`. Options `-o`, `-latin` and `-hebrew`
override its destination and fixture paths. It uses the existing OpenType shaper
and visual-order helper, then supplies final positions directly to the adapter.
Its fixed rows exercise Latin ligatures, Hebrew marks, extra spacing, several
font sizes and solid paint on two pages. It does not exercise pagination,
fragment traversal, atomic composition or CSS paint order.

Tests inspect serialized page streams, exact GIDs, resource sharing, embedded
composite outlines, validation atomicity and output retries. The proof can also
be checked with `qpdf --check` and rendered with Poppler.

See the [backend audit](doc/folio_pdf_backend_integration_brief.md) and
[pipeline handoff](doc/docrender_rendering_pipeline_folio_handoff.md).
