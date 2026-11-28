# Minimal retained box-paint context

**Status:** Planning proposal, 28 November 2026. Follows
[REMARKS.md](REMARKS.md), the [pipeline trace](css_paint_context_trace.md) and the
agreed relaxed paragraph painting rules. Selected-box inspection is implemented;
paint-order implementation and display-list design remain open. Semantic text
output remains deferred.

The proposed boundary is a read-only view of selected box subtrees over the existing
page, content and fitted-fragment snapshots. It supplies box relationships,
decoration, context membership where applicable, and contained paragraph output.
Paragraphs retain their existing horizontal glyph/atomic traversal and resolved
paint. No additional glyph-level ordering or source-element association is required.

The [box-subtree walkthrough](box_subtree_paint_walkthrough.md) illustrates this
scope. `pagelayout` manages the required ordering algorithm; its exact package
placement remains open, without a pluggable paint-order interface.

The [selected-box Go API](selected_box_subtree_api_proposal.md) supplies a small
fragment-scoped inspection handle over these snapshots. It adds no glyph-level
requirements or paint-order algorithm.

The [minimal box traversal sketch](box_paint_traversal_sketch.md) proposes the
next step: frame attachment, ordinary-block decoration/content phases, and entry
into atomic interiors through existing paragraph anchors. These are planning
rules; no traversal implementation or display-list representation is chosen.

## Information to expose

| Information | Minimum content | Reuse and remaining work |
| --- | --- | --- |
| Receiver attachment | Selected page/frame identity and order; ordered branch-fragment placements; each selected root's receiver. | Reuse `Page`, `PageFrame` and their fragments. Define shared receiver context/overflow rules rather than copying template CSS. |
| Selected box relationships | Qualified box occurrence, structural parent/ordered children, and relevant box role. | `SelectedRoot`, `Selected` and `SelectedBox.Children` expose the selected projection using existing snapshots and `Box.Info()`. Carry the parent path during inspection; no second tree is needed. |
| Paint-context participation | Enclosing context, whether a box establishes one, and its order/stack level where supported. | Derive current facts from box roles and the admitted profile once receiver policy is defined. General `z-index` and effect support remain deferred; no speculative per-box fields are required now. |
| Decoration and placement | Resolved box paint, this occurrence's geometry/coordinate parent and effective fragment edges. | Reuse `FitContainer`, geometry, used values and source paint, including consumed atomic interiors. |
| Contained paragraph output | Selected paragraph occurrence, owning box/group and resolved placement. | Reuse `FragmentID`, `FitParagraph` and typed inspection. Replay the completed horizontal output under the existing relaxed rules. |
| Inline child subtree attachment | Selected parent paragraph/local atomic ID, corresponding box occurrence and selected interior. | Reuse `GroupInput.Occurrences`, `ParentAtomic` and atomic anchors. The interior includes box decoration even when it contains no glyphs. |

These are observable facts, not proposed record types or separate stores. Context
and structural relationships need not be duplicated when one can be derived from
the other for the supported profile. The algorithm determines decoration/content
phases; no final paint index or phase must be stored on every box.

Sources: [content snapshots](../../pagelayout/content/document.go),
[selected fragments](../../pagelayout/content/fit.go),
[atomic references](../../pagelayout/content/extract.go),
[typed inspection](../../pagelayout/preparation/typography/fit.go),
[page output](../../pagelayout/page.go) and
[glyph results](../../typography/render/result.go).

## Snapshot reuse and occurrence identity

A `content.Box` already identifies its snapshot, tree and normalized box and keeps
that snapshot alive. Its selected placement additionally belongs to a particular
page/frame/branch-fragment occurrence. A container continued onto another page
reuses the same source box with different selected geometry and edges. Paragraph
`FragmentID`s already distinguish paragraph occurrences. No new global ID scheme
or identity based on addresses of copied inspection values is required here.

The selected view combines normalized order with fitted containers, geometry and
paragraph membership. Geometry maps alone do not identify all structural ancestors
or paragraph relationships. Keep the ancestors needed to order selected subtrees;
exclude unselected suffixes. Ordinary inline source boundaries inside settled
paragraph output do not each require a paint record.

Structural parentage, coordinate parentage and stacking-context membership remain
distinct. A block formatting context is not automatically a stacking context.
`RelativeTo` supplies a coordinate origin, while `BoxInfo.Context` supplies source
style context. Neither is a substitute for the supported paint-context rules.

## Paragraph and atomic boundary

Completed paragraph output is the horizontal painting authority for the spike.
`khipu` and `typography` already embody the agreed treatment of style spans,
glyph clusters and ordering. The renderer consumes those decisions without
reopening them; practical differences from strict CSS inline painting are accepted.
Existing glyph order, positions, resolved foreground and atomic placement remain
usable as published. RTL, combining marks and ligatures do not introduce additional
CSS scope labels, logical glyph ordinals or source-to-glyph maps. A cross-element
ligature keeps its existing single-glyph, resolved-color behavior.

Atomic anchors remain useful for finding a selected inline box and its local
placement within that horizontal material. Existing physical traversal boundaries
can support this join; there is no new requirement to reconstruct strict CSS
logical ordering inside the paragraph.

The bridge qualifies the local atomic ID by its selected parent paragraph, joins
the corresponding box and selected interior, and reaches that interior once.
Interior paragraph output has its own occurrence identity. Empty atomics may still
have decoration. The original group's occurrence list can contain suffix atomics,
so only selected attachments participate.

Composition retains one coordinate authority. Child paragraph origins already
include receiver composition; adding the local anchor to them again is incorrect.
The existing precision choice between local float64 anchors and CSS float32 origins
remains a placement question, independent of glyph-level CSS ordering.

## Read-only access and ownership

The ordering stage needs to inspect:

1. Receivers and their selected roots in defined order, including empty receivers.
2. Selected box relationships, context participation and decoration/geometry.
3. Contained paragraph occurrences and their existing completed output.
4. The selected inline child subtree attached to a paragraph's atomic occurrence.

The selected-box API exposes immutable handles and copied child slices; this
does not choose an execution or display-list representation. No mutable tree nodes
escape; inspection performs no cascade, shaping, fitting or backend painting.

The content snapshot remains authoritative for structure and source paint; fitted
fragments for selected geometry/edges; paragraph results for horizontal output.
The typed adapter keeps typography dependencies out of root/content/template.
No new glyph-result construction handshake or CSS association payload is required.

Holding Box handles also retains existing source text privately. Semantic text
remains deferred as an output requirement; this proposal does not require stripping
current snapshots or exporting their text. Existing borrowed-font lifetime
obligations remain unchanged. Painting needs no live preparation session or fitter.

## Remaining box-level decisions

- Use selected subtree access for continuing containers and all consumed atomic
  interiors. Each selected decoration and paragraph output
  contributes once; traversing ancestor structure does not duplicate that output.
- Define page/frame paint contexts, content attachment and overflow behavior for
  the flat template profile, including template-page versus content-root paint.
- Specify the supported box-decoration/content phases while preserving the agreed
  paragraph behavior. Context hierarchy, structural order and coordinate hierarchy
  must not be conflated.
- Keep ordinary inline backgrounds and richer effects outside the simple painting
  target. Their future geometry or CSS rules are not prerequisites for this spike.
- Retain additional stacking/effect facts at the original style boundary only when
  those features are admitted. The current profile does not require general z-index.

These questions concern box subtrees and their placement in painting contexts.
No paint-order implementation or backend operation representation is chosen here.
