# Minimal traversal API and package placement

**Status:** Planning proposal, 28 November 2026. Builds on the
[box traversal sketch](box_paint_traversal_sketch.md) and the implemented
[SelectedBox API](selected_box_subtree_api_proposal.md), following
[REMARKS.md](REMARKS.md). All declarations below are proposed, not implemented.
Paragraph painting retains the existing `khipu`/`typography` behavior. Semantic
text and display-list design remain deferred.

## Package placement

| Package | Responsibility |
| --- | --- |
| `pagelayout/paint` — new | The fixed page/frame and box-phase traversal over selected snapshots; coordinate resolution; entry into selected atomic interiors. No typography dependency. |
| `pagelayout/paint/typography` — new | Connect existing typed paragraph inspection and `FragmentResult` construction to that traversal; interleave glyph ranges and atomic interiors using existing anchors. |
| `pagelayout/preparation/typography` — existing | Continue to supply preparation, fitting and typed frozen inspection. No new rendering dependency. |
| `docrender` — existing | Backend drawing, PDF page/document lifecycle, and local font/resource pairing. |

`paint` imports root `pagelayout` for frozen `Page` values, plus `content`,
`template` and core geometry types. Root `pagelayout`, `content` and `template`
do not import `paint` or its typography bridge. The new bridge imports `paint`,
`preparation/typography` and `typography/render`. Neither new package imports
`docrender`, Folio or concrete font adapters.

This keeps the graph acyclic and respects the existing
[dependency checks](../../pagelayout/preparation/typography/dependencies_test.go),
which also keep `typography/render` out of the preparation adapter itself.
The caller wires a selected page to the bridge through the existing `Shipout`
consumer; pagination does not acquire a rendering dependency.

## Neutral box traversal

The public surface can be two small argument types and one concrete function:

```go
package paint // github.com/npillmayer/pagelayout/paint

import (
    "context"

    "github.com/npillmayer/css-box-layout/layout"
    "github.com/npillmayer/pagelayout"
    "github.com/npillmayer/pagelayout/content"
    "github.com/npillmayer/pagelayout/template"
)

// Each callback draws the supplied decoration: background, then borders.
// Page currently has only background paint. All rectangles use CSS pixels.
type Decorations struct {
    Page  func(template.PageInfo) error
    Frame func(template.FrameInfo) error
    Box   func(container content.FitContainer, pageBorder layout.Rect) error
}

// One selected paragraph occurrence, with its matching composed fragment.
type Paragraph struct {
    Fragment     *content.FitFragment
    Info         content.FitParagraph
    PageX, PageY float32 // page-local CSS origin, including frame attachment
}

func WalkPage(
    ctx context.Context,
    page *pagelayout.Page,
    decorations Decorations,
    paragraph func(p Paragraph, enterAtomic func(content.Box) error) error,
) error
```

`Decorations` supplies output destinations, not a replaceable ordering algorithm.
`WalkPage` always performs the agreed traversal: page background, frames in catalog
order, frame decoration, ordinary-block decorations across all roots of that
frame, then their paragraph contents. Atomic interiors enter only through the
paragraph callback. No phase selector, custom comparator or `PaintOrder` interface
is proposed.

The decoration callbacks consume complete decoration tasks in the specified local
background-before-border order. They do not traverse descendants. `Box` receives
the existing container record for paint and effective edges, plus its resolved
page-local border rectangle. `container.Geometry` keeps its original
parent-content meaning; the callback uses `pageBorder` for drawing. Page and frame
descriptors already contain page-local dimensions/rectangles. This avoids a new
unified decoration record or changing the existing geometry contracts.

`Paragraph.Info` preserves the selected occurrence ID, group, owner, `ParentAtomic`
and payload-cloning contract. Its original `X/Y` remain receiver-relative;
`PageX/PageY` add the receiving frame origin once. `Fragment` is an independent
wrapper over the same frozen backing, suitable for existing typed inspection.
Changing that wrapper or copied records cannot retarget the walk's own selection.
These arguments describe a call; they are not a new retained paragraph result.

The function follows the spike's proposed attachment rules: selected source-root
paint remains on its own box, frame attachment does not recascade content, and no
implicit frame clip is introduced. An empty frame still has a decoration call;
zero-height or glyph-free selected material is not filtered away by the walker.

## The atomic entry point

`enterAtomic` belongs to one active paragraph invocation. Its argument is the
existing qualified source Box obtained from that paragraph's atomic occurrence
mapping. The walker checks that it is an immediate selected inline-block child
of this paragraph, then paints its decoration and runs the same two phases over
its interior. Nested paragraphs receive their own atomic entry functions.

The function is synchronous and call-scoped: do not retain it, invoke it later,
or call it concurrently. Each selected immediate atomic must be entered exactly
once before the paragraph callback returns success. Unknown, unselected, duplicate
or omitted children are errors. The bridge supplies anchor order; the neutral
walker does not reinterpret horizontal ordering from `Children()` or coordinates.

This narrow entry point keeps the box algorithm inside `paint`. The paragraph
bridge decides only when the existing horizontal output reaches an atomic; it
cannot replace the interior's box-phase algorithm or supply another translation.
No second flat pass over atomic paragraphs follows the parent callback.

## Typography bridge

The public bridge can be one function, reusing the decoration callbacks:

```go
package typography // github.com/npillmayer/pagelayout/paint/typography

import (
    "context"

    "github.com/npillmayer/pagelayout"
    "github.com/npillmayer/pagelayout/paint"
    "github.com/npillmayer/typography/render"
)

func WalkPage(
    ctx context.Context,
    page *pagelayout.Page,
    orderer render.GlyphOrderer,
    decorations paint.Decorations,
    glyphs func(result *render.FragmentResult, span render.IndexRange,
        xBP, yBP float64) error,
) error
```

`orderer` is the existing explicit service required by `BuildFragmentResult`.
Use the pipeline's existing chosen implementation; this adds no CSS glyph-order
policy or fallback. The bridge runs the neutral `paint.WalkPage` with its concrete
paragraph handler. That handler:

1. Uses `preparation/typography.InspectFragment(p.Fragment)` and `Group(p.Info.ID)`
   to obtain the matching typed paragraph, checking its group/owner association.
2. Supplies existing resolved foreground spans/defaults to `BuildFragmentResult`.
   The immutable result remains the agreed outline-free representation. No new
   shaping, line breaking, source-element partitioning or semantic retention occurs.
3. Consumes that result's existing glyph and anchor sequence. It submits each
   intervening glyph range, joins the anchor's local ID through the typed group's
   `Input.Occurrences`, calls `enterAtomic(box)`, then resumes the same result.
4. Preserves line-end anchors, consecutive anchors at the same glyph boundary,
   and glyph-free atomic material. It does not sort anchors by X or by source BoxID.

The supplied glyph callback receives a page-local translation in big points:
`xBP = 0.75 * float64(p.PageX)`, likewise for Y. Glyph origins are already local
big points and include shaping/baseline placement. No further baseline, ancestor
or atomic-anchor offset is added. Inner paragraphs receive their own composed
page origins from the neutral walker.

Typed inspection returns a paragraph fragment, not a prebuilt `FragmentResult`.
Reuse within a walk is an implementation choice; no public cache or result-provider
interface is needed here. Existing source paint/occurrence metadata is temporary
bridge input and does not enter the glyph callback.

The callback signature matches the existing
[`pdf.Page.PaintFragmentGlyphs`](../pdf/fragment.go) method. The
[decoration adapter sketch](pdf_decoration_adapter_sketch.md) proposes a separate
`docrender/adapter/pagelayout` package for the three decoration handlers. A caller
with a created PDF page could wire both paths as follows:

```go
// paintty and pgpdf are the proposed traversal bridge and PDF decoration adapter.
decorations := pgpdf.PDFDecorations(pdfPage)
err := paintty.WalkPage(ctx, selectedPage, orderer,
    decorations, pdfPage.PaintFragmentGlyphs)
```

The same bridge can feed an SVG consumer. The callbacks prescribe the order and
arguments delivered by this traversal; they do not choose whether a consumer
draws immediately or records work. No command collection, display-list node,
graphics-state stack or replay API is specified.

## Lifetime and errors

All calls are serialized and synchronous. Missing context, page, required
callbacks or explicit ordering service fail before the first output callback.
Frozen page/fragment data needs no preparation session. The ordering service is
borrowed only during the walk. Callers keep font resources valid during the walk
and longer if their consumer retains glyph results. Callbacks do not mutate
shared results or borrowed services. Snapshot argument copies may be retained;
the atomic entry function may not.

Cancellation, bad paragraph/atomic associations, result-construction failures and
output errors stop the walk. These can arise after earlier output callbacks.
Preserve the original cause and add the relevant page/frame/box/paragraph context.
A traversal error does not roll back earlier output. Page creation, PDF finalization
and any retry/discard policy stay with the caller and existing delivery handling.

The existing selected-box API needs no additions for this proposal. The two new
packages separate fixed box ordering from the typed paragraph join while leaving
the established immutable glyph result and relaxed paragraph rules intact.

Sources: [selected inspection](../../pagelayout/content/selected.go),
[page/frame snapshots](../../pagelayout/page.go),
[delivery](../../pagelayout/shipout.go),
[typed paragraph inspection](../../pagelayout/preparation/typography/fit.go),
[glyph result construction and anchors](../../typography/render/result.go) and
[backend range submission](../pdf/fragment.go).
