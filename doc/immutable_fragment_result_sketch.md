# Immutable glyph fragment result

**Status:** Working direction for the two-page PDF spike, 28 November 2026.
[REMARKS.md](REMARKS.md) sets the priorities: module responsibilities, useful
positioned-glyph output and simple print paint. Semantic text is deferred.
The [Go API proposal](immutable_fragment_result_api_proposal.md) is illustrative;
there is no implementation or display-list decision.

## Purpose and scope

Return one complete immutable result per selected `typography/layout.Fragment`.
It contains enough drawing information to reuse the same local placement in SVG
and PDF without repeating ordering or pen accumulation. A structural pagelayout
fragment may contain several paragraph occurrences and atomic interiors, each
with its own result and external occurrence association.

The result is deliberately detached from semantic text. Full paragraph text,
replacement spellings, raw source anchors, cluster ranges and logical glyph
ordinals are construction inputs or scratch, not retained output. Existing
source data used by preparation, pagination and paint resolution is not removed
from those stages.

## Retained contents

| Part | Purpose |
| --- | --- |
| Logical fragment box and lines | Preserve selected local allocation and baselines, with ranges into glyph and atomic tables. These are not ink bounds. |
| Positioned glyphs | Exact GID, result-local font reference, final X/Y and resolved solid foreground color. |
| Font entries | Exact retained sized instance plus its captured font-unit-to-big-point conversion. |
| Atomic anchors | Paragraph-local ID, local line, physical glyph boundary and precise local border origin. Child geometry stays outside. |

All geometry uses fractional big points with Y downward. Glyphs preserve physical
drawing order. An atomic's `BeforeGlyph` identifies its boundary among selected
glyphs; atomic-table order distinguishes adjacent anchors at the same boundary.
Neither sequence is a complete CSS page paint order.

Spaces, kerns, fill and silent replacements affect the pen through their final
frozen advances. They do not need public source/material records in the spike.
A valid empty selection returns empty placement tables and no baseline.

## Construction versus retention

Construction traverses only selected lines, using their original frozen indexes
and local geometry. It uses retained direction to order each detached text piece
once. Temporary ordinals or a permutation keep paint/font associations attached
to the exact glyphs, including repeated anchors and GIDs.

For source-dependent colors, shared render helpers convert scalar anchors to
processed-byte paint positions while the matching text and replacement facts
are available. Ordinary and generated coordinate domains remain distinct.
Only the resolved color survives. Uniform default paint requires no source
mapping; glyphless material needs no invented color entry.

The existing supported soft-hyphen mapping is reused. A needed but unsupported
mapping fails construction rather than inventing paint ownership. Semantic
reconstruction is not a condition for rendering uniformly colored glyphs.

Publication validates glyph conservation, ordering, finite geometry/scales,
font references and selected atomic correspondence. Source/cluster checks are
construction-time checks where needed for association and paint. There is no
public cluster graph or requirement to preserve all consumed source.

## Ownership contract

| Object | Ownership |
| --- | --- |
| Placement, line, color and atomic records | Owned immutable snapshot; read-only indexed access returns values. |
| Font instances | Strongly referenced, but face/scale services remain borrowed and stable for their consumers. No closing or rebinding by the result. |
| Paint spans, source strings and temporary mappings | Borrowed or read during construction; not retained after publication. |
| Ordering service and scratch | Used only during construction; no callbacks or scratch arrays retained. |
| Original fragment and frozen arena | Not retained by the completed result. |
| Page occurrence and child bindings | Owned by the bridge; qualify paragraph-local atomic IDs and retain matching selected geometry. |
| Folio fonts, PDF names and embedding caches | Owned locally by `docrender`; absent from typography output. |

The result can outlive preparations, trials and original fragment handles.
Dropping it releases references but never closes a font service or invalidates
another result. Numeric/color inspection is read-only and can be concurrent;
backend/font service calls follow their separate concurrency contracts.

Paint is a captured drawing value. Repainting with different spans means building
a new small result from retained input, rather than keeping semantic text alive
solely for later recoloring. No mutable CSS styles are captured.

## Font and positioning authority

A GID is interpreted with its exact sized font instance. Deduplication is not
required; do not merge by family name or compare arbitrary opaque face values.
Capture the scale used during construction so SVG outline scaling and numerical
inspection do not need rebinding.

The chosen shaper determines glyph selection and shaping advances/offsets.
Layout supplies baselines, final knot advances and atomic geometry. The result
captures their combined final positions. Folio's nominal widths cannot replace
these positions or drive a second layout.

Folio may create a separate private font model from the same immutable binary.
`docrender` owns that association, the font resource capability checks and any
backend-specific descriptors or subsetting requirements. Generic typography and
pagelayout do not acquire Folio types or font-metric requirements.

Sources: [font instance](../../khipu/contracts/font_instance.go),
[current traversal](../../typography/render/paragraph.go),
[resource capability](../../typography/font/identity.go).

## Atomic and page boundaries

Copy local border origins from `Fragment.AtomicPlacements()`. A knot's advance
is not the border origin; signed margins can produce negative advances and
decreasing X positions. Preserve traversal order and selected-line membership.

The bridge binds anchors to selected child boxes/interior paragraphs and
retains border/content offsets. It must avoid duplicate child traversal and
choose one authoritative transform path. Current pagelayout CSS float32
composition and precise local float64 anchors require an explicit precision
contract; adding both origins is not valid.

CSS paint traversal remains a separately encapsulated algorithm with open module
ownership. Neither this result's atomic order nor its line/glyph tables assigns
that algorithm to typography or `docrender`, or decides display-list structure.

Sources: [fragment geometry](../../typography/layout/fragment.go),
[atomic composition experiment](../../tytui/documentlayout/proof_fragment_boxes.go),
[page conversion](../../pagelayout/preparation/typography/fit.go).

## Deferred work

Original-text retention for rendering, text extraction/search guarantees,
reading order, accessibility, complete consumption mapping and semantic PDF
spans are later concerns. Rich CSS effects are also outside the first target.
Future extensions should follow demonstrated responsibilities and requirements;
the spike does not reserve an unused semantic subsystem in its public result.
