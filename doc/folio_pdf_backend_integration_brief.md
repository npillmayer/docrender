# Folio PDF backend boundary for the two-page spike

**Status:** Boundary brief and first implementation audit, 28 November 2026,
under [REMARKS.md](REMARKS.md). The bottom adapter is implemented in
[`docrender/pdf`](../pdf) against **Folio v0.11.0**. Fragment glyph submission is
connected; selected page composition and CSS paint traversal remain planning
work. No display-list design is selected.

## Integration direction

Keep all Folio-specific adaptation in `docrender`. The upstream typesetter
supplies selected GIDs, exact sized font references and authoritative placement.
Folio supplies the backend mechanisms needed to emit the PDF. Its internal font
requirements must not reshape typography or pagelayout contracts.

The working font strategy permits two representations of the same immutable
font binary: one used by the chosen typesetting/shaping path, another built
privately for Folio. `docrender` keeps them associated. Reusing a parsed model
could be an optimization if convenient, but a single parse is not a goal that
justifies exposing parser internals or adding Folio-specific methods upstream.

This supersedes the earlier mandatory `font.Face` adapter/no-reparse approach
and semantic-text requirements. The audit below records the pinned APIs used
by the first implementation.

## Ownership

| Data or behavior | Owner |
| --- | --- |
| Selected GIDs and shaped advances/offsets | User-chosen shaper through the existing typesetter. |
| Final line/knot spacing, baselines and glyph/atomic origins | Layout and shared retained rendering traversal. |
| Resolved foreground and supported box paint | Rendering inputs and the separate paint algorithm. |
| CSS paint traversal | Encapsulated algorithm; module ownership open. |
| Pairing a retained font with its exact binary and Folio model | `docrender`. |
| PDF resource names, used-glyph sets, encoding/subsetting state | `docrender` backend integration and Folio. |
| Serialization and output finalization | PDF backend. |

The backend receives painting decisions and settled geometry. It does not walk
CSS trees, choose paint order, infer document styles or perform text layout.

## Pair the two font representations

At setup or first backend resource use, `docrender` must establish that the
Folio model corresponds to the exact font program used for the retained GIDs.
Use the same immutable binary, with any relevant face selection kept consistent.
The implemented profile accepts a single static SFNT face with monochrome
TrueType `glyf`/`loca` outlines. CFF, collections, variable/color fonts and font
containers are rejected. Gentium Plus and Noto Sans Hebrew are the proof fixtures.

The existing typography resource capability can provide retained bytes for
compatible faces. For a foreign face, an explicit caller-supplied association
may be required. Generic `FontFace` does not guarantee embedding support.
A missing association is a backend error, not permission to rediscover a font
by family name or substitute another program. The current `BindFont` requires
`ResourceFace`; caller-injected associations for other faces are future work.

Keep provenance/resource identity, font-program identity and sized-instance
identity distinct. Several sizes can share one program while retaining their
own placement scale. Cache design is local to `docrender`; do not compare
arbitrary interface values or treat a bare GID as globally meaningful.

The second parse consumes the retained binary. It does not require reopening
an original font path during replay. Backend private structures may have their
own lifetime; retained upstream font services remain borrowed and are not
closed by a fragment result.

Sources: [font instances](../../khipu/contracts/font_instance.go),
[resource identity capability](../../typography/font/identity.go),
[immutable resource](../../typography/fontresource/resource.go).

## Preserve authoritative positioning

The chosen shaper selects glyphs and supplies signed advances/offsets.
Layout supplies final knot advances, side skips, baselines and atomic positions.
Shared traversal computes final origins from those facts once.

A backend must reproduce those origins. Folio's nominal widths may be necessary
for its internal PDF font structures, but must not replace shaped spacing,
kerning, mark placement or layout justification. Do not ask Folio to shape or
remeasure the text.

The first experiment can explicitly position each glyph. If a later encoding
batches glyphs or adjusts a text pen, it must be demonstrably equivalent to the
stored origins and use authoritative upstream advances where advances are
needed. Adjacent offset origins alone are not a general reconstruction of
shaping advances.

Coordinate conversion and font-size selection occur once at the PDF boundary.
Positions, font size and scale must refer to the same retained instance. Validate
any narrowing of GID or coordinate representations rather than truncating.

## Pinned backend audit and local adaptation

The Go module proxy resolves `github.com/carlos7ags/folio@v0.11.0` to tag commit
`e3f1bc3851c79072d76f904b32e3fdd30117ceb4`; its module requires Go 1.26.0.
The adapter uses this pinned release without modifying Folio or sibling modules.

| Verified API/behavior | Local use in docrender |
| --- | --- |
| `font.ParseTTF` retains a private parsed model over supplied bytes. | Parse a copy read from the upstream immutable resource. Validate identity and the instance's bound scale against the parsed UPEM; never rebind the upstream face. |
| `EmbeddedFont.EncodeGIDs(gids, "")` registers exact 16-bit GIDs and returns big-endian codes. | Validate the GID against the actual glyph count before narrowing. No original-text input is introduced. |
| Empty text creates U+0000 placeholder `/ToUnicode` entries. | Remove the Type0 dictionary's `/ToUnicode` reference. Folio's generated CMap object remains unreferenced; it is not offered as semantic output. |
| `EmbeddedFont.BuildObjects` builds resources through an object-registration callback. Its subsetter preserves GIDs, but glyph-only use creates duplicate U+0000 cmap segments inside the subset and nondeterministic output. | Replace the generated FontFile2 payload with the exact original retained program and remove the subset prefix from resource names. Full-font embedding is the initial profile; glyph-only subsetting is deferred. One document-owned embedding state per program SHA256 is shared across sizes/pages; provenance identity is validated separately. |
| Font descriptor ascent, descent, cap height and bounding box are emitted in design units in this release. | Normalize those descriptor fields to PDF's 1000-em units inside the callback. Nominal widths remain backend metadata and never place glyphs. |
| `document.Page`'s embedded-font registration and stream initialization helpers are private; `ContentStream()` is nil before content exists. | Use public `content.NewStream`, `BuildObjects` and `document.Writer` directly. Construct only the catalog, page tree and resource dictionaries locally. No dummy text, reflection or dependency patch is needed. |
| `content.Stream.SetTextMatrix` and `ShowTextHex` emit explicit origins and codes. | One absolute, upright text matrix per glyph, converting page-local Y down to PDF Y up once. Folio serializes fractional numbers to six decimal places. |

Pinned sources: [font embedding](https://github.com/carlos7ags/folio/blob/v0.11.0/font/embed.go),
[font subsetting](https://github.com/carlos7ags/folio/blob/v0.11.0/font/subset.go),
[page helpers](https://github.com/carlos7ags/folio/blob/v0.11.0/document/page.go),
[writer](https://github.com/carlos7ags/folio/blob/v0.11.0/document/writer.go),
[content stream](https://github.com/carlos7ags/folio/blob/v0.11.0/content/stream.go).

## Implemented bottom boundary and ownership

The experimental API consists of zero-ready `pdf.Document`, `BindFont`,
`AddPage`, `Page.PaintGlyphs`, `Page.PaintFragmentGlyphs`, `Page.FillRect` and
`Document.WriteTo`. It consumes
final geometry and opaque RGB in caller order. `pdf.Glyph` is an immediate
backend call argument, separate from typography's shared immutable fragment result.

`PaintFragmentGlyphs(result, glyphRange, x, y)` consumes a half-open range of the
result's glyph table and adds the caller's explicit translation once. It uses
captured colors, GIDs, origins, sized instances and `UnitsToBP`; no source text,
ordering, metrics, scale-service query or outline extraction occurs. It binds
only visible selected fonts and retains neither the result nor upstream services.
Fully transparent glyphs are skipped, partial alpha is rejected, and the direct
glyph/rectangle primitives continue to require opaque paint.

Ranges let the caller submit text around `AtomicAnchor.BeforeGlyph` boundaries.
The adapter does not inspect/draw anchors, child boxes or line allocation. It
infers no baseline/frame offset and defines no atomic recursion, page selection,
CSS paint ordering or display-list structure.

The adapter owns the private Folio model, used-glyph sets and page streams.
It releases no borrowed upstream service and needs none after successful font
binding. A `Font` value is sized and document-specific; a foreign-document or
zero binding is rejected. Neither arbitrary face-interface equality nor a
bare GID identifies a font. Separate documents share no mutable embedding state.
Within a document, all operations are serial; copying a used document is invalid.

Full embedding trades PDF size for a direct correspondence to the retained font
program. Folio still builds a temporary subset internally before the callback
replaces its payload. Avoiding that wasted work and supporting valid glyph-only
subsetting are local adapter/backend follow-ups, not upstream text requirements.

Each paint call validates completely before changing page streams or font usage.
Fragment submissions stage new font programs too: a late binding, paint or GID
failure consumes no resource names and leaves document bindings unchanged.
There is no whole-document transaction or rollback. Serialization builds a fresh
object graph each time and permits retries and later painting. The caller owns
the output writer; an I/O error may leave partial bytes and requires a fresh
destination for retry. The proof command serializes to memory before opening
its output file. These rules remain separate from fragment construction errors.

The only paint primitives in this step are glyphs and opaque filled rectangles.
The adapter has no tree traversal, atomic callback, style resolution or general
graphics-state API. Those boundaries remain open.

## Semantic text and richer output are deferred

The spike does not require original-text retention, cluster-based
`/ActualText`, complete `/ToUnicode` semantics, extraction/search/copy
guarantees or tagged accessibility output. Incidental backend metadata is not
a claim of semantic correctness.

The rendering payload consists of positioned glyphs and drawing paint, with
fonts/resources connected locally. Original source data remains upstream only
where existing preparation, pagination or paint construction needs it.

Images, rich compositing, advanced font formats and broad PDF conformance
profiles do not drive this first contract. Unsupported cases should be handled
under an explicit accepted profile; the initial test document can stay simple.

## Evidence and remaining integration work

[`pdf` tests](../pdf/document_test.go) inspect final page streams for exact
origins, GIDs, sizes and colors; verify resource sharing, byte-for-byte original
font embedding and composite outlines; and exercise invalid batches and write retries.
[Font tests](../pdf/font_test.go) check capability/identity/scale failures,
distinct provenance with shared bytes, and separate document embedding state.
[Fragment tests](../pdf/fragment_test.go) exercise range translation/reuse, real
RTL/ligatures/SHY suffixes, explicit atomic interleaving, transparent no-ops,
late failure/retry, captured-scale use and collection of upstream services.

[`folio-proof`](../cmd/folio-proof/main.go) produces a two-page proof through
paragraph preparation/layout, `BuildFragmentResult` and `PaintFragmentGlyphs`, without outline
extraction. Both pages were rendered with Poppler and inspected; `qpdf --check`
reported no syntax or stream-encoding errors. This demonstrates the bottom
boundary with fixed rows, not the full page-layout rendering spike.

Verify that two selected pages contain the expected glyphs, positions and
foreground/box paint; that selected atomic interiors are composed once; that
the embedded program matches the upstream GIDs; and that repeated font use
has a coherent document resource lifetime.

Use RTL marks, a ligature and spacing that differs from nominal advances to
check placement authority. Keep failures in font pairing, resources and output
distinct from completed glyph-result construction. Source text extraction is
not part of this acceptance check.

See the [pipeline handoff](docrender_rendering_pipeline_folio_handoff.md),
[fragment API](immutable_fragment_result_api_proposal.md) and
[page boundary inventory](pagelayout_painter_boundary_inventory.md) for the
other responsibilities. Selected page composition remains a planning boundary;
the completed connector does not decide CSS traversal ownership or a display list.
