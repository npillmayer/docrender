# PDF decoration adapter for pagelayout

**Status:** Planning proposal for the two-page spike, 28 November 2026. Completes
the output side of the [traversal API proposal](box_paint_traversal_api_proposal.md)
under [REMARKS.md](REMARKS.md). No implementation, module changes or display-list
representation is introduced.

## Package placement and binding

Propose `github.com/npillmayer/docrender/adapter/pagelayout`, imported as `pgpdf`.
It translates pagelayout's resolved decoration records to `docrender/pdf` calls.
The adapter imports the proposed `pagelayout/paint` and existing content/template
types. The PDF backend keeps its existing lower-level API and does not import
pagelayout. Pagelayout does not import docrender.

Adding the adapter would add a pagelayout module dependency to docrender at
implementation time; the package dependency remains confined to this integration
package. The adapter needs no typography bridge or concrete font adapter import.

One exported binding function is enough:

```go
package pagelayout // github.com/npillmayer/docrender/adapter/pagelayout

import (
    "github.com/npillmayer/docrender/pdf"
    "github.com/npillmayer/pagelayout/paint"
)

// PDFDecorations binds the three decoration callbacks to one existing PDF page.
// target must be a page returned by pdf.Document.AddPage.
func PDFDecorations(target *pdf.Page) paint.Decorations
```

The returned value supplies `Page`, `Frame` and `Box` callbacks with the exact
signatures in the traversal proposal. Their implementation can remain private;
no public adapter object or drawing interface is necessary. Binding creates no
page, paints nothing and performs no layout or styling.

The caller creates the correctly sized PDF page and connects both output paths:

```go
// pgpdf and paintty are proposed packages; pdfPage already exists.
decorations := pgpdf.PDFDecorations(pdfPage)
err := paintty.WalkPage(ctx, selectedPage, orderer,
    decorations, pdfPage.PaintFragmentGlyphs)
```

The traversal chooses which decoration to paint and when. The adapter completes
that one decoration synchronously, with background before borders. It never
visits children, calls `enterAtomic`, or starts another page traversal. Glyphs
continue directly through the existing PDF method with unchanged paragraph rules.

## Mapping the three callbacks

| Callback input | Background area | Border geometry and paint |
| --- | --- | --- |
| `Page(template.PageInfo)` | `(0, 0, Width, Height)` from the supplied template page, using `Paint.Background`. | None: the current template page profile has zero effective borders. |
| `Frame(template.FrameInfo)` | The supplied page-local `BorderBox`, using `Paint.Background`. | Effective widths from `Border`; colors/styles from the named `Paint.Borders` sides. |
| `Box(content.FitContainer, pageBorder layout.Rect)` | The supplied page-local `pageBorder`, using `container.Paint.Background`. | Effective widths from `container.Edges.Border`; colors/styles from `container.Paint.Borders`, ordered top/right/bottom/left. |

Frame and box callbacks can share a private decoration routine after normalizing
the two existing border-paint record shapes. This is local argument handling,
with no new public paint record or retained collection.

Background paint covers the selected border rectangle. Margins receive no paint;
padding is already inside that rectangle. Source body/root decoration stays on
its selected box, following the proposed attachment policy. It does not replace
the template page background.

The `Box` callback uses `pageBorder` for drawing. It does not re-resolve
`container.Geometry.RelativeTo`, add `Geometry.Content`, or consult computed
source border widths. `Edges.Border` already reflects continuation: missing
opening/closing edges remain missing. `First`/`Last` need no second clipping or
edge-removal operation in this adapter.

The same mapping covers inline-blocks and their interior containers. Their
complete edges come from `SelectedBox.Container()` even under a split ancestor.
An empty decorated frame or atomic still paints when the traversal invokes it.

## Units and backend calls

All callback geometry and edge widths arrive in CSS pixels, with a top-left
origin and Y downward. Convert each position and extent to big points once:

```text
BP value = float64(CSS-pixel value) × 0.75
```

For an opaque rectangle, the adapter calls the existing
[`pdf.Page.FillRect`](../pdf/document.go):

```go
err := target.FillRect(
    0.75*float64(rect.X), 0.75*float64(rect.Y),
    0.75*float64(rect.W), 0.75*float64(rect.H),
    color,
)
```

`FillRect` already performs the PDF Y-axis conversion. The adapter keeps the
top-left convention and adds no frame, ancestor, baseline or atomic translation.
The traversal supplies resolved page geometry. Negative page-local positions may
be valid; the adapter does not clamp positions or install a receiver clip.

## Small border profile using the current backend

For the first spike, propose square solid borders with one common opaque color
on all positive-width sides. Widths may differ, and any side may have effective
width zero. A wholly transparent border also produces no drawing. This uses
the existing rectangle primitive without a backend extension.

For border rectangle `(x, y, w, h)` and effective widths `t, r, b, l`, one
nonoverlapping decomposition is:

| Band | Rectangle in CSS pixels |
| --- | --- |
| Top | `(x, y, w, t)` |
| Bottom | `(x, y + h - b, w, b)` |
| Left | `(x, y + t, l, h - t - b)` |
| Right | `(x + w - r, y + t, r, h - t - b)` |

Fill the background first, then these nonempty bands. The common border color
makes the allocation of corners between bands visually immaterial. Filling only
the bands leaves a transparent interior's existing paint intact.

This proposal is narrower than the loader's per-side color support. Differently
colored positive-width sides, including mixed opaque/transparent sides, need a
defined corner treatment before extending this rectangle scheme. CSS specifies
constraints on adjoining border color transitions; arbitrary overlap order is
not a general solution. A path/polygon primitive would be a possible later backend
extension, with no required change to traversal callbacks.
[CSS border color transitions](https://www.w3.org/TR/css-backgrounds-3/#corner-transitions)

This is a proposed adapter limit, not a change to content/template admission.
It should be reported as unsupported rather than silently flattening side colors.
The glyph path and our relaxed paragraph painting rules are unaffected.

## Paint, errors and ownership

- Transparent backgrounds are skipped. Opaque backgrounds are filled. Partial
  alpha remains unsupported by the current PDF backend; no implicit blending or
  opacity conversion is introduced.
- Zero-width sides paint nothing, including fragment-omitted sides whose source
  style is still `solid`. `none`/`hidden` already have zero effective widths in
  valid snapshots. Wholly transparent borders are skipped; supported visible
  borders follow the common-color rule above.
- Validate the complete decoration's finite geometry, nonnegative sizes/widths,
  border extents and supported paint before its first fill. The bands require
  `l + r <= w` and `t + b <= h`. Use the effective published geometry rather than
  repairing an inconsistent record by changing layout. Degenerate rectangles
  produce no fill.
- Unsupported paint uses the existing `pdf.ErrUnsupported` category; invalid
  geometry uses `pdf.ErrInvalidInput`. Preserve backend errors and useful frame
  or source-box context for the traversal's error report.

Validating a complete decoration first avoids painting its background before
discovering an unsupported border. Backend failure still stops the traversal;
this proposal makes no whole-page rollback promise. Page creation, serialization
and any retry/discard policy remain with the caller.

The callback closures retain only their target PDF page. They consume copied
decoration records during the call and need no source document, preparation
session, paragraph result or font service. The caller keeps the PDF document and
page valid and uses them serially. The adapter neither closes nor finalizes them.

Useful later checks are a shaded page with an unused bordered frame, an asymmetric
common-color border, a box continued onto page two, and an empty decorated atomic.
They can establish coordinate conversion, preservation of omitted edges and
absence of duplicate drawing without changing paragraph tests. No executable
tests or implementation are added by this sketch.
