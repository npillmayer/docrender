# Minimal immutable fragment API proposal

**Status:** Planning proposal, 28 November 2026. These are illustrative Go
declarations for the working immutable-result direction, not implemented APIs
or a display-list design. Names and signatures remain reviewable.

This makes the [contents and ownership sketch](immutable_fragment_result_sketch.md)
concrete enough to discuss construction and inspection. The proposed home is
`typography/render`. One result describes one selected `layout.Fragment`, with
local geometry and source correspondence. Page occurrences and atomic interiors
remain associations maintained by the caller.

The [four-case walkthrough](immutable_fragment_result_walkthrough.md) traces RTL
marks, a ligature, a soft-hyphen replacement and an atomic inline box through
these declarations. The proposal now includes retained text direction, explicit
cluster invariants and a shared paint-source lookup. Atomic composition remains
a bridge contract.

## Construction

```go
// Proposed declarations in package render; method bodies are omitted.
// Imports: khipu, khipu/contracts, typography/layout,
//          golang.org/x/text/unicode/bidi.

type FragmentResult struct {
    // Private immutable storage. The zero value is not a constructed result.
}

func BuildFragmentResult(
    fragment *layout.Fragment,
    orderer GlyphOrderer,
) (*FragmentResult, error)

type GlyphOrderer interface {
    // Return logical input indexes in physical drawing order.
    VisualPermutation(
        logical []contracts.ShapedGlyph,
        direction bidi.Direction,
    ) ([]int, error)
}
```

The constructor takes the already-selected fragment and an ordering capability.
It obtains the complete processed paragraph text from the matching frozen
owner, preserving paragraph-relative source coordinates. There is no separate
caller-supplied text string to mismatch with the fragment. The current
`FrozenKhipu.SourceReader` provides access to that retained text; the result must
retain immutable text storage without retaining the reader or the frozen arena.
This reads retained source, not an external file.

The ordering method receives one complete detached directional glyph fragment
in its original shaping order. Its argument is borrowed for the call: the
service must neither modify nor retain it. The returned permutation must contain
every input index exactly once. Construction consumes/copies that mapping and
retains no service-owned storage or ordering service. For example, `[2, 1, 0]`
means that logical glyph 2 becomes the first physical glyph. This carries
identity even when glyph values, GIDs and scalar anchors repeat.

This is a proposed change to the ordering boundary, not an adapter signature
that exists today. The current render interface returns reordered glyph values,
which alone cannot reliably recover original ordinals. The concrete OpenType
ordering helper is generic and can order ordinal records; generic `render`
would still depend only on the injected capability.

Construction is synchronous and all-or-nothing. Validate selected associations,
source domains, retained directions, permutation bijections, glyph/cluster membership, fonts, finite
geometry and matching atomic placements before publishing. An error returns
`nil` and preserves its cause with original line/knot context where available.
No new public error hierarchy is needed in this proposal. A valid empty
selection returns a non-nil result with zero placement counts and no baseline;
the source paragraph need not be empty. A nil/invalid fragment or missing
ordering capability is an input error.

The constructor uses retained font scale conversions but performs no font
binding, metrics, shaping, line layout, identity lookup or outline extraction.
It does not invoke paint mapping. A structurally valid replacement can therefore
produce a placement result even when its spelling is outside the paint helper's
supported mapping policy.

## Read-only surface

```go
func (r *FragmentResult) OriginalRange() layout.LineRange
func (r *FragmentResult) Box() layout.Box
func (r *FragmentResult) ParagraphText() string

func (r *FragmentResult) LineCount() int
func (r *FragmentResult) Line(i int) (FragmentLine, bool)

func (r *FragmentResult) MaterialCount() int
func (r *FragmentResult) Material(i int) (FragmentMaterial, bool)

func (r *FragmentResult) GlyphCount() int
func (r *FragmentResult) Glyph(i int) (PositionedGlyph, bool)

func (r *FragmentResult) ClusterCount() int
func (r *FragmentResult) Cluster(i int) (GlyphCluster, bool)
func (r *FragmentResult) ClusterGlyph(cluster, member int) (glyph int, ok bool)

func (r *FragmentResult) FontCount() int
func (r *FragmentResult) Font(i int) (RetainedFont, bool)

func (r *FragmentResult) AtomicCount() int
func (r *FragmentResult) Atomic(i int) (AtomicAnchor, bool)
```

Accessors return detached value records, immutable strings or individual indexes.
No slices, maps or mutable nested records escape. Invalid indexes, including
negative ones, return the zero value and `false`. Valid inspection requires no
services and cannot introduce a late build error. Callers use constructor-made
results; the exported type's zero value is not an alternative construction path.

All local indexes below are zero-based and scoped to the owning result. Zero
is a valid index, never an absence sentinel. `IndexRange` is half-open and indexes
the table named by its field. The proposed plain `int` indexes keep this surface
small; stronger handle types can be considered later if actual misuse warrants
them.

## Value records

```go
type IndexRange struct{ Start, End int }

type FragmentLine struct {
    OriginalLine                           int
    X, Top, Height, Baseline                float64
    Width, TargetWidth, LeftSkip, RightSkip  float64
    Materials                              IndexRange
}

type FragmentMaterial struct {
    Line, OriginalKnot int
    Kind              khipu.KnotType
    Provenance        khipu.FrozenMaterial
    Source            khipu.SourceRange // Always processed-paragraph coordinates.
    X, Advance        float64           // Initial pen X and final knot advance.
    Glyphs, Clusters   IndexRange

    HasText         bool
    Font            int            // Valid only when HasText, even for silent text.
    Direction       bidi.Direction // Retained shaping direction; valid when HasText.
    IsReplacement   bool           // Valid only when HasText.
    ReplacementText string         // Exact generated text; empty can be meaningful.

    Atomic int // Valid only when Kind == khipu.KTAtomic.
}

type PositionedGlyph struct {
    Material     int
    GlyphID      uint32
    X, Y         float64 // Final local origin, including shaping offsets.
    LogicalIndex int     // Ordinal in this material's original CopyText glyphs.
    ScalarAnchor uint32  // Original raw shaping anchor; not a byte offset.
    Cluster      int     // Index into this result's cluster table.
}

type GlyphCluster struct {
    Material   int
    Input      khipu.SourceRange // Domain determined by its text material.
    GlyphCount int              // Enumerate membership with ClusterGlyph.
}

type RetainedFont struct {
    Instance  contracts.FontInstance
    UnitsToBP float64 // Captured positive conversion: one font unit to big points.
}

type AtomicAnchor struct {
    Material int
    ID       contracts.AtomicID // Paragraph-local occurrence ID.
    X, Y     float64            // Precise fragment-local border origin.
}
```

The records reuse existing scalar contracts without exporting frozen snapshots
or shaping parameters. Line membership comes through the material, and glyph
font membership through the text material's font index. An atomic anchor's source
marker and line likewise come through its material. These joins avoid multiple
copies of the same association.

For every text material, `Direction` is copied from the selected snapshot's
`FrozenText.Context.Params.Direction`, including replacement and silent text.
It must be `bidi.LeftToRight` or `bidi.RightToLeft` and is exactly the direction
passed to the ordering service. It describes that retained text piece, not the
paragraph's base direction. It is not inferred again from the text or from glyph
positions. When `HasText` is false, no direction or font is meaningful; do not
use an enum's numeric zero as a presence test.

Direction is inspection metadata after construction. Consumers replay the
already-ordered glyphs without reordering again. It does not expose embedding
levels, original isolate markup, or a page reading order.

All geometry here uses fractional big points, with Y downward. `Box` is logical
geometry and carries `HasBaseline`; it is not an ink-bound promise. Material
advance may be negative, and its initial pen position is distinct from an
atomic border origin. Nonglyph material remains inspectable, including retained
spaces, fill and silent replacements. Fill is identified by provenance, not
guessed from an empty source range.

Lines follow the selected paragraph sequence. Materials follow physical frozen
knot order, and each material's glyph range follows its computed drawing order.
Atomic records follow the same physical traversal. No list is sorted by X.
Clusters are grouped by material in retained cluster order; this does not
declare logical reading order across materials or page occurrences.

## Source, cluster and identity rules

| Record | Meaning of its coordinates |
| --- | --- |
| `FragmentMaterial.Source` | The original processed paragraph partition, including for a replacement. It addresses `ParagraphText()`. |
| Ordinary text's `GlyphCluster.Input` and `ScalarAnchor` | Byte/scalar range and scalar anchor in the complete processed paragraph. |
| Replacement text's `GlyphCluster.Input` and `ScalarAnchor` | Range and scalar anchor local to that material's exact `ReplacementText`. The domain is scoped by the material index. |

`IsReplacement`, rather than string emptiness, chooses the text domain. Byte and
scalar coordinates remain distinct. A replacement's original partition is a
whole-partition association; this API does not invent a character-by-character
map. Processed-byte paint anchors are supplied by the separate `PaintSources`
helper proposed below. Cluster ranges remain the source correspondence; a paint
anchor does not replace them.

### Cluster invariants

These are publication requirements of `BuildFragmentResult`, not checks left
to each SVG or PDF consumer. Violations return no result. The constructor must
preserve the selected frozen records; it must not repair inconsistent metadata
by sorting, merging or inventing clusters.

1. **Material ownership.** Every glyph and cluster belongs to exactly one text
   material. Material glyph/cluster ranges are bounded, disjoint ranges covering
   their respective tables, and every record's `Material` points back to its
   owning range. A material without text has empty glyph/cluster ranges. Empty
   ranges may share an endpoint; `HasText` remains independent of glyph count.
2. **Logical-to-physical bijection.** For a material with `n` glyphs, the
   permutation has length `n` and contains each integer in `[0,n)` exactly once.
   Physical glyph `m.Glyphs.Start + p` retains `LogicalIndex=permutation[p]`.
   Thus logical indexes are unique within that material, including when GIDs,
   scalar anchors or full glyph values repeat. They are not fragment-wide IDs.
3. **Preserved cluster membership.** Each original `FrozenCluster` glyph
   interval is bounded within the logical snapshot. Those intervals assign
   every logical glyph to exactly one cluster, with no overlap or omission.
   Copy the cluster's input range and remap its members using the permutation;
   do not apply the original interval to the physical glyph table. A cluster's
   `GlyphCount` is its preserved member count, not its character count.
4. **Exact inverse access.** `ClusterGlyph(c,j)` succeeds exactly for valid
   `c` and `0 <= j < Cluster(c).GlyphCount`. It enumerates distinct physical
   glyph indexes in physical order; members need not be contiguous. Each such
   glyph has `Cluster=c` and the same `Material` as the cluster. Conversely,
   every glyph appears exactly once in its referenced cluster's membership.
5. **Consistent source domains.** Byte and scalar endpoints of every source
   range identify the same UTF-8 boundaries in the declared text domain. An
   ordinary cluster's input lies within its material's paragraph source range;
   a replacement cluster addresses `ReplacementText` independently of the
   original partition. Each glyph's raw scalar anchor is inside its cluster's
   half-open scalar interval. Anchors are copied unchanged, may repeat, and
   need not identify distinct glyphs. No byte/scalar arithmetic is inferred
   from glyph count or visual order.
6. **Empty material and incomplete coverage.** Preserve any retained zero-member
   clusters, with no successful membership lookups. Text with no glyphs may
   have an empty cluster range and nonempty original source, including silent
   replacements. Do not synthesize clusters for omitted source or require
   cluster ranges to cover every source character. Generated fill and atomic
   material have no text clusters.

For example, logical cluster members `{0,1}` under permutation `[2,1,0]`
become physical members `{1,2}`. A ligature can have one member and a multi-scalar
input range. Both satisfy these rules without additional character geometry.

### Provenance and coverage

The minimal provenance scope is the result itself: interpret `OriginalLine`
and `OriginalKnot` together with the owning `*FragmentResult`. They refer to
the frozen owner used for that construction, not to the processed text string.
The outer bridge retains the result-to-paragraph/occurrence association. This
proposal provides no cross-result frozen-owner equality operation; callers
needing that relationship retain it explicitly. Equal text, source offsets or
knot indexes do not establish it, especially across continuation results.

These records describe selected retained material, not complete source
consumption. Trimmed or omitted text still requires the external prefix and
coverage information. Original HTML source nodes, paint spans and semantic PDF
span policy also remain external.

## Shared paint-source mapping

The working helper shape replaces the existing frozen-snapshot `PaintSources`
signature when implementation is undertaken. Current code is unchanged; this
is not a proposed compatibility overload or a second mapping algorithm.

```go
type PaintSourceMap struct {
    // Private immutable anchors, plus the owning FragmentResult reference.
    // The zero value is not a constructed map.
}

// Map all selected text materials under the supported paint-source policy.
func PaintSources(fragment *FragmentResult) (*PaintSourceMap, error)

// The exact result whose physical glyph indexes this map accepts.
func (p *PaintSourceMap) Fragment() *FragmentResult

// For a valid glyph index, return a processed-paragraph UTF-8 byte anchor.
func (p *PaintSourceMap) SourceByte(glyph int) (byteOffset uint64, ok bool)

// Proposed errors.Is sentinel; initializer deliberately omitted here.
var ErrUnsupportedPaintMapping error
```

`PaintSources` takes only the completed result. It reads its immutable strings,
material provenance and raw glyph anchors; it receives no paint spans, colors,
font services, frozen snapshots or DOM callbacks. SVG proof and page/PDF
consumers use the same helper, then resolve their own paint spans at the
returned processed-byte anchor. There are no ordering, layout or font queries.

The helper builds one complete immutable lookup, indexed by **physical glyph
index**, rather than by scalar anchor alone. On success, every index in
`[0, fragment.GlyphCount())` has exactly one entry and `SourceByte` returns
`true`; byte offset zero is valid. Invalid indexes return `(0, false)`. `false`
never means an unsupported mapping hidden inside a successful map. Repeated
anchors within the same material/domain map consistently, while equal raw
anchors in different replacement materials remain independent.

The supported mapping policy is the current `PaintSources` policy:

| Material | Mapping |
| --- | --- |
| Ordinary text | Convert each paragraph-relative scalar anchor to the corresponding UTF-8 scalar-start byte in `ParagraphText()`. |
| Pre-break replacement | Read its original paragraph partition, omit soft hyphens except its last U+00AD, and replace that marker with `-`. Require the resulting spelling to equal `ReplacementText` exactly. Map each generated scalar to the original scalar's start byte; the hyphen maps to the marker's start byte. |
| Post-break replacement | Read its original partition and omit soft hyphens. Require exact equality with `ReplacementText`, then map each retained generated scalar to its original start byte. |
| Material without text | Produce no glyph entries. Atomic boxes, spaces and fill do not acquire invented paint anchors. |

The spelling check applies to every selected text material, including glyphless
replacements, preserving the existing helper's validation policy. A supported
silent replacement contributes no entries. An unfamiliar silent transformation
can still fail this optional helper; it does not invalidate the placement
result. Empty selections and atom-only selections have a valid empty map.

Construction of the map is all-or-nothing: unsupported generated spelling
returns `(nil, error)` wrapping `ErrUnsupportedPaintMapping`. Nil or
unconstructed result input wraps `contracts.ErrIllegalArguments`. Errors include
the relevant material and original line/knot context when available and preserve
underlying causes. Do not guess a fallback anchor, silently omit a glyph or
return a partial map. A successful map's accessors cannot later fail on source
mapping. Calling this helper does not mutate or invalidate the fragment result,
including on failure.

`PaintSourceMap` owns its numeric lookup and retains the exact immutable result
returned by `Fragment()`. That makes its index scope inspectable and also keeps
the result's text/font references reachable; it acquires no ownership of closing
borrowed font services. It retains no construction scratch or working layout
handles. Dropping it releases those references. There are no writable slices,
maps, callbacks or `Close` method. One successful map can be read concurrently
and shared by SVG and PDF consumers; each must use its associated result.

For the walkthrough's RTL example, `SourceByte(0)` is `(4,true)` for bet, and
the mark/alef entries are `(0,true)`. For `co-`, the hyphen maps to processed
byte 2. For a separate `operate` continuation result, its first physical glyph
maps to processed byte 4 despite replacement-local scalar anchor zero. These
are paint anchors, not original HTML offsets, complete cluster ranges or a
policy for PDF semantic text.

## Ownership and reuse

The result owns its selected records and private membership storage. All
mutable build buffers must be copied or exclusively transferred before
publication. Stable strings may be shared. Once construction returns, the
caller can release the original fragment, frozen handle and ordering service;
the result has no dependency on preparations, trials or page sessions.

`RetainedFont.Instance` is the deliberate borrowing boundary. Returning it by
value preserves the exact sized instance and strong references to its opaque
face/scale services. It does not make those service implementations owned by
the result or enforce their immutability. Their owner must keep them stable and
usable for retained consumers. Numeric inspection uses `UnitsToBP` and needs no
font query. There is no `Close`, font rebinding or mandatory font deduplication;
do not compare arbitrary face interface values or substitute family names.

SVG can join a glyph to its retained font, request an outline separately, and
use the stored origin and scale. PDF can join the same records to its own font
resources and source policy. Either consumer can fail on missing capabilities
without changing the result. Read-only inspection can be concurrent; calls into
borrowed font/backend services follow those services' own concurrency contracts.

Atomic anchors retain no child tree, child result or rendering callback. The
page bridge binds their paragraph-local IDs to selected child occurrences,
retains the matching selected child geometry and content-edge offsets, composes
their placement once, and controls painting order. It qualifies atomic IDs by
paragraph occurrence and prevents duplicate traversal of an included interior.
Choosing between precise local-anchor composition and already-composed page
origins, including their precision contract, remains bridge work. Neither this
result nor the paint helper decides that transform or a display-list operation.

## Review boundary

This proposal offers indexed inspection and a permutation as concrete API
shapes to review. Internal storage, allocation/caching strategy, font-entry
reuse and any private callback walker remain implementation questions. Neither
an outline API, page painter API, PDF encoding API nor display-list format is
specified here.

Current-code references: [fragment geometry](../../typography/layout/fragment.go),
[frozen source and clusters](../../khipu/frozen_result.go),
[font instance](../../khipu/contracts/font_instance.go),
[current proof traversal](../../typography/render/paragraph.go),
[current render services](../../typography/render/services.go),
[generic ordering helper](../../opentype/otshape/visual_order.go),
[current paint mapping](../../typography/render/paint_sources.go).
