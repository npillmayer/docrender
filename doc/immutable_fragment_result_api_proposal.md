# Minimal immutable fragment API

**Status:** Implemented typography boundary for the two-page PDF spike,
28 November 2026. [REMARKS.md](REMARKS.md) governs scope. The declarations below
describe [`typography/render`](../../typography/render/result.go); page bridge
and display-list decisions remain open.

The implementation lives in `typography/render`. One result describes one selected
`layout.Fragment`, with positioned glyphs, resolved solid foreground colors,
font references, logical line geometry and atomic anchors. It retains no
paragraph text, replacement strings or semantic cluster model.

The [ownership sketch](immutable_fragment_result_sketch.md) explains lifetime;
the [walkthrough](immutable_fragment_result_walkthrough.md) checks RTL, ligatures,
soft hyphens and atomics. Page paint traversal is a separate algorithm whose
module ownership remains open.

## Construction and paint input

```go
// Public declarations in package render; bodies are omitted.
// Imports: image/color, khipu/contracts, typography/layout,
//          golang.org/x/text/unicode/bidi.

type FragmentResult struct {
    // Private immutable storage. Use the constructor; zero inspects as empty.
}

type SolidPaintSpan struct {
    Start, End uint64      // Half-open processed-paragraph UTF-8 bytes.
    Color      color.NRGBA // Already resolved; no CSS interpretation here.
}

type TextPaint struct {
    Default color.NRGBA
    Spans   []SolidPaintSpan
}

func BuildFragmentResult(
    fragment *layout.Fragment,
    paint TextPaint,
    orderer GlyphOrderer,
) (*FragmentResult, error)

type GlyphOrderer interface {
    // Logical input ordinals, returned in physical drawing order.
    VisualPermutation(
        logical []contracts.ShapedGlyph,
        direction bidi.Direction,
    ) ([]int, error)
}
```

The constructor borrows the fragment, paint spans and ordering service for the
synchronous call. It validates and publishes a complete result or returns
`nil, error`, preserving causes and original line/knot context where available.
Invalid input wraps `contracts.ErrIllegalArguments`. An empty selected range
returns a valid empty result with no baseline, even if its frozen owner has
other lines.
The orderer must be nonnil, including for empty selections. Nil/zero results can
be inspected as empty, but are not constructed results.

Paint spans address the full processed paragraph associated with the fragment,
including for a suffix selection. They are ordered, nonoverlapping, nonempty
ranges with valid UTF-8 boundaries within that source. Gaps use `Default`.
The caller supplies the matching group's resolved paint; no frame style is
inherited. The zero color is transparent, not an implicit black sentinel.
The simple print fixture can supply opaque black explicitly.

`color.NRGBA` is a backend-neutral value representation, not a promise of
general CSS opacity or blending support. The spike uses straightforward solid
paint; profile decisions for additional effects remain separate.

Construction reuses the retained glyph traversal. Direction, source anchors,
clusters and replacement text are available temporarily from frozen input.
It orders each detached directional text fragment once, resolves its glyph
colors, and computes final local origins. It neither reshapes nor rebreaks,
restacks, queries line metrics, binds a new font instance or extracts outlines.

The ordering service borrows its detached input without modifying or retaining
it. Its permutation must contain every logical input index exactly once.
Construction carries font and paint associations through that permutation and
discards it afterward. The completed result retains neither the service nor
logical glyph ordinals. Construction rejects mutation of the borrowed glyphs.
The OpenType render adapter implements this interface without parsing fonts.
Existing outline proofs retain their diagnostic output and reordered-value
backend interface, sharing the private knot walker and glyph positioning helper.

## Read-only surface

```go
func (r *FragmentResult) Box() layout.Box

func (r *FragmentResult) LineCount() int
func (r *FragmentResult) Line(i int) (FragmentLine, bool)

func (r *FragmentResult) GlyphCount() int
func (r *FragmentResult) Glyph(i int) (PositionedGlyph, bool)

func (r *FragmentResult) FontCount() int
func (r *FragmentResult) Font(i int) (RetainedFont, bool)

func (r *FragmentResult) AtomicCount() int
func (r *FragmentResult) Atomic(i int) (AtomicAnchor, bool)
```

Accessors return value records; no mutable slices or maps escape. Invalid indexes
return the zero value and `false`. Valid inspection needs no services. All table
indexes are result-local, and zero is a valid index. The result does not retain
the original fragment merely to expose its provenance; the bridge keeps the
occurrence association it needs.

## Retained value records

```go
type IndexRange struct{ Start, End int }

type FragmentLine struct {
    X, Top, Height, Baseline float64
    Width, TargetWidth       float64
    Glyphs, Atomics          IndexRange
}

type PositionedGlyph struct {
    GlyphID uint32
    Font    int
    X, Y    float64     // Final local origin, including shaping offsets.
    Color   color.NRGBA // Resolved foreground; no source-text lookup at replay.
}

type RetainedFont struct {
    Instance  contracts.FontInstance
    UnitsToBP float64 // Captured positive conversion from font units.
}

type AtomicAnchor struct {
    ID          contracts.AtomicID // Paragraph-local occurrence identity.
    Line        int
    BeforeGlyph int     // Boundary in the result's physical glyph table.
    X, Y        float64 // Precise fragment-local border origin.
}
```

Geometry uses fractional big points, Y downward. `Box` and line dimensions
are logical allocation, not ink bounds. Lines follow selected line order;
their half-open glyph and atomic ranges partition the respective tables.

`BeforeGlyph` counts the selected glyphs preceding that anchor in frozen
physical traversal. It is a boundary in its line's glyph range, including the
end boundary. Several anchors can share a boundary; their atomic-table order
preserves their order even when no glyph separates them. A glyphless line can
still contain anchors. Anchors are not sorted by X.

This association lets the bridge locate an atomic among surrounding text without
retaining a material table or walking knots again. It does not instruct the CSS
paint algorithm to draw children in that order or define display-list operations.

Spaces, fill, kerns and silent replacements contribute their retained knot
advances during construction. They need no separate public records for this
spike. Their effects survive in glyph/anchor positions and line geometry.

## Construction-time paint mapping

The shared mapping algorithm stays with typography's render traversal. Reuse
the supported rules in [`PaintSources`](../../typography/render/paint_sources.go)
while frozen records are available. Map raw scalar anchors to processed-byte
paint positions, select the corresponding resolved color, and keep only that
color in the result. Scratch mappings and source strings are released before
publication. There is no retained `PaintSourceMap` or source accessor.
Source is read temporarily through the selected fragment's frozen owner when
spans are present; callers do not pass a separate copy of paragraph text.

With no spans, every glyph uses `Default`; no source mapping is necessary.
Glyphless material produces no color and needs no mapping solely for semantic
validation. With source-dependent paint, ordinary anchors use paragraph scalar
coordinates. Generated anchors use replacement-local coordinates and the shared
explicit soft-hyphen policy: verify the generated spelling against the original
partition while mapping its retained scalars and inserted hyphen.

An unsupported mapping that is needed to choose a glyph's color fails
construction with contextual error; never invent an anchor or silently use the
default. Uniform-color rendering does not need that policy and must not be
blocked by a semantic reconstruction requirement. This is a narrower use of the
existing helper than the earlier proposal to validate every silent replacement.

One glyph receives one color. Repeated anchors in a text piece share the mapped
paint; a ligature is not split to follow an internal color boundary. Distinct
mark anchors remain distinct. SVG and PDF consume the same resolved colors.
Changing paint requires rebuilding this small result from retained input.

## Traversal invariants

Before publication, require:

- A bijective ordering permutation for each selected text snapshot. Carry all
  associated values together, including for repeated GIDs and anchors.
- Valid sized font references, positive finite scales, finite final positions,
  and bounded line/table associations.
- Correct byte/scalar domains wherever source-dependent paint is resolved.
  Temporary cluster associations preserve original logical membership through
  ordering; original cluster intervals never index reordered glyphs directly.
  No source coverage is invented for glyphless or omitted material.
- Every selected glyph emitted once and every selected atomic matched once to
  its authoritative `Fragment.AtomicPlacements()` entry. Excluded lines emit
  nothing.
- Anchor boundaries inside their owning line's glyph interval and nondecreasing
  in physical traversal order. Coincident or backward X positions are valid.

Direction is copied from each retained shaping context and validated for the
ordering operation, including generated text. It is construction input, not a
new retained direction/run table. No public cluster graph, inverse-cluster
accessor or complete source-consumption invariant is required.

## Ownership and position authority

The result owns its selected numeric/color records and retains exact existing
font instances. Font/scale objects remain borrowed services: strong references
keep them reachable, but the result does not close or rebind them. Capture
`UnitsToBP` so ordinary numeric inspection needs no font query. The implementation
adds one font entry per nonempty text piece, without comparing opaque face values.

After construction, callers can release the original fragment, source readers,
paint input and ordering service. The result keeps no frozen arena, preparation,
trial, source text, DOM lookup, callback or page session alive.

The chosen shaper owns GID selection and shaping advances/offsets. Layout owns
final knot spacing and line/atomic placement. Construction captures their
combined effect in final origins: glyph pens use shaped advances, while the
next knot starts after the frozen knot's final advance.

Folio must reproduce those origins. Its nominal font widths may be needed for
PDF internals but cannot replace settled placement. Absolute origins suffice for
the spike; a later batching API needing explicit advances must take authoritative
upstream values, not infer them from offset glyph origins or introduce Folio
metrics into typography.

## Bridge and backend boundaries

The bridge qualifies atomic IDs by paragraph occurrence, retains matching child
geometry/content-edge offsets, and arranges each included interior once.
Precision and the choice between local-anchor composition and already-composed
page origins remain bridge contracts. Neither child data nor transform policy
enters `FragmentResult`.

CSS paint traversal is an independent algorithm over resolved page/box/text
facts. Its module owner is open. Local glyph/anchor traversal does not settle
normal-flow paint order, stacking, or a display-list design.

Only `docrender` pairs retained font instances/resources with Folio's private
font representation. Parsing the same immutable binary separately for Folio is
acceptable. Semantic PDF text, reading order, extraction, accessibility and
retained original text are deferred. Existing source information used by
pagination or paint construction remains in those stages.

Compared with the earlier proposal, this removes public paragraph/replacement
text, source ranges, material/cluster tables, logical glyph ordinals, retained
direction and a persistent paint-source lookup. It keeps the immutable delivery
and borrowed-font contract, with resolved color as the drawing information that
replaces retained source-based paint lookup.

Implementation coverage lives in
[`fragment_result_test.go`](../../typography/integration/fragment_result_test.go)
and [`result_test.go`](../../typography/render/result_test.go), including real
ligatures/RTL/soft hyphens, signed positioning, atomic boundaries, failures and
release of construction inputs. Docrender's
[`Page.PaintFragmentGlyphs`](../pdf/fragment.go) consumes explicit glyph ranges
and translations, using captured scale and resolved color. It skips transparent
glyphs, rejects partial alpha and leaves atomic composition and page paint order
with the caller. Selected page occurrence integration remains open.
