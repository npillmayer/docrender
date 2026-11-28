# docrender

Bottom-up rendering work for the typesetting pipeline. [`pdf`](pdf) connects
typography's immutable `FragmentResult` to a small Folio adapter for positioned
glyphs and opaque rectangle fills. Page composition, CSS paint traversal
ownership and display-list design remain open.

## Current boundary

```go
var doc pdf.Document // zero value is ready
page, err := doc.AddPage(595, 842) // big points, 1/72 inch
if err != nil { return err }
// result is a completed *render.FragmentResult with resolved glyph colors.
err = page.PaintFragmentGlyphs(result,
    render.IndexRange{End: result.GlyphCount()}, 48, 60)
if err != nil { return err }
_, err = doc.WriteTo(destination) // caller-owned io.Writer
```

The half-open range indexes the result's glyph table. The explicit X/Y translation
is added once to each retained origin; no baseline, frame or atomic offset is
inferred. Empty ranges are no-ops. Atomic anchors and child interiors are not
drawn. A caller may split submissions at `AtomicAnchor.BeforeGlyph` and interleave
its own drawing; this method supplies no composition or CSS ordering policy.

Origins are page-local, top-left, Y down; glyph origins already include all
shaping offsets and layout adjustments. Submission order is paint order. Each
glyph receives an absolute PDF text matrix, so Folio's nominal widths cannot
change the next origin. This immediate backend API does not select a display-list
design or paint-traversal owner.

For callers with individual backend glyphs, `Document.BindFont(instance)` and
`Page.PaintGlyphs([]pdf.Glyph{...})` remain available. Their font handles belong
to one document and carry an explicit size; paint and origin are per glyph.

Font faces must provide `typography/font.ResourceFace`. The adapter parses the
same retained bytes into a private Folio model and validates the bound scale.
Fragment submission uses captured `UnitsToBP` without querying the scale service.
It never reopens a font path, reshapes text or asks the upstream face for new
metrics. Identical programs share document-local embedding state across sizes;
font handles belong to their originating document. This first adapter embeds
the original programs in full: Folio v0.11.0's subsetter needs a further local
adaptation for glyph-only input. See the backend audit for details.

The initial profile supports static, single-face TrueType outlines and opaque
`color.NRGBA` paint. Fragment submission skips fully transparent glyphs without
binding their fonts; partial alpha fails. Direct glyph/rectangle calls require
opaque paint. CFF, collections and variable/color fonts remain unsupported.
Semantic text, tagging and extraction guarantees are deferred.
Use a document serially; separate documents own separate mutable font state.
Painting validates each complete submission before mutation, including staging
new font programs. Failed fragment submission leaves page content, font usage
and bindings unchanged. The result and its borrowed services are not retained;
writing needs only the document's private font bytes/state. A failed write may leave
partial bytes at the destination; retry with a fresh writer.

## Build and proof

The pipeline modules require **Go 1.27.4**. Folio is pinned to **v0.11.0**.
`go.mod` uses local replacements for the sibling development checkouts: `typography`, `khipu`,
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
override its destination and fixture paths. Each fixed row goes through
`paragraph.Prepare`, layout, `render.BuildFragmentResult` and
`Page.PaintFragmentGlyphs`. It exercises Latin ligatures, Hebrew marks, several
font sizes and solid paint on two pages. It does not exercise pagination,
atomic child composition or CSS paint order.

Tests inspect serialized page streams, exact GIDs, translated ranges, RTL,
ligatures, soft-hyphen suffixes, transparent paint, explicit atomic boundaries,
resource sharing, embedded composite outlines, validation atomicity and output
retries. GC checks verify release of the result and its font services. The proof can also
be checked with `qpdf --check` and rendered with Poppler.

See the [backend audit](doc/folio_pdf_backend_integration_brief.md) and
[pipeline handoff](doc/docrender_rendering_pipeline_folio_handoff.md).
