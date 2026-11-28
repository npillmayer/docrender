# Immutable glyph fragment result

**Status:** Working direction, 28 November 2026. The public traversal boundary
returns a complete immutable result per selected typography paragraph fragment.
The contents and ownership below are proposed contracts. A
[minimal Go API proposal](immutable_fragment_result_api_proposal.md) now sketches
construction and read-only inspection; no API is implemented or finalized, and
display-list design remains undecided.

The result supplies reusable local glyph placement, font references, source
correspondence and atomic anchors. SVG and PDF consumers can inspect it without
repeating visual ordering, pen accumulation, shaping or line layout. Publication
succeeds for the whole selected fragment or returns an error with no result.

This develops the [traversal boundary sketch](glyph_traversal_boundary_sketch.md).
Here, **fragment means `typography/layout.Fragment`**: one selected line range
from one frozen paragraph owner. A pagelayout `content.FitFragment` can contain
several paragraph occurrences, including atomic interiors; it therefore requires
several such results plus the page integration's associations.

## Proposed contents

These are semantic parts, not a prescribed arrangement of structs and slices.
References between parts are local to the result and must not be reused in a
different result.

| Part | Contents and meaning |
| --- | --- |
| Fragment summary | Original frozen line range, local logical box and baseline presence, and provenance scope for original indexes. Coordinates remain fractional big points with Y downward. Logical bounds are not ink bounds. |
| Source context | Immutable processed paragraph text and a description of its coordinate domain; selected generated replacement strings have separate domains. Page occurrence and decoded HTML source identity remain external associations. |
| Font references | References to the retained sized font instances used by selected text, with the numeric font-unit-to-big-point conversion used to place glyphs. Glyphs refer to these entries. |
| Lines | Local and original line indexes, top/height/baseline, occupied and target widths, side skips, and references to the selected material in physical knot order. No restacking instructions. |
| Material records | Original knot index, final advance, material kind, original paragraph source range and an association to text, space, fill, silent replacement or atomic material as applicable. Text materials retain their shaping direction, including replacements and silent text. Include nonglyph material that survives in the frozen range. |
| Positioned glyphs | GID, local X/Y including shaping offsets, font reference, line/material association, raw scalar anchor with its domain, and a stable association to the logical shaping glyph/cluster. Iteration supplies physical drawing order. |
| Cluster and replacement records | Logical source ranges and membership in the positioned glyphs, with ordinary and replacement-local coordinates distinguished. Replacement records also retain their whole original-source partition. |
| Atomic anchors | Paragraph-local atomic ID, containing line/material, physical traversal position, source marker range and precise local border origin. Child layout and paint are resolved by the page integration. |

The ordered material view connects glyphs, spaces and atomics without forcing a
consumer to walk the original frozen knots again. A separate logical view relates
clusters and source ranges. Neither is CSS page paint order.

Ordinary unsplit paragraphs can use the same result contract over their complete
selected line range. A valid empty range produces an empty result with no
baseline, glyphs or atomic anchors; it does not draw the diagnostic frozen owner
or imply that the source paragraph itself was empty.

## Ownership contract

| Object | Ownership and lifetime |
| --- | --- |
| Placement, line, material, cluster and anchor records | Owned by the result as a completed immutable snapshot. Construction must copy temporary ordering buffers and all mutable nested mapping data it retains. |
| Public inspection values | Scalars may be returned by value. Slice access must return independent copies or use another explicit read-only inspection API; no writable backing arrays or maps escape. |
| Processed and replacement text | Retained as immutable text storage. Sharing stable strings is acceptable; borrowing mutable input buffers is not. Ranges keep their declared domain rather than silently rebasing to a suffix. |
| Font instances | Strong references keep the face/scale objects reachable, but the result borrows their services and does not acquire ownership of resource closure. |
| Ordering service and build scratch | Used during construction only. The completed result retains neither service callbacks nor scratch buffers. |
| Optional paint-source lookup | Owns immutable processed-byte anchors indexed by physical glyph index and retains its exact fragment result. It adds no paint spans, font-service ownership or working layout state. |
| Preparation, fitter, trial and pagination state | Not retained. Keeping original index provenance must not require keeping execution alive. |
| DOM/source lookup, paint spans and page/atomic bindings | Supplied and retained separately by the integration while needed. They are not hidden callbacks captured by the glyph result. |
| Outline caches and backend registries | Owned by their consumers. No paths, PDF resource names, used-glyph registries or mutable backend objects enter the result. |

The proposed storage target is detached **selected records**. Keeping source text
alive may retain the complete processed paragraph, but should not require a
backlink to the full frozen shaping arena merely to resolve a text range. If a
shared source view would retain that arena, make the tradeoff explicit before
adopting it. Exact text storage and provenance-token implementation remain open.

Inspection of stored numeric/source facts requires no font queries. Dropping a
result releases its references; it never closes a face or invalidates another
result. Concurrent read-only inspection is compatible with immutable records.
SVG/PDF service concurrency remains governed by those services, independently.

## Font references

Each font entry denotes the exact existing `contracts.FontInstance` associated
with the selected shaping records. It preserves the opaque face, physical em
size and retained scale. Capture the finite positive numeric conversion used
during construction so downstream consumers can scale outlines consistently
without rebinding the face or recomputing positions.

References are result-local handles, not font-family names, file paths or PDF
resource names. A GID is interpreted together with its referenced face. Two
glyphs with the same numeric GID but different font entries need not represent
the same outline.

Sharing a font entry within a result is allowed when the instance relationship
is known. Deduplication is not required for correctness. Do not assume arbitrary
`FontFace` implementations are comparable, or merge sized instances by family
name or content hash. Cross-result font-resource caching belongs to consumers
with an explicit identity/capability policy.

The result promises usable retained placement facts, not universal embedding or
outline support. `font.ResourceFace` is an optional capability. An SVG or PDF
consumer may need to validate it or use an explicitly supplied adapter, and may
reject a face it cannot use. Such rejection leaves the result intact. The
traversal does not load fonts, select fallbacks, extract outlines, obtain embedding
descriptors or assign resource identities.

Current `FontInstance` binds size only. This sketch does not invent variation or
synthetic-style identity absent from the retained contract. Those would need
explicit upstream representation before a backend could rely on them.

Sources: [font instance](../../khipu/contracts/font_instance.go),
[opaque face contracts](../../khipu/contracts/font_api.go),
[optional resource identity](../../typography/font/identity.go).

## Source and cluster correspondence

### Coordinate domains

Preserve three distinct domains:

| Domain | Available within the result | External association |
| --- | --- | --- |
| Processed paragraph | UTF-8 text, byte ranges and separately identified scalar ranges/anchors. Ordinary shaping clusters address this domain. | Pagelayout identifies the source group and placed occurrence. |
| Generated replacement | Exact selected replacement text and its local byte/scalar coordinates, linked to the original paragraph partition represented by that replacement half. | A supported mapping policy can associate individual replacement anchors with processed paint; arbitrary spelling changes do not imply such a mapping. |
| Original decoded HTML | Not owned or reconstructed by this result. | `Input.TextSources`, `FragmentGroup.Coverage` and a retained source lookup connect processed bytes to original nodes and decoded bytes. |

Original line/knot indexes belong to the particular frozen owner from which this
result was built. Continuation fragments may have different frozen owners while
sharing a processed paragraph. The page bridge maintains that relationship; text
equality, `SourceOffset`, a bare knot index or equal GIDs cannot establish it.
The result's provenance scope must qualify its indexes without becoming a public
resumption token or forcing a dependency on pagelayout identities.

### Physical glyphs and logical clusters

Each positioned glyph retains an unambiguous link to its original shaping
ordinal and cluster membership within the associated text material. Reordering
must carry that association along. `FrozenCluster.FirstGlyph/GlyphCount` describes
the original copied glyph slice; those ranges cannot be used unchanged against
a visually reordered slice.

For example, one ligature glyph for `ffi` can associate with processed bytes
`[1,4)` in `office`. That range remains one cluster even if its text came from
several HTML nodes. Conversely, a base plus marks can have several glyphs sharing
an anchor. Preserve membership without treating the anchor or GID as a unique
glyph identifier. Do not require every logical cluster to map to a contiguous
interval in the physical glyph view; the representation must support the actual
ordering association.

The [API proposal's cluster invariants](immutable_fragment_result_api_proposal.md#cluster-invariants)
make the permutation, unique material ownership, exact inverse membership and
source-domain validation publication requirements. Retained direction is
inspection metadata; it does not ask consumers to reorder again or provide a
complete bidi/reading-order model.

Record scalar anchors and byte ranges with distinct meanings. A raw `Cluster`
number is not an offset into a UTF-8 string. If a helper provides a processed-byte
paint anchor, distinguish that derived value from the raw shaping anchor and
from a full cluster range. Availability must be explicit; zero is a valid offset,
not an unknown sentinel.

### Replacements and material without glyphs

A selected pre/post-break replacement has two associations: its clusters address
generated text, while the frozen knot records the original paragraph partition
it represents. Retain both. A visible hyphen inserted for an authored soft hyphen
does not establish a general per-character mapping for all generated text.
The [proposed shared `PaintSources` helper](immutable_fragment_result_api_proposal.md#shared-paint-source-mapping)
returns a complete immutable lookup for one result, using the existing supported
policy. Unknown mappings fail explicitly without changing the placement result.
The helper validates selected text materials, including silent replacements,
and returns no partial map. Each valid glyph index maps to a processed-byte
anchor; zero is a valid value, while failed mapping is a construction error.

Material can consume or represent source without producing a glyph. Preserve
retained spaces, silent replacement records and generated-fill distinctions.
Generated fill has no authored text. An empty glyph list must not be interpreted
as no source material.

There is also a limit to what traversal can recover: trimmed spaces and other
omitted source may be absent from the frozen material. The result describes its
selected retained material; it does not promise complete consumption coverage.
Pagelayout's fitted-prefix endpoints and source coverage remain available in the
outer association. If a later standalone semantic consumer needs complete
consumption in this result, define an explicit neutral input for that information;
do not infer it from the first/last glyph or import `glyphing/paragraph` into
`render` to obtain it.

These mappings permit later semantic output. They do not yet select `/ActualText`
spans, whitespace recovery or reading order across page frames and channels.

Sources: [frozen shaping provenance](../../khipu/frozen_result.go),
[prefix coverage limits](../../typography/glyphing/paragraph/prefix_coverage.go),
[paint mapping](../../typography/render/paint_sources.go),
[pagelayout source joins](../../pagelayout/preparation/typography/fit_sources.go).

## Atomic anchors and child ownership

An anchor represents a selected atomic occurrence in this paragraph. Its ID is
paragraph-local. Pair it with the external paragraph/occurrence association when
looking up a child; the same numeric ID can appear in another group.

The anchor's X/Y is the authoritative retained **local border origin**, in
fractional big points. Copy it from `Fragment.AtomicPlacements()` and associate
it with the corresponding `FrozenKhipu.Atomic(knot)` entry. The knot's advance is
needed for the surrounding pen; it is not a replacement for the border origin.
Zero or negative advances can yield coincident or decreasing X coordinates, so
preserve physical traversal order rather than sorting by X.

The result owns no child CSS tree, child glyph result, image or drawing callback.
The page bridge binds the anchor to the consumed box and its interior paragraph
occurrences through `GroupInput.Occurrences` and `ParentAtomic`. Those child
results are composed separately. Authored U+FFFC remains ordinary text unless
there is an explicit retained atomic occurrence.

Validate one matching anchor record for each selected atomic knot and no anchors
from excluded lines. The bridge then ensures each included child interior is
painted once for that occurrence. A separate flat pass over all child groups
must not duplicate the interior already reached through its anchor.

Pagelayout publishes already-resolved child origins in CSS `float32`; tytui's
experiment can instead preserve precise atomic placement and apply content-edge
offsets. The result retains precise local anchors so information is not lost at
this boundary. Choosing the page-level authoritative transform remains a separate
contract; adding both origin paths would translate the child twice.

Sources: [retained atomic placement](../../typography/layout/vlist.go),
[fragment validation](../../typography/layout/fragment.go),
[source atomic occurrences](../../pagelayout/content/extract.go),
[tytui atomic composition](../../tytui/documentlayout/proof_fragment_boxes.go).

## Publication and consumer invariants

Before publication, validate associations and ranges, matching selected lines,
valid font references, finite positions and scale, glyph/cluster membership,
and the atomic correspondence. Retained processed text must come from the
matching frozen source or an explicitly validated source binding; equal text
alone does not establish document identity. All-or-nothing publication leaves the frozen
input unchanged. Errors preserve their cause and the relevant line/material
context. A consumer's later failure does not mutate the result.

The proposed result is complete enough to replay local glyph positions and join
their retained source/atomic facts after the original fragment handle is dropped.
It needs no ordering service after construction. Backend resource use still
obeys the borrowed-font lifetime, and original HTML joins still require the
external document association described above.

Planning checks should cover:

- Immutable inspection, including nested mappings and repeated SVG/PDF use.
- Shared faces at different sizes, font switches, repeated GIDs and missing
  optional backend capabilities.
- RTL marks and ligatures, clusters crossing source nodes, replacement-local
  anchors, retained spaces, silent material and valid offset zero.
- Atomic anchors at coincident/backward positions, nested interiors represented
  by separate results, and exclusion of unselected lines.
- Collection of preparations, callbacks and original fragment handles while
  completed results remain usable with their borrowed fonts and source storage.

No changes to the sibling implementations are implied by this sketch. The
[API proposal](immutable_fragment_result_api_proposal.md) offers indexed accessors
and a minimal result-local provenance scope for review; text storage and safe
font-entry reuse remain implementation questions. The selected public delivery
direction is immutable results; any private callback walker is an implementation
choice. Page paint, display-list operations, backend resource handles and
serialization remain separate boundaries.
