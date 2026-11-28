# Minimal box painting over SelectedBox

**Status:** Neutral traversal implemented for the two-page PDF spike,
28 November 2026; the typography join remains proposed.
Uses the implemented [SelectedBox API](selected_box_subtree_api_proposal.md)
and follows [REMARKS.md](REMARKS.md). No typography bridge, PDF adapter,
display-list representation or semantic-text requirement is introduced.

The implemented algorithm has two box-level passes within a receiving frame: ordinary
block decoration, then paragraph contents. Inline-block interiors enter only at
their existing paragraph anchors, and apply the same two passes locally. The
style-span, cluster, glyph-order and foreground decisions in `khipu`/`typography`
remain authoritative.

The [Go API and package proposal](box_paint_traversal_api_proposal.md) sketches
`pagelayout/paint` for this fixed traversal and `pagelayout/paint/typography` for
the paragraph/atomic join. The neutral package is implemented; the typed join
remains proposed. No display-list representation is selected.

## Working scope and frame attachment

Use the current static normal-flow content profile, including anonymous blocks,
flow-root containers and inline-block interiors. The fixture uses transparent or
opaque solid backgrounds and solid borders. Ordinary inline backgrounds, richer
effects and general authored stacking levels remain outside this painting spike;
this does not narrow the loader's existing admission rules.

The neutral traversal implements these attachment rules:

| Boundary | Rule for the spike |
| --- | --- |
| Page | Paint the template page background once, using `Page.Info()`. |
| Frames | Visit `Page.Frames()` in catalog order. Finish each frame's decoration and contents before the next frame. Include unused frames' decoration. |
| Frame decoration | Paint its solid background across `Frame.BorderBox`, then its effective borders. `Frame.ContentBox` supplies the origin for attached content. |
| Content attachment | Treat `PageFrame.Fragments()` as ordered roots in that frame's normal-flow contents. Use their composed `SelectedRoot()` handles; skip invalid roots. |
| Source styling | Keep original content styles. Attachment creates no new CSS ancestry, cascade or inherited frame paint. |
| Content root background | Paint a selected source root, including a body root, on its selected box rectangle. Do not propagate it to the template page background. |
| Overflow | Add no implicit frame or content-box clip. If content ink crosses into a later frame, the later frame's painting follows it in catalog order. The physical output page remains the outer boundary. |

Finishing each frame as one painting unit is the spike's attachment policy for
the separate template/content documents. It is consistent with fixed boxes
forming stacking contexts in
[CSS Positioned Layout](https://www.w3.org/TR/css-position-3/#painting-order),
but the cross-document attachment and source-root background rules are our
explicit print convention. No extra context ID or retained template CSS is needed
for this restricted case.

The roots within a frame share the same ordinary-block phases. A routed branch
root, `EstablishesBFC`, or a composition wrapper does not by itself start another
paint scope. This matters when margins or ink make neighboring material overlap.

## Two passes over ordinary blocks

CSS separates in-flow block backgrounds/borders from the later inline-content
phase, and treats inline-block interiors as local painting units with exceptions
for positioned descendants. The current content profile excludes those positioned
cases. This motivates the following reduced box algorithm while preserving our
agreed paragraph behavior.
[CSS 2.2, Appendix E](https://www.w3.org/TR/CSS22/zindex.html#painting-order)

1. **Decoration pass:** visit all selected roots in frame order and walk ordinary
   block containers in structural preorder. For each `Container()`, paint its
   background, then borders, using the selected geometry and effective edges.
   At a `Paragraph()` node, stop this pass: its atomic children are not ordinary
   block descendants in this phase.
2. **Content pass:** visit the same roots and ordinary container children in
   structural order. At a paragraph group, use its selected `FitParagraph` and
   existing completed horizontal output. The paragraph's atomic entry points
   handle the group's inline-block children.

Anonymous containers participate structurally even when their paint is empty.
Transparent backgrounds and absent borders produce no drawing, but do not prune
children. A zero-glyph paragraph or empty atomic likewise remains meaningful.

The phase boundary spans **all roots in a frame**. For adjacent ordinary blocks
A and B, this yields A decoration, B decoration, A paragraph, B paragraph.
It avoids allowing B's background to cover A's text merely because a recursive
walker finished A's entire subtree first. No new glyph ordering is involved.

The following is control-flow notation, not proposed function signatures or
stored operations:

```text
page
    template page background
    for each frame in catalog order
        frame background and borders
        decoration pass over its ordered selected roots
        content pass over those roots

decoration pass at ordinary container
    its background and borders
    decoration pass over children, stopping at paragraph groups

content pass at ordinary container
    content pass over children

content pass at paragraph group
    existing paragraph traversal
        at an atomic anchor: enter that selected inline-block

enter inline-block
    its background and borders
    decoration pass over its ordinary interior children
    content pass over its interior children
```

Nested inline-blocks repeat the last step at their own paragraph anchors. Their
decorations do not leak into the enclosing frame's first pass.

## Paragraph and atomic join

The typography-aware bridge uses the existing `InspectFragment(fragment)` bundle.
`SelectedBox.Paragraph().ID` identifies the matching typed group. The bridge obtains
that occurrence's `FragmentResult` through the existing `BuildFragmentResult` path
(or reuses one already built); typed inspection itself returns the underlying
paragraph fragment, not this result. The result supplies the agreed glyph sequence
and atomic boundaries without outline extraction or reshaping.

At an anchor, qualify its paragraph-local ID by the parent paragraph occurrence.
Use that group's existing `Input.Occurrences` to find the source Box, then
`fragment.Selected(box)` to reach the selected inline-block. It must be one of
this paragraph group's immediate selected children. A missing or mismatched join
is a boundary error; it is not permission to paint an unselected source subtree.

Replay glyphs up to the existing anchor boundary, paint that atomic interior, then
continue the same paragraph output. `BeforeGlyph` boundaries, including line-end
boundaries, and equal-boundary anchors retain their existing order. Neither
`Children()` order nor X coordinates supplies a replacement horizontal order.
There is no sorting, source-text reconstruction or new glyph-to-box association.

An inline-block containing an explicit paragraph has this inspection shape:

```text
outer paragraph group
    inline-block A
        inner paragraph-owning block B
            inner paragraph group H
```

On entry, paint A's decoration, then B's decoration, then H's completed paragraph.
H retains `Owner=B` and `ParentAtomic=A`. For multiple inner blocks, complete their
decoration pass before their paragraph pass. Nested atomics enter through H's
existing anchors. Empty interiors still receive their applicable box decoration.

The content pass does not additionally walk a paragraph group's `Children()` after
replaying the paragraph. Likewise, `bundle.Groups()` is a lookup inventory, not a
second paint loop over all outer and inner paragraphs. Each selected interior is
reached through its parent anchor once. This joining rule reuses the existing
paragraph decisions; it does not revisit strict CSS ordering inside paragraphs.

## Coordinates and continuation

For the spike, the traversal uses pagelayout's published composed CSS-pixel placement
as the box/paragraph coordinate authority. Keep typography glyph offsets in their
existing local big points. This accepts the current CSS `float32` placement
precision; preserving additional local-anchor precision can be separate work.

| Material | Page placement |
| --- | --- |
| Frame decoration | Already page-local `BorderBox`; do not add `ContentBox`. |
| Selected container | Resolve `Geometry.Frame` through the `RelativeTo` chain of parent **content** origins within its fragment; add the receiving frame's `ContentBox` origin once. |
| Paragraph glyph | `0.75 × (frame content origin + FitParagraph.X/Y) + local glyph origin`, in big points. |
| Atomic interior | Use its own selected container geometry and each inner paragraph's published `X/Y`, under the same receiving frame. The parent anchor determines entry, not another translation to add. |

The inspection parent and coordinate parent can differ. Carry the frame descriptor
and matching composed fragment alongside the selected handle; do not derive
coordinates by summing inspection ancestors. Raw BoxIDs are scoped to that
fragment's source tree.

A container continued onto page two contributes one selected decoration occurrence
per page. Use each occurrence's effective edges: the first retains its opening
edges, the last its closing edges, with side edges throughout. Its paragraphs are
the selected occurrences for that page. Complete atomic interiors keep their own
full edges even beneath a continued ancestor. Nothing is carried forward as a
page-height offset or repainted from the original whole paragraph.

## Responsibilities and next checks

`pagelayout` remains the working owner of the required box/context ordering
algorithm. A typography-aware bridge joins paragraph output and atomic children
without adding typography imports to `content`, template or root orchestration.
`docrender` handles backend paint and font resources. The
[API proposal](box_paint_traversal_api_proposal.md) places these responsibilities
in the implemented neutral package and proposed typed bridge, with synchronous
output/paragraph callbacks.
There is no pluggable paint-order policy or selected output representation.

Existing snapshots supply the required inputs. The neutral traversal tests use
publicly constructed frozen pages and a fixture backend to check:

- A decorated container continues across two pages with correct fragment edges.
- An inline-block with its own paragraph paints its interior exactly once, at its
  existing anchor; nested and glyph-free interiors follow the same rule.
- Two ordinary blocks, including separate roots in one frame, follow the shared
  decoration/content phases when their painted areas overlap.
- An unused decorated frame still paints, catalog order governs frame callbacks,
  and signed child coordinates remain intact.

These checks are executable in [paint tests](../../pagelayout/paint/paint_test.go),
alongside ownership, atomic-entry, cancellation and callback-failure coverage.
They do not render glyphs or verify visual ink overlap. The existing paragraph
regression cases remain unchanged. Semantic text, display-list design and general
stacking/effect support stay deferred.

Sources: [selected inspection](../../pagelayout/content/selected.go),
[page/frame output](../../pagelayout/page.go),
[frame descriptors](../../pagelayout/template/catalog.go),
[composed coordinates](../../pagelayout/content/compose.go),
[typed paragraphs](../../pagelayout/preparation/typography/fit.go),
[atomic occurrence mapping](../../pagelayout/content/extract.go) and
[glyph/anchor results](../../typography/render/result.go).
