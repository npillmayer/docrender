# Reusing retained glyph traversal without outlines

**Status:** Planning sketch, 28 November 2026. A complete immutable result per
selected typography paragraph fragment is the working public boundary. Concrete
Go APIs and display-list representation remain undecided.

The proposed direction is to separate retained glyph placement from proof
decoration inside `typography/render`. Its existing paragraph and fragment proof
builders would consume the same placement logic as a future glyph-based painter.
Outline extraction, font identity lookup and outline bounds would become
consumer capabilities. The page bridge would continue to own occurrence identity,
page placement, atomic interiors and CSS painting order.

This develops the glyph-traversal item in the
[painter boundary inventory](pagelayout_painter_boundary_inventory.md), using
`tytui/tycli` as integration evidence. The intended dependency direction remains
`docrender` toward typography and pagelayout; neither library acquires Folio or
workbench dependencies.

The [immutable result sketch](immutable_fragment_result_sketch.md) develops its
contents and ownership, including fonts, source/cluster mappings and atomic
anchors. The callback comparison below records the tradeoffs behind this direction.
The [minimal Go API proposal](immutable_fragment_result_api_proposal.md) sketches
construction and read-only accessors without implementing that boundary.

## Current coupling

The shared implementation already has a useful division:

| Existing code | Work performed | Proposed treatment |
| --- | --- | --- |
| `BuildParagraphProof` and `BuildFragmentProof` | Validate retained inputs and select local line placements with original frozen line/knot identities. | Preserve the two input cases and share their traversal. A fragment renders only its published range. |
| `appendLine` | Use retained baselines and left skips; visit physical knots; advance through every knot's final width. | Move this responsibility into the common placement traversal. |
| `appendText` | Validate the font instance, identify its resource, map source paint, apply visual order, scale offsets and advances, and construct glyph positions. | Separate placement and raw provenance from identity and paint decoration. Keep one implementation of the positioning arithmetic. |
| `appendGlyph` | Validate the origin, request an outline, extend conservative bounds, and append a proof glyph. | Keep origin validation common; make outlines, path storage and outline bounds part of the proof consumer. |
| `render.Backend` | Combines `VisualOrder` and `Outline`; `Services` also requires an identity provider. | Let consumers request ordering, outlines and identity independently. An adapter may still implement all three. |

Sources: [paragraph proof](../../typography/render/paragraph.go),
[fragment proof](../../typography/render/fragment.go),
[service contracts](../../typography/render/services.go).

## Responsibility sketch

```text
Frozen paragraph or selected fragment + retained placements
                         |
                         v
typography/render: local placement and raw source correspondence
    retained lines → physical knots → ordered glyphs and atomic anchors
                         |
              +----------+----------------------+
              |                                 |
              v                                 v
Existing proof consumer                docrender page bridge and painter
paint + identity + outlines             paint + occurrence/source joins
conservative outline bounds            page origins + atomic interiors
              |                        CSS paint order and decorations
              v                                 |
tycli SVG demo                                  v
                                      Backend-neutral painting boundary
                                                |
                                                v
                                      PDF or other output backend
```

The arrows describe responsibilities. They do not imply materialized arrays,
callbacks, nested drawing operations or any other display-list structure.

| Owner | Responsibility |
| --- | --- |
| Typography layout and khipu | Publish selected immutable lines, knots, glyphs, clusters, font instances and atomic placements. Shaping, breaking and line assembly remain complete before traversal. |
| Typography render traversal | Select retained lines, preserve their indexes, apply directional ordering once per detached glyph fragment, and compute local glyph origins from retained scale/offsets/advances. Preserve raw source associations and atomic positions/order. |
| Typography render paint helpers | Map glyph source anchors to processed-byte paint ranges using the existing supported replacement rules. Provide the same mapping to proof and page consumers. |
| Typography proof consumer | Produce current proof glyphs, resolve proof colors/identity, obtain outlines, and calculate conservative proof bounds. |
| Docrender page integration | Join typography output to pagelayout occurrences, source coverage and resources; apply page/frame origins once; coordinate atomic interiors. |
| Docrender painter | Determine CSS paint order and convert resolved box/text paint into backend-neutral drawing semantics. It does not compute glyph advances again. |
| Output backend and font adapter | Convert to device coordinates, encode glyphs, register/embed resources and serialize output. Font-format capabilities remain separate from glyph placement. |
| Tytui | Exercise these responsibilities with visible demos and regression cases. Its private tree, frame scheduler and proof format impose no compatibility requirement. |

## Minimum information crossing the traversal boundary

The current `render.Glyph` is a proof record: it contains a path and a face hash,
but lacks a retained font capability and full cluster correspondence. The shared
boundary needs the following information, with its eventual grouping left open.

| Information | Why it must survive |
| --- | --- |
| Glyph ID and final local origin | A consumer can draw without interpreting shaping advances again. Positions remain fractional `float64` big points, with Y downward. |
| Retained font instance | The GID's face, physical em size and retained scale remain available. A hash cannot substitute for the face/resource capability. This can be associated once per group of glyphs. |
| Font-unit scale | Outline consumers still need the conversion used by placement. Preserve it consistently with the retained instance; consumers must not select a different size or reconstruct a face. |
| Local line index, original frozen line and knot identity | Fragment-local geometry can be joined to the original horizontal material without replaying an accepted prefix. Qualify these identities by their frozen owner. |
| Glyph-to-logical-source association | Keep raw scalar anchors, cluster ranges and their coordinate domain. Ordinary text is paragraph-relative; generated replacements have local shaping coordinates and separate original-source partitions. |
| Line and knot facts | Retained widths, source ranges and material kind distinguish visible text, spaces, silent replacements and generated fill. Consumers needing semantics must be able to inspect non-glyph material. |
| Atomic occurrence and placement | Preserve where an atomic appears in physical knot traversal and its precise retained border origin. The page layer can associate and paint its interior without treating the atomic as a glyph. |

This boundary can expose a view over retained facts or detached records. It must
not force each glyph to duplicate all font/source metadata. Document-specific
`FragmentID`, frame IDs, HTML nodes, PDF font names, paths and outlines stay out
of the local placement contract.

### Preserve both physical and logical order

Frozen knots are already in physical order. `CopyText` returns a detached glyph
slice in retained shaping order; the current renderer applies `VisualOrder`
once to that slice. Its source metadata must remain associated after ordering.

`FrozenCluster.FirstGlyph/GlyphCount` indexes the original snapshot slice. Those
indexes cannot simply be reused against a reordered slice. A revised boundary
must preserve an association, for example by carrying a logical index with each
ordered item or obtaining an ordering permutation. The choice is open. Matching
by GID or assuming one unique glyph per scalar anchor would lose information for
repeated glyphs and marks.

The current `PaintSources` helper maps anchors to processed-byte color ownership.
Reuse its supported rules; do not reinterpret its one-anchor lookup as a complete
Unicode span. Original source-node joins and logical reading order remain with
the document integration. Trimmed spaces and silent material cannot be recovered
by visiting visible glyphs alone.

Sources: [frozen text and cluster contracts](../../khipu/frozen_result.go),
[shaped glyph records](../../khipu/contracts/shaping_stream.go),
[paint source mapping](../../typography/render/paint_sources.go),
[pagelayout source coverage](../../pagelayout/preparation/typography/fit_sources.go).

## Capability changes

**Ordering becomes independently consumable.** The OpenType rendering service's
`VisualOrder` method delegates to `otshape.VisualOrder` without using its font
parser/cache. A narrow ordering capability can therefore be supplied without
outline access. Keep the adapter dependency in its existing layer; the generic
render package need not import a concrete OpenType implementation. Strengthen
the ordering contract as needed to preserve the logical association above.

**Placement does not require resource identity.** It uses the already-bound
`FontInstance` and its retained scale. It does not call `FontFace.Scale` to bind
again, query line metrics, hash resources or open font bytes. A PDF consumer can
later reject a face it cannot embed; that is distinct from an inability to
position its glyphs.

**Paint decoration remains reusable and separate.** A geometry consumer needs
only retained layout/shaping facts. The immutable result retains the matching
complete processed text and each text material's direction. The
[proposed `PaintSources` helper](immutable_fragment_result_api_proposal.md#shared-paint-source-mapping)
maps that result's physical glyph indexes to processed-byte paint anchors in a
separate immutable lookup. The consumer applies its matching paint spans.
Preserve explicit failure for unsupported generated-text mappings without
invalidating placement. The existing proof's named/hex color validation belongs
with its paint decoration; it should not become an accidental color-format
restriction on all future backends. Pagelayout already supplies checked
`color.NRGBA` values, so the page bridge need not serialize colors to SVG strings
to position text.

**Outlines and bounds are optional consumer work.** The SVG proof consumer asks
for outlines and extends bounds with their control points. A glyph-based consumer
can use local positions without any outline requests. Line allocation bounds
remain available, but an outline-free result must not label them as ink bounds.
Bounds policy must stay explicit for callers that crop previews or size a canvas.

Sources: [OpenType rendering service](../../typography/render/adapter/opentype/render.go),
[font instance](../../khipu/contracts/font_instance.go),
[retained resource capability](../../typography/font/identity.go),
[pagelayout text paint](../../pagelayout/content/extract.go).

## Page composition and atomic interiors

The traversal operates in paragraph-local coordinates. Docrender supplies the
page and receiver placement from frozen pagelayout output. It preserves local
precision and applies the receiving origin once; it does not add ancestor margins
or padding again.

Atomic occurrences need an ordered association to surrounding text, even if the
first consumer merely records their positions. Typography exposes that anchor;
it does not descend into HTML/CSS children. The page layer associates the anchor
with the correct retained interior occurrence and chooses the required paint
sequence. A list containing only glyphs would otherwise force the page layer to
walk knots a second time to recover the missing structural positions.

The existing tytui fragment walker demonstrates the association and guards
against duplicate descent. It preserves precise atomic border origins and adds
content-edge offsets. Pagelayout's interior group origins are already resolved
through CSS `float32` geometry. Before combining those paths, agree on which
placement is authoritative and what precision is promised. This is a separate
page-boundary decision; extracting the shared glyph traversal does not settle it.

Sources: [tytui fragment walker](../../tytui/documentlayout/proof_fragment.go),
[atomic subtree composition](../../tytui/documentlayout/proof_fragment_boxes.go),
[pagelayout placement conversion](../../pagelayout/preparation/typography/fit.go).

## Callbacks and a returned result

Both candidates perform the same synchronous traversal over selected immutable
material. They preserve local positions, logical correspondence and atomic
anchors, and require no live preparation or pagination session. Their difference
is when consumers receive those facts and who owns the storage.

For this comparison, callbacks borrow read-only records for the duration of each
call; a returned result owns completed placement/source records and exposes them
read-only. These are proposed ownership contracts, not automatic properties of
callbacks or Go structs. Giving each callback transferred storage would reduce
the borrowing hazard but change its allocation cost.

| Concern | Synchronous callbacks | Complete returned result |
| --- | --- | --- |
| Publication | A consumer observes records while traversal is still running. | A consumer receives the result only after all selected material succeeds. |
| Slice ownership | Scalars can be copied. Retaining a batch requires copying borrowed slices and any mutable nested metadata. Copying a slice header is insufficient. | Result storage remains stable through later inspections. Private backing storage with copied accessors, or another explicit read-only contract, must protect it from mutation. |
| Font lifetime | Borrowed face/scale services must remain usable during traversal and for anything the consumer retains. | The result retains font-instance handles, whose underlying services must remain usable through later consumption. It does not own or close those resources. |
| Traversal error | Earlier callbacks have already occurred. An error stops further delivery but cannot undo their effects. | Return an error with no result. Private partially built records can be discarded without affecting a consumer. |
| Consumer error | A callback can stop traversal with an error; preserve its cause and line/knot context. | SVG or PDF may fail after successful construction; the immutable placement result remains reusable. |
| Early termination | Natural, if the contract distinguishes an intentional stop from complete traversal. A callback error is not a complete success. | The proposed complete-result operation traverses the whole selected fragment. Selecting a smaller retained range remains a separate input choice. |
| Reuse | Consumers can collect or fan out records during one traversal. Otherwise each traversal repeats placement and visual ordering, without reshaping. | SVG, PDF and diagnostics can inspect the same positioned data without rerunning visual ordering or pen accumulation. |
| Peak placement storage | Can be limited to the current line/knot/batch and source association data if consumers do not collect. Current `CopyText` still allocates detached shaping snapshots. | Storage scales with the selected fragment's glyphs and retained metadata. Font programs need not be copied into every result. |
| Backend storage | Buffering, outline data, semantic spans and PDF resource state may still dominate memory. | The placement result is additional storage; consumers can release it after their required passes. Neither choice requires retaining a whole document. |
| Concurrency | A callback is synchronous; it does not imply parallel traversal, safe reentry or a goroutine protocol. | Immutable records can be shared, but borrowed font/backend service guarantees still govern concurrent use. |

Both forms can accidentally retain more than expected. A closure may capture a
preparation or result owner; a returned source view may keep an entire frozen
paragraph alive even when only a few lines were selected. Lifetime checks should
distinguish required font/source retention from unwanted execution state.

### Error boundaries

There are three distinct completion boundaries:

1. **Placement:** selected glyphs, local geometry and raw correspondence are
   traversed successfully.
2. **Consumer construction:** SVG outlines/bounds or backend painting/resources
   are prepared successfully.
3. **External delivery:** serialized output is written successfully.

A complete placement result makes the first boundary all-or-nothing. It does
not guarantee success at the other two. A late missing outline, unsupported
embedding capability or write failure can still occur.

Callbacks can preserve all-or-nothing proof construction by accumulating into a
private collector and publishing only on success. Directly mutating a backend
during callbacks needs its own discard/commit strategy or an explicit
partial-delivery contract. A resource registry may already have changed even if
no output bytes have been written. Prevalidating inputs does not eliminate later
ordering, consumer or I/O failures.

For example, if SVG outline extraction fails on the last glyph, the current
proof builder must still return no proof. With a complete placement result, the
outline consumer discards only its private proof; placement remains available
for another consumer. With callbacks, the proof wrapper discards its accumulated
records and a retry normally traverses again. Neither approach rewrites the
frozen input or silently resumes at an inferred glyph offset.

If a callback fans out to SVG and PDF, failure in one consumer does not undo
work in the other. Separate consumers of a completed result make independent
failure and retry easier to express. Retrying a PDF consumer still requires
valid backend state; reusing the placement result alone does not reset it.

### SVG reuse

The existing proof builder already accumulates glyph paths, lines and bounds
before publishing. A callback collector fits that implementation, but offers
limited storage savings while those complete proof records are still required.
Streaming SVG also needs a policy for the viewport, whose outline-based bounds
may only be known after traversal: buffer, make another pass, or accept an
explicit viewport supplied by the caller.

A returned placement result lets the SVG consumer resolve paint/identity,
extract outlines and calculate bounds separately. It adds a placement buffer,
but the same local positions can then support inspection or another output
format. Outline failure must not mutate that reusable result. Outline-free
logical bounds remain distinct from conservative outline bounds.

### PDF reuse

A callback can feed the page painter or an intermediate collector incrementally.
It must still respect page paint ordering and source-span decisions; per-glyph
callback order alone does not decide CSS paint order or PDF text spans. Resource
registration and writing are later responsibilities with their own error state.

A returned result supports separate inspection of font requirements, semantic
coverage and physical positions before replay. Those passes do not need to
recompute glyph order. Keeping positions local also permits different page
translations without modifying the result. Resource names, embedding registries
and extraction policy remain outside it.

Either consumer needs access to line/knot facts and atomic anchors, including
material without visible glyphs. That requirement is independent of delivery
shape. It also does not make the placement result a display list: it contains no
general box paint, clipping, composition groups or page-level operation order.

### Working direction

Use a **complete immutable result per selected typography paragraph fragment** as
the working public boundary. It matches existing publication behavior, makes SVG/PDF
reuse straightforward, and keeps placement failure separate from backend
effects. A paragraph-sized result can serve ordinary unsplit inputs. Callers can
release results as they finish; document-wide retention is not implied.

A private callback traversal is compatible with that choice: one collector builds
the result, and proof consumers share the same placement algorithm. Public
callback access can remain an option if measured allocation or one-pass consumer
needs justify its lifetime and partial-delivery contracts. Supporting both public
forms immediately would add contracts before that need is demonstrated.

As the contents and ownership contract develop, measure peak retained memory and
repeated-pass cost on representative selected fragments. Check that consumers can preserve
source/atomic associations, that no execution owners leak, and that font services
remain correctly borrowed. Exact result types, callback granularity and storage
representation remain open, independently of the display-list design.

## Proposed sequence and evidence

1. **Factor the existing proof traversal internally.** Separate line/knot/glyph
   placement from outline decoration, preserving the current proof's geometry,
   colors, source anchors, bounds and failure behavior. Keep one shared path for
   ordinary paragraphs and selected fragments.
2. **Exercise placement without resource services.** A diagnostic consumer should
   receive GIDs, instances, positions and raw correspondence using only ordering
   and retained scale. Prove that identity, outline, font loading, metrics,
   rebinding and shaping are not called.
3. **Resolve source association and atomic output.** Check repeated cluster
   anchors, RTL marks, ligatures, manual replacement domains, silent material and
   selected atomic interiors before exposing a public contract.
4. **Exercise the shared behavior in tycli.** Keep the existing SVG proof as a
   visible consumer. A second experimental consumer can inspect outline-free
   records without committing to PDF or to a display list.
5. **Connect a selected pagelayout page.** Validate occurrence joins, translations,
   atomic precision and source coverage. Then define the smallest public boundary
   justified by both proof and page consumers.

Existing evidence to preserve includes
[retained fragment tests](../../typography/integration/fragment_render_test.go),
[manual-prefix rendering](../../typography/integration/prefix_manual_render_test.go),
[tytui placement proofs](../../tytui/documentlayout/proof_fragment_test.go), and
[directional-order checks](../../tytui/pipeline/visual_order_test.go).
In addition to before/after comparisons, use independent expected origins for
signed advances and mark offsets; compare glyph/source conservation across split
fragments; and ensure empty fragments and late failures publish no partial proof.

This work would establish reusable positioned-glyph output. It would not yet
provide font embedding descriptors, decide `/ActualText` spans, resolve ordinary
inline backgrounds, or define general CSS stacking/compositing. Display-list
nesting, operation types, resource handles and replay strategy remain open.
