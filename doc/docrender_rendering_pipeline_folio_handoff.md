# docrender: rendering responsibilities for the two-page spike

**Status:** Planning handoff, revised 28 November 2026 under
[REMARKS.md](REMARKS.md). The target is a straightforward two-page PDF with
simple paint. The bottom Folio adapter is now implemented in
[`docrender/pdf`](../pdf); the upper boundaries remain proposals. A finalized
pipeline package layout and display-list design are still open.

## Objective

Establish a sound division between completed typesetting, glyph traversal, page
paint traversal and PDF output. Use a small print example to exercise those
boundaries. Full browser painting and high-quality semantic PDF output are later
work, not prerequisites for this spike.

The pipeline under discussion is:

```text
completed pagelayout pages and selected paragraph fragments
    ├─ reusable typography traversal → immutable local glyph/color/anchor results
    └─ retained page/box geometry and resolved paint
                         ↓
              page occurrence / atomic bindings
                         ↓
        encapsulated supported paint traversal
                  (module owner open)
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

## Responsibilities

| Responsibility | Owner or placement |
| --- | --- |
| Shaping, line breaking and local geometry | Existing typography/khipu and layout stages. |
| Pagination, selected occurrences and receiving frames | `pagelayout`. |
| Shared retained glyph traversal | Proposed in `typography/render`, reusing current placement and paint-mapping algorithms. |
| Paragraph/page association and atomic child composition | Bridge between page output and rendering inputs. |
| Supported CSS paint traversal | A separate algorithm with module ownership still open. |
| Folio-specific fonts, resources, encoding and serialization | `docrender`. |
| Diagnostic SVG and pipeline experiments | Existing proof consumers and `tytui/tycli`. |

CSS paint traversal must not be hidden inside the PDF backend or inferred from
glyph traversal. Equally, describing a painter responsibility does not assign it
to a `docrender/paint` package or require a new module now.

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
Ordinary inline backgrounds lack published per-line rectangle geometry today;
either exclude them from that fixture or define the needed geometry separately.
They do not justify approximating paint by glyph ink bounds.

Transforms, stacking-level extensions, group opacity/blending, images, rich
backgrounds, decorations, outlines, rounded borders and generated content do not
drive the initial contract. The accepted profile can favor sensible print
defaults. Any relaxation should be explicit for admitted input; it is not a
claim of general HTML/CSS fidelity.

## Paint traversal remains an architectural question

Specify the algorithm's inputs and ordering obligations before selecting its
module. It needs resolved page/container paint, selected paragraph occurrences,
atomic bindings and the relevant structural relationships for the supported
normal-flow profile.

Important cases include container backgrounds relative to text, borders,
included atomic interiors, and overlap caused by signed margins or glyph
overhang. A simple profile still needs deterministic order. Go map iteration,
numeric BoxID sorting and drawing every rectangle before every glyph do not
establish that contract.

Keep ordering logic independently testable from Folio. Its placement may be in
an existing module or a separate component once dependency direction and input
ownership are clear. This handoff makes no choice among those homes.

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
two-page proof. The other steps still require their own design and integration;
the proof command's row placement is not a shared traversal implementation.

1. Choose a simple two-page fixture and document its supported print paint.
2. Finalize the small shared glyph-result boundary and construction-time color
   resolution, preserving borrowed-font lifetime.
3. Establish selected page/atomic bindings and their coordinate authority.
4. Describe and test the supported paint-order algorithm; choose its module only
   after its dependencies are understood.
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
