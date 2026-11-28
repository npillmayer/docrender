# Reusing retained glyph traversal without outlines

**Status:** Glyph boundary implemented; page bridge planning remains open,
28 November 2026.
[REMARKS.md](REMARKS.md) governs scope. The working public boundary is a complete
immutable result per selected typography paragraph fragment. Semantic text is
deferred. Required CSS paint ordering is to be managed by `pagelayout`; its exact
package placement and display-list structure remain open.

The reusable work belongs near the existing `typography/render` traversal:
select retained lines, walk physical knots, order detached text pieces, accumulate
retained advances, resolve foreground paint and publish local glyph positions.
Outline extraction and PDF resources remain consumer work.

The [implemented API](immutable_fragment_result_api_proposal.md),
[ownership sketch](immutable_fragment_result_sketch.md) and
[walkthrough](immutable_fragment_result_walkthrough.md) develop this boundary.

## Shared traversal and consumer boundaries

| Current code | Responsibility to reuse | Consumer-specific work to separate |
| --- | --- | --- |
| `BuildParagraphProof` / `BuildFragmentProof` | Select completed paragraph or fragment placements with correct original indexes. | Proof record format and outline-bound accumulation. |
| Private `walkRetained` | Start from retained baseline/left skip; traverse physical knots and advance through every final knot width. | General box painting is not performed here. |
| Private `walkGlyphs` and shared `PaintSources` | Use retained font scale, resolve glyph paint from supported source mappings, accumulate shaped advances/offsets after ordering once. | Font identity lookup and outline acquisition. |
| `appendGlyph` | Validate/store final origin and resolved foreground. | Path storage and outline-derived bounds. |
| `GlyphOrderer` / OpenType `VisualPermutation` | Supply physical glyph ordinals without requiring outlines or identity. | Existing proof `Backend` and identity services retain their separate capabilities. |

Sources: [paragraph traversal](../../typography/render/paragraph.go),
[fragment entry point](../../typography/render/fragment.go),
[services](../../typography/render/services.go),
[paint mapping](../../typography/render/paint_sources.go).

## Responsibility boundaries

| Responsibility | Owner or placement |
| --- | --- |
| Glyph selection, shaping advances/offsets | User-chosen shaper through the existing typography/khipu contracts. |
| Breaking, final spacing, baselines and local atomic placement | Existing layout stages; rendering does not repeat them. |
| Reusable selected glyph traversal and temporary paint-source mapping | Implemented in `typography/render`, shared with outline proofs. |
| Source paint projection | Caller/integration supplies resolved foreground spans, separately from shaping styles. |
| Page/paragraph occurrence binding and atomic child geometry | Page bridge; compose selected geometry once. |
| CSS paint traversal | Required algorithm managed by `pagelayout`; exact package placement remains open. |
| Outline proof construction | SVG/proof consumer, using retained fonts and positions. |
| Folio font pairing, resource registration and PDF serialization | `docrender`, isolated from upstream modules. |
| Experiments and visible demos | `tytui`, especially `tycli`; not a compatibility requirement. |

Glyph traversal replays completed typography under the agreed relaxed paragraph
painting rules. Page paint traversal orders box decoration, contained paragraph
output and selected child subtrees within the supported contexts. It introduces
no source-element-to-glyph CSS association or second horizontal ordering pass.

## A smaller retained boundary

The result keeps glyph IDs, exact sized font references, captured scale, final
local origins, resolved foreground colors, logical lines and atomic anchors.
It keeps no paragraph/replacement text, source ranges, cluster table, logical
glyph ordinals or persistent source-to-paint map.

Direction, cluster provenance and scalar anchors remain available during
construction. A permutation or ordinal-bearing scratch record preserves the
association between glyphs, fonts and colors while ordering. Identical GIDs or
anchors cannot establish identity. Knots are already physical; never sort them,
glyphs or atomics by X.

Temporary source/cluster facts support the existing foreground mapping. They do
not become a public semantic result, CSS scope ownership or a new source coverage
requirement.

For nonuniform foreground, reuse the existing ordinary-text and explicit
soft-hyphen paint mapping. Keep byte and scalar domains distinct and report
unsupported mappings when they are needed to choose paint. A single default
color needs no source reconstruction. Silent material still advances the pen
but produces neither a glyph nor a color record.

The completed result can release source strings, snapshot copies and ordering
scratch. Source information required upstream by fitting and pagination remains
with those stages. Deferring semantic PDF output does not change their contracts.

## Position and font authority

For each selected text piece, start at the line's current knot pen and retained
baseline. Apply scaled shaping offsets to each glyph origin; advance the inner
pen by shaped advances. Advance the enclosing knot pen by the frozen knot width,
including final spacing and glyphless material. This is the existing placement
arithmetic, not a new measurement step.

The result's final origins are authoritative for consumers. Folio may need
nominal font widths for its own PDF data, but they must not determine a different
glyph sequence or replace these origins. A first backend can place glyphs
explicitly; optimized batching is later work and must preserve placement.

A result's font instance preserves opaque face/size/scale services. It does not
require a resource hash, outline capability or PDF descriptor during placement.
`docrender` separately obtains the corresponding immutable font binary and may
let Folio parse it into its own model. Duplicate parsing is acceptable here.
No parsed-font accessor or Folio-specific metric surface is required upstream.

## Atomic composition and paint traversal

The result retains each included atomic's local border origin and its boundary
among physical glyphs. Adjacent atomics can share that boundary and retain
their order in the atomic table. Negative/zero advances are valid. The result
does not contain child boxes, child glyph results or drawing callbacks.

The bridge qualifies IDs by paragraph occurrence, joins selected child geometry,
and avoids duplicate descent. The existing tytui walker demonstrates precise
local-anchor composition with content-edge offsets. Pagelayout also publishes
already-resolved origins through CSS float32 coordinates. A bridge contract must
choose an authoritative path and precision; the result does not settle it.

The required box/context algorithm managed by `pagelayout` specifies supported
decoration/content phases and receiver attachment. Its exact package remains
open. Reuse horizontal traversal and selected atomic placement; the remaining
subtree work is to include each child's decoration and contained output once.
Numeric BoxID sorting and a flat pass over all paragraph groups do not supply
that structure.

Sources: [tytui fragment composition](../../tytui/documentlayout/proof_fragment.go),
[child composition](../../tytui/documentlayout/proof_fragment_boxes.go),
[page atomic conversion](../../pagelayout/preparation/typography/fit.go).

## Callbacks versus the working result direction

| Concern | Synchronous callbacks | Complete immutable result |
| --- | --- | --- |
| Publication | Earlier records may be observed before a late failure. | No result is published until selected construction succeeds. |
| Ownership | Retained buffers require copying and explicit borrowing rules. | Private completed storage exposes read-only value access. |
| Reuse | A consumer can collect records; otherwise ordering/placement repeats. | SVG and PDF inspect the same settled positions and colors. |
| Failure | Consumer effects require their own discard or partial-delivery policy. | Backend failure leaves the result reusable. |
| Storage | Can stay near one piece if consumers do not collect. | Scales with selected glyphs and anchors, without semantic text retention. |

The working direction remains one immutable result. A private callback walker
can build it without establishing a second public protocol. Fonts remain
borrowed in either design. No result needs to retain a preparation, trial,
fitter or pagination session.

Construction now includes the foreground values needed for drawing. Unsupported
source-dependent paint can therefore fail construction, while uniformly colored
placement does not depend on source mapping. Outline/resource/serialization
failures remain later consumer boundaries.

## Experimental evidence and next planning checks

`tycli` exercises ordinary SVG proofs and positioned fragment/flow proofs.
Its documentlayout experiment demonstrates retained geometry, font reuse, RTL
and ligatures, selected atomic interiors and precise origin composition. Its
document profile is narrower than the independent typography paragraph pipeline;
manual replacement evidence comes from typography's prefix-render tests.

Useful existing evidence:
[fragment rendering](../../typography/integration/fragment_render_test.go),
[manual-prefix rendering](../../typography/integration/prefix_manual_render_test.go),
[tytui fragment proofs](../../tytui/documentlayout/proof_fragment_test.go),
[directional ordering](../../tytui/pipeline/visual_order_test.go).

Remaining integration work:

1. A simple two-page print input and its accepted foreground/box paint.
2. Consume the implemented detached glyph/color/anchor result at the page bridge.
3. Page occurrence joins and an explicit atomic transform contract.
4. A required paint-order algorithm managed by `pagelayout`, with selected
   structural facts and frame policies from the [context trace](css_paint_context_trace.md).
5. Extend the implemented `pdf.Page.PaintFragmentGlyphs` connection to
   selected page occurrences, using identical font bytes and authoritative
   glyph placement. Local range submission is already tested.

Semantic extraction and complex CSS effects are not prerequisites for these
checks. Concrete display-list operations, representation and replay protocol
remain undecided. The current implementation covers the typography boundary;
it does not implement page paint traversal or atomic child composition.
