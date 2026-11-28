# docrender: rendering responsibilities for the two-page spike

**Status:** Planning handoff, revised 28 November 2026 under
[REMARKS.md](REMARKS.md). The target is a straightforward two-page PDF with
simple paint. The bottom Folio adapter is now implemented in
[`docrender/pdf`](../pdf), along with the slim fragment result in
[`typography/render`](../../typography/render/result.go). `Page.PaintFragmentGlyphs`
now connects them with explicit glyph ranges and translations. Page composition,
pipeline package layout and display-list design are still open.

## Objective

Establish a sound division between completed typesetting, glyph traversal, page
paint traversal and PDF output. Use a small print example to exercise those
boundaries. Full browser painting and high-quality semantic PDF output are later
work, not prerequisites for this spike.

CSS planning focuses on box subtrees, stacking contexts and box decoration around
contained paragraph output. Preserve the agreed relaxed paragraph painting rules:
existing horizontal glyph/atomic traversal, settled positions and resolved colors.
No glyph-level CSS ownership, logical reordering or source-element association is
required by the page paint boundary.

The pipeline under discussion is:

```text
completed pagelayout pages and selected paragraph fragments
    ├─ reusable typography traversal → immutable local glyph/color/anchor results
    └─ retained page/box geometry and resolved paint
                         ↓
              page occurrence / atomic bindings
                         ↓
        required supported CSS paint traversal
          (managed by pagelayout; package open)
                         ↓
            backend-neutral painting boundary
             (display-list design open)
                         ↓
      docrender PDF adapter and font/resource pairing
                         ↓
                        PDF
```

The chosen shaper remains authoritative for glyph selection and shaping
advances/offsets. Layout supplies final spacing, baselines and page placement.
The output backend serializes those results without changing typography.

The interplay of `khipu` and `typography` already handles the agreed style-span,
glyph-cluster and horizontal ordering behavior. The renderer must not reopen
those decisions. Differences from strict CSS inline painting are accepted for
the current practical scope and do not create new renderer requirements.

## Responsibilities

| Responsibility | Owner or placement |
| --- | --- |
| Shaping, line breaking and local geometry | Existing typography/khipu and layout stages. |
| Pagination, selected occurrences and receiving frames | `pagelayout`. |
| Shared retained glyph traversal | Implemented in `typography/render`, sharing placement and paint-mapping algorithms with outline proofs. |
| Paragraph/page association and atomic child composition | Bridge between page output and rendering inputs. |
| Supported CSS box/context paint traversal | Required algorithm managed by `pagelayout`; proposed placement in `pagelayout/paint` with a separate `paint/typography` bridge. Paragraph painting keeps its existing rules. |
| Folio-specific fonts, resources, encoding and serialization | `docrender`. |
| Diagnostic SVG and pipeline experiments | Existing proof consumers and `tytui/tycli`. |

CSS paint traversal stays outside `docrender` and is not inferred from glyph
traversal. It is an integral pipeline requirement; no pluggable paint-order
interface or new module is implied.

## Current page boundary

Pagelayout publishes selected pages, frames, structural fitted fragments and
typed paragraph occurrences. It does not publish the complete page render tree
assumed by the earliest handoff. Use the
[boundary inventory](pagelayout_painter_boundary_inventory.md) for available
geometry, paint and occurrence facts.

Current typography fragments retain enough shaped material to derive final glyph
origins. Shared rendering traversal performs that derivation once, without
reshaping, measuring, breaking or restacking. The completed
[fragment result](immutable_fragment_result_api_proposal.md) exposes GIDs, fonts,
final origins, resolved foreground colors, logical lines and atomic anchors.

Source text, direction, replacement spellings and cluster associations are
temporary construction facts where required for ordering or paint. They do
not become renderer output obligations. Existing source information used by
pagination and source paint remains in its current stages.

## First print profile

Start with positioned text in existing page-template frames and the simple
solid paint needed by the example. Keep the typography cases meaningful:
font changes, RTL/marks, ligatures, selected replacements and atomic placement
must retain their existing geometry and supported foreground behavior.

A concrete fixture should declare the small box-paint subset it exercises.
Ordinary inline backgrounds are outside this simple painting target; their
per-line geometry remains deferred. They do not justify new glyph-level paint
associations or approximating backgrounds by glyph ink bounds.

Transforms, stacking-level extensions, group opacity/blending, images, rich
backgrounds, decorations, outlines, rounded borders and generated content do not
drive the initial contract. The accepted profile can favor sensible print
defaults. Any relaxation should be explicit for admitted input; it is not a
claim of general HTML/CSS fidelity.

## Required paint traversal and retained context

The working owner is `pagelayout`, following the last section of REMARKS. Specify
the algorithm's inputs and ordering obligations before fixing its package and
boundary details. It needs selected box subtrees, resolved decoration, contained
paragraph occurrences, atomic child bindings and the supported context rules.

Important cases include decorated blocks containing paragraphs, selected inline
box interiors and page/frame attachment. A stacking context occupies a place in
its parent context; it is not a global BoxID or z-index sort. General authored
stacking levels remain deferred. Existing horizontal painting does not need to
be reopened to establish these box-level responsibilities.

The [CSS paint-context trace](css_paint_context_trace.md) inventories what survives
styling, normalization, fitting and template attachment. Private normalized trees
and source ancestry survive; [selected subtree access](selected_box_subtree_api_proposal.md)
is implemented, while receiver paint-context policy still needs a contract.
The [box-subtree walkthrough](box_subtree_paint_walkthrough.md)
illustrates the scope. Keep ordering logic independently testable from Folio and
preserve the existing typography adapter dependency boundary.

The [box traversal sketch](box_paint_traversal_sketch.md) proposes concrete rules
for the spike: finish frames in catalog order, use shared decoration/content
phases for their ordinary blocks, and enter inline-block interiors at existing
paragraph anchors. It also proposes using the published composed CSS origins for
placement. This remains a planning proposal without implementation or display-list
decisions.

The [traversal API proposal](box_paint_traversal_api_proposal.md) sketches the
package split and a concrete `WalkPage` entry point with synchronous decoration
and paragraph callbacks. The typed bridge uses existing glyph results and atomic
anchors; it adds no further glyph-order policy or display-list design.

The [PDF decoration adapter sketch](pdf_decoration_adapter_sketch.md) proposes
`docrender/adapter/pagelayout` to translate page/frame/box decoration callbacks to
the existing PDF rectangle primitive. Its first profile uses opaque or transparent
backgrounds and common-color square borders with effective per-side widths.
The proposed restriction is local to that adapter, not to upstream loading.

The eventual painting boundary stays backend-neutral. Flat/nested display-list
representation, operation types, graphics-state encoding and replay interfaces
remain open.

## Geometry and atomic composition

Typography results use local float64 big points, Y downward. Page/frame and
group origins currently use CSS pixels. For frame content origin F, group
origin G relative to the receiver, and local glyph origin q in big points:

```text
page origin in big points = 0.75 × (F + G) + q
```

Do not rescale q or add already-composed ancestor edges again. Logical allocation
is not an instruction to clip glyph overhang.

The bridge qualifies atomic IDs by paragraph occurrence and retains geometry
from the matching selected child layout. It chooses one authoritative path
between precise local-anchor composition and already-composed child origins,
including their precision contract. It adds content-edge offsets once and
visits each included interior once. The glyph result owns no child tree or
drawing callback.

## Font integration and PDF output

The [Folio boundary brief](folio_pdf_backend_integration_brief.md) follows an
isolated backend approach. `docrender` associates each retained upstream font
with the same immutable font binary and Folio's private font model. A second
parse is acceptable. Avoiding that parse is not an upstream API requirement.

Folio may need nominal widths, descriptors and other font data for its own
encoding/subsetting machinery. Those requirements remain in `docrender`.
They neither replace chosen glyphs nor determine a new pen trajectory.
The backend must reproduce the authoritative final origins; explicit glyph
placement is sufficient for the first experiment.

The [pinned backend audit](folio_pdf_backend_integration_brief.md#pinned-backend-audit-and-local-adaptation)
and first adapter establish glyph-only registration, resource publication and
explicit positioned output against Folio v0.11.0. They introduce no original
Unicode strings into the backend boundary. Backend-specific resource names and
mutable caches do not enter generic results.

## Deferred semantic text

The rendering result does not retain original paragraph strings, decoded HTML
text, replacement strings, cluster/source graphs or reading-order metadata.
Text extraction, search/copy guarantees, accessibility/tagging and semantic
`/ActualText` or `/ToUnicode` design are deferred.

Font machinery may create incidental metadata, but it does not set semantic
acceptance criteria or require a retained text subsystem. Do not invent Unicode
from GIDs. Later semantic output needs an explicit, separately scoped contract.

## Planning sequence and evidence

Implementation proceeds bottom up: step 5 below has a working adapter and fixed
two-page proof, and step 2 has a tested typography result sharing traversal with
outline proofs. The proof submits those results directly to the PDF adapter.
Connecting these to selected page occurrences remains separate;
the proof command's row placement is not a page paint algorithm.

1. Choose a simple two-page fixture and document its supported print paint.
2. **Fragment result implemented:** capture local glyph/anchor geometry and solid
   color, preserving borrowed-font lifetime and releasing construction inputs.
   **PDF connector implemented:** submit explicit glyph ranges/translations,
   stage resource bindings, and release result/font services after submission.
3. **Selected-box inspection implemented:** expose selected containers, paragraph
   groups and atomic interiors over existing snapshots. Establish page/atomic
   rendering bindings and their coordinate authority.
4. Refine the proposed `pagelayout/paint` traversal and `paint/typography` bridge,
   preserving existing paragraph painting. The API and package split remain
   planning proposals.
5. **First adapter implemented:** verify local Folio font pairing, glyph
   registration and exact positioned output; extend checks as integration grows.
6. Compare the produced page geometry and drawing behavior with retained
   typography/proof evidence, without requiring semantic PDF extraction.

Construction errors publish no fragment result; later resource/output failures
need their own complete-output and retry handling. A successful page callback
alone does not mean a finished PDF.

Useful repository evidence:
[page delivery](../../pagelayout/shipout.go),
[typed fragments](../../pagelayout/preparation/typography/fit.go),
[shared traversal](../../typography/render/paragraph.go),
[fragment proof cases](../../tytui/documentlayout/proof_fragment_test.go).
