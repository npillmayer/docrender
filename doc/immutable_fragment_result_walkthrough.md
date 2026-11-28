# Walking the fragment API through four cases

**Status:** Planning review, 28 November 2026. This walks through the
[proposed Go API](immutable_fragment_result_api_proposal.md) without implementing
it or choosing a display-list design. Retained direction, cluster invariants and
the shared paint helper are now part of the refined proposal; all remain
unimplemented planning contracts.

The four cases fit the result's local placement model. The main open points are
the external bindings needed for semantic text and atomic interiors. The traces
below also illustrate the proposed direction, membership and paint-mapping
contracts that implementation would have to preserve.

Each example uses a separate processed paragraph, so indexes and source offsets
start afresh. Glyph labels stand for font-specific GIDs. Shaping choices and
numeric geometry are illustrative retained inputs, not measured output from a
font or an execution of the proposed API. The source ranges follow the current
UTF-8/scalar contracts; current-code evidence is linked with each case.

## Common construction and reading path

`BuildFragmentResult(fragment, orderer)` reads only the selected lines, copies
their retained material and precise atomic placements, retains the matching
full paragraph text, and computes final glyph origins. Ordering is applied once
to each detached directional text fragment. The constructor publishes a complete
result or returns an error and no result.

For a glyph `g`, the consumer follows:

```text
Glyph(g).Material → Material(m).Line → Line(line)
                 → Material(m).Font → Font(font)
Glyph(g).Cluster  → Cluster(c).Input
                 → ClusterGlyph(c, member) → physical glyph indexes
```

`HasText` makes the font index meaningful. An anchor follows
`Atomic(a).Material` to the same line/source records. All joins stay within the
same result; a numeric index from another result is not interchangeable.

SVG and PDF consumers reuse final local positions and the exact retained sized
font. `PaintSources(result)` separately builds a complete immutable
`PaintSourceMap`; `SourceByte(glyphIndex)` looks up each physical glyph's
processed-byte paint anchor. `Fragment()` identifies the result owning those
indexes. Consumers can share that lookup and apply their own paint spans.
Outlines, resource capabilities, colors and page translations are resolved by
those consumers or their integration. A later consumer error leaves the result
reusable.

## 1. RTL text with a combining mark

Use the logical paragraph `אְב`: U+05D0 ALEF, U+05B0 SHEVA, U+05D1 BET.
Its three scalars occupy bytes `[0,2)`, `[2,4)`, `[4,6)`.

Assume one retained RTL text material with logical glyphs
`[alef, mark, bet]`, raw scalar anchors `[0, 0, 2]`, and two clusters:

| Cluster index | Input bytes | Input scalars | Original logical glyph ordinals |
| --- | --- | --- | --- |
| 0 | `[0,4)` | `[0,2)` | 0, 1 |
| 1 | `[4,6)` | `[2,3)` | 2 |

The material retains `Direction=bidi.RightToLeft`, the direction supplied to
the ordering service, which returns `[2,1,0]`. For a concrete arithmetic example,
let the material start at X=10, baseline Y=20, and `UnitsToBP=0.01`.
Logical advances are `[600,0,500]` font units; the mark has offsets `(200,200)`
and the letters have zero offsets. The retained material advance is 11 big
points. The proposed result then contains:

| Physical glyph index | Glyph | `LogicalIndex` | `ScalarAnchor` | `Cluster` | Final `(X,Y)` |
| --- | --- | --- | --- | --- | --- |
| 0 | bet | 2 | 2 | 1 | `(10,20)` |
| 1 | mark | 1 | 0 | 0 | `(17,18)` |
| 2 | alef | 0 | 0 | 0 | `(15,20)` |

`ClusterGlyph(0,0)` returns 1 and `ClusterGlyph(0,1)` returns 2; cluster 1
contains physical glyph 0. The mark's zero advance and nonzero offset survive.
The final X values need not increase with traversal order. The next material
starts at X=21, using the retained knot advance.

After `PaintSources(result)` succeeds, `SourceByte(0)` returns `(4,true)` for
bet. `SourceByte(1)` and `SourceByte(2)` both return `(0,true)` for the mark
and alef. Neither the raw anchor nor GID is a
unique glyph identity. A consumer must not reverse this result again or apply
the original cluster intervals to its physical glyph table.

**What works:** the permutation, `LogicalIndex`, cluster membership and final
origins preserve all facts needed for positioned replay. Mixed-direction lines
also retain the frozen physical material order; the result builder must not
reorder the line's materials using the paragraph's base direction.

**Refined direction contract:** `FragmentMaterial.Direction` copies
`FrozenText.Context.Params.Direction` and is meaningful whenever `HasText`,
including silent/replacement text. Construction requires LTR or RTL. Retention
preserves the fact for inspection; consumers do not reorder again. Direction
alone does not provide embedding levels, isolate context or a page reading
order. None of those should be guessed from X coordinates or by rerunning bidi
on the stored string; original markup may have contributed context outside it.

Sources: [ordering semantics](../../opentype/otshape/visual_order.go),
[repeated-anchor ordering check](../../typography/integration/render_test.go),
[scalar-to-byte mapping check](../../typography/render/paint_sources_test.go).

## 2. A ligature in `office`

Assume the selected font shapes `office` as `[o, ffi, c, e]`. This is an explicit
example assumption; the exact ligature depends on the retained shaping result.
In this ASCII paragraph, byte and scalar offsets happen to agree.

The ligature is physical glyph 1, with `LogicalIndex=1`, `ScalarAnchor=1`,
and `Cluster=1`. That cluster's input is bytes `[1,4)`, scalars `[1,4)`, and
`GlyphCount=1`; `ClusterGlyph(1,0)` returns 1. The glyph resolves its font through
its text material. The material still covers the whole word `[0,6)`.

The consumer draws one glyph. It can associate that glyph with the complete
source sequence `ffi`, even if the three letters came from several HTML text
nodes. The integration intersects the whole cluster range with its external
source maps. Using only `ScalarAnchor=1` for that join would lose the other two
letters' source associations.

For paint, the current policy assigns one color to the glyph at its mapped
anchor. A color boundary inside `ffi` does not create three independently
paintable glyphs. Conversely, the RTL mark example shows why a helper should
preserve per-glyph anchors instead of reducing every cluster to one guessed
source position: other retained shaping can supply distinct mark anchors.

**What works:** a cluster's source range and glyph count are independent. The
API supports one glyph for several characters and several glyphs for a cluster.
Its separate membership accessor does not require physical members to be
contiguous, even though they are contiguous in these two examples.

**Refined cluster contract:** every glyph must belong to exactly one text material
and one retained cluster. The cluster and glyph back-references must agree;
membership must reproduce the original frozen intervals through the validated
permutation. Material-local `LogicalIndex` values must be unique and cover the
original glyph snapshot. Do not validate using GID equality, anchor uniqueness,
character count or increasing X.
The [explicit publication invariants](immutable_fragment_result_api_proposal.md#cluster-invariants)
also require source-domain consistency and preserve empty retained material
without fabricating coverage.

No ligature caret positions or per-character geometry are retained. Those are
outside positioned replay, not required additions for this bridge. The result
also supplies source facts rather than choosing PDF text-extraction spans.

Sources: [frozen cluster contract](../../khipu/frozen_result.go),
[whole-cluster source coverage](../../typography/glyphing/paragraph/prefix_coverage.go),
[tytui mixed RTL/ligature experiment](../../tytui/documentlayout/proof_fragment_test.go).

## 3. A selected soft-hyphen break

Use `co\u00ADoperate`, with a selected break producing `co-` and `operate`.
The original paragraph is 11 UTF-8 bytes and 10 scalars. The soft hyphen occupies
bytes `[2,4)` and scalar range `[2,3)`.

Consider a first result containing the pre-break line and a second result
containing its continuation. Both retain the complete processed paragraph text.
The relevant text materials have:

| Field | First result | Continuation result |
| --- | --- | --- |
| `Provenance` | `PreBreakMaterial` | `PostBreakMaterial` |
| `HasText`, `IsReplacement` | true, true | true, true |
| `Direction` | `bidi.LeftToRight` | `bidi.LeftToRight` |
| `ReplacementText` | `co-` | `operate` |
| Original `Source.Bytes` | `[0,4)` | `[4,11)` |
| Original source scalars | `[0,3)` | `[3,10)` |
| Generated text domain | Local bytes/scalars `[0,3)` | Local bytes/scalars `[0,7)` |

The first partition includes `co` and the original marker; it is not just the
soft-hyphen span. In a simple one-glyph-per-generated-character shaping, the
hyphen's `ScalarAnchor=2` and cluster input `[2,3)` address `co-`. The shared
soft-hyphen paint policy maps this anchor to original processed byte 2.

The continuation's first `o` has replacement-local `ScalarAnchor=0` and cluster
input `[0,1)`. Its paint anchor is processed byte 4, not byte 0. The continuation
remains replacement material even though its visible spelling matches the
original suffix. Local cluster indexes and font indexes can restart in each
result; original paragraph source coordinates do not.

If the generated material is RTL, ordering permutes its glyph ordinals exactly
as for ordinary text. Its anchors still address the generated string. Placement
and source-domain conversion are separate operations.

**What works:** `IsReplacement`, exact replacement text, original source
partition and local cluster ranges preserve both coordinate domains. A selected
empty replacement can have `HasText=true`, `IsReplacement=true`, empty text,
empty glyph/cluster ranges and nonempty original source. Dropping that material
would lose retained provenance. An unselected soft hyphen does not acquire a
visible hyphen merely because the full paragraph string contains the marker.

**Refined helper contract:** the proposal replaces the frozen-snapshot helper
signature with `PaintSources(*FragmentResult) (*PaintSourceMap, error)`.
It maps physical glyph indexes to processed UTF-8 byte anchors, preserving the
existing soft-hyphen policy. An unsupported generated spelling returns no map
and an error matching `ErrUnsupportedPaintMapping`; placement remains valid.
The spelling check also applies to glyphless replacements, as in today's
helper. Supported silent material adds no entries; an unfamiliar silent
transformation can still fail the optional mapping step. Consumers neither
reconstruct frozen snapshots nor duplicate the transformation. Byte zero is a
successful value, and `SourceByte` returns false only for an invalid glyph index.

**External semantic boundary:** the result does not identify a discretionary
pair across separate frozen owners, provide continuation authority, or report
all consumed/trimmed source. Prefix endpoints and the paragraph/occurrence
binding stay external. A consumer needs an explicit policy for visible hyphens
versus original source text; concatenating generated strings or cluster slices
is not a complete semantic reconstruction algorithm.

The current typography integration fixture exercises these same `[0,4)` and
`[4,11)` partitions across a changed-width continuation. Tytui's document demo
currently uses a narrower no-hyphenation profile, so it is not evidence that all
four cases already run together through that route.

Sources: [manual-prefix rendering fixture](../../typography/integration/prefix_manual_render_test.go),
[replacement and silent-material snapshots](../../khipu/frozen_result_test.go),
[current paint policy](../../typography/render/paint_sources.go),
[tytui profile](../../tytui/AGENTS.md).

## 4. An atomic inline box with negative advance

Use processed text `x\uFFFCy`, with an explicitly attached atomic occurrence
ID 1 at the marker. Its source is bytes `[1,4)`, scalars `[1,2)`. An authored
U+FFFC without that retained occurrence would instead remain text.

Suppose the atom's material starts at X=30 on baseline Y=20. For illustration,
its pre-resolved border size is 10 by 8 big points, with left margin -15,
right margin 2 and bottom margin 1. With the currently supported
bottom-margin-edge alignment, layout has already produced:

| Fact | Retained value |
| --- | --- |
| Material `X` | 30 |
| Material `Advance` | -3, from `-15 + 10 + 2` |
| Material classification | `Kind=KTAtomic`, `HasText=false` |
| Material `Atomic` | 0, the valid first anchor index |
| `Atomic(0)` | `ID=1`, matching material index, border origin `(15,11)` |
| Next physical material's pen X | 27 |

The constructor copies the anchor from `Fragment.AtomicPlacements()`; it does
not recalculate it from this example's margins. The atomic ID and anchor-table
index are different concepts. No font index is meaningful on the atomic
material, and its glyph/cluster ranges are empty.

This preserves a border origin different from the material pen, and a following
pen that moves backward. Sorting materials or anchors by X would corrupt the
retained traversal. An atomic knot on an excluded line contributes no anchor
to this result, even if its frozen owner contains that line.

**What works:** the anchor identifies the selected occurrence and its precise
local placement without imposing child ownership on typography. The bridge
qualifies ID 1 with the paragraph occurrence, joins it to the selected child box,
and finds any interior paragraph results through the existing parent-atomic
association. Another paragraph can also contain ID 1.

**Bridge gap:** the result contains no border size, content-edge offsets, box
paint or child interior. The external binding must retain the geometry from the
same selected layout, not a later refit or a lookup by bare atomic ID. It must
also specify the authoritative placement path: compose the precise local anchor
with the parent transform, or use an already-composed page child origin under a
defined policy. Adding both translates twice. Current pagelayout placement
conversion goes through CSS `float32`, whereas tytui demonstrates preserving
the precise local anchor; this precision/authority question is still open.

Child interiors must be reached once per selected occurrence. Following an
anchor and independently traversing the same child through a flat group list
would duplicate it. Material traversal supplies associations, not the complete
CSS painting order.

Sources: [atomic geometry and placement](../../typography/glyphing/paragraph/atomic.go),
[page atomic conversion](../../pagelayout/preparation/typography/fit.go),
[precise nested-atomic composition](../../tytui/documentlayout/proof_fragment_boxes.go),
[negative-advance and duplicate-interior checks](../../tytui/documentlayout/proof_fragment_test.go).

## Refined contracts and remaining boundaries

| Item | Assessment | Planning action |
| --- | --- | --- |
| Direction on retained text materials | Included in the refined proposal. | Copy and validate the exact shaping direction, including silent/replacement text; do not reorder again or imply a full bidi model. |
| Glyph/cluster association invariants | Explicit publication requirements in the refined proposal. | Preserve bijection, ownership, inverse membership and source domains, including repeated anchors and silent material. |
| Processed-byte paint mapping | `PaintSources` and immutable `PaintSourceMap` proposed. | Share the mapping across consumers; preserve supported spelling checks, explicit failure and no partial publication. |
| Logical consumption and cross-result semantics | Deliberately outside this result. | Document the outer binding to full source maps, prefix endpoints and selected paragraph occurrences. |
| Atomic child geometry and origin authority | A bridge contract is still required. | Retain selected child geometry, qualify identities, settle transform/precision authority and require one composition per occurrence. |
| Backend font capabilities | Exact instances are retained, but arbitrary faces may lack outline/embedding capabilities. | Keep capability validation and its errors with the consumer; no font reload or substitution in traversal. |

No additional per-glyph advance, outline, character box or child callback is
needed to explain these four local-placement traces. Efficient backend encoding,
hit testing and broader semantic output may have further requirements; they
should not silently enlarge this proposal's contract.

When implementation is authorized, these examples suggest checks for: RTL marks
with repeated anchors; ligatures across source-node/paint boundaries; selected
and unselected soft hyphens, including silent and RTL alternatives; negative or
zero atomic advances; fragments excluding an atom; identical numeric indexes
in different results; and repeated SVG/PDF use after releasing construction
inputs while borrowed fonts remain valid. This review inspects existing code
and checks example arithmetic; it does not report new runtime test results.
