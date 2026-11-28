# Painter boundary inventory for pagelayout

**Status:** Planning inventory for the two-page PDF spike, 28 November 2026.
[REMARKS.md](REMARKS.md) governs scope. The typography fragment result and bottom
PDF adapter are implemented; page integration, paint traversal ownership and
display-list representation remain open.

`pagelayout` publishes enough frozen geometry, shaped text and source paint to
start a rendering integration for a deliberately bounded profile. Its output is
a collection of placed fragment occurrences with typed paragraph payloads.
The [rendering handoff](docrender_rendering_pipeline_folio_handoff.md) separates
those facts from the paint traversal that will consume them.
The first planning task is to account for the information a painter needs before
deciding whether an additional representation is necessary.

The immediate boundaries are supported normal-flow paint ordering, selected
occurrence/atomic composition, shared glyph traversal and backend font pairing.
Ordinary inline backgrounds can be excluded from the first fixture while their
geometry contract is unresolved. Semantic text recovery is deferred. These
concerns should not be treated as one missing page-tree API.

## Scope and status terms

This inventory describes the current sibling checkouts. It preserves the
handoff's separation of layout, painting and backend serialization. The existing
[Folio brief](folio_pdf_backend_integration_brief.md) now records the pinned
v0.11.0 audit and first `docrender/pdf` adapter. The page/fragment bridge remains
unimplemented; the backend proof uses fixed rows supplied by its caller.

The tables distinguish available facts, required boundary work and deferred work:

| Status | Meaning |
| --- | --- |
| Available | Published output or an existing explicit service supplies the fact. |
| Derivable | A rendering traversal or coordinate conversion can obtain it without new layout, shaping or CSS resolution. |
| Contract gap | The integration needs a documented policy, an access capability, or additional published facts. This does not prescribe a new API. |
| Outside profile | Current loading/preparation contracts exclude the feature; its absence does not block a narrower renderer. |
| Deferred | Not required for the two-page rendering spike, even if upstream facts already exist. |

Some rows combine statuses because the raw facts are available while their use
still requires agreement. A source paint value alone is not proof that the output
contains the geometry needed to paint it.

## Delivery and ownership

The current access path is:

```text
selected Sequence.Shipout
    → PageConsumer receives *Page
        → Page.Info and Page.Frames
            → PageFrame.Frame and PageFrame.Fragments
                → content.FitFragment geometry, used values and containers
                → preparation/typography.InspectFragment
                    → paragraph occurrences, input paint and source coverage
                    → typography/layout.Fragment
                        → retained lines and khipu.FrozenKhipu
```

This is an access map, not a proposed package graph or display-list structure.
`InspectFragment` is specific to payloads produced by the typography preparation
adapter. It rejects incompatible payloads; a generic `FitPayload` is not itself
a glyph-output interface.

| Painter requirement | Current output | Status and boundary work |
| --- | --- | --- |
| Selected pages in document order | `Sequence.Shipout`, `Page.Number`, `Page.Template`; document results contain selected spans. | **Available.** The caller delivers spans in order and decides whether earlier complete spans may be output after a later failure. The painter makes no pagination decisions. |
| Page size and page paint | `Page.Info()` returns `template.PageInfo` with CSS-pixel dimensions, resolved paint and source. | **Available.** Current page boxes have no effective border or padding. Physical page metadata beyond these dimensions needs a separate backend contract. |
| All receiving frames | `Page.Frames()` includes unused frames, in catalog order. `FrameInfo` contains border/content rectangles, edges, flow, paint and source. | **Available.** An empty receiver can still have background/borders; lack of text is not a reason to skip its paint. |
| Stable output after pagination closes | Frozen pages and paragraphs survive preparation cleanup; fonts and rendering services remain borrowed. | **Available with caller obligation.** Retain the required resource services through output. Do not retain a paginator, continuation or preparation session just to render. |
| Failures and retries | `Shipout` reports completed calls, failed page and next page. Calls are synchronous; failure can leave external effects. | **Contract gap at document-output level.** Successful page callbacks need not mean a PDF has been finalized. Define assembly/finalization and retry behavior separately; calling `Shipout` again starts that sequence again. |
| Stable diagnostic identity | Page number/template, template-local frame ID, qualified `content.Box`, `FragmentID`, source references. | **Available.** Qualify IDs by their owning page/tree/snapshot. Paragraph occurrence IDs are in-memory identities, not serialized identifiers; containers have no equivalent public occurrence token. |

Sources: [page output](../../pagelayout/page.go),
[delivery](../../pagelayout/shipout.go),
[template descriptors](../../pagelayout/template/catalog.go),
[typed fragment inspection](../../pagelayout/preparation/typography/fit.go),
[delivery example](../../pagelayout/shipout_example_test.go).

## Geometry and fragment decoration

| Painter requirement | Current output | Status and boundary work |
| --- | --- | --- |
| Page placement of frames | `FrameInfo.BorderBox` and `ContentBox` are already page-local CSS pixels. | **Available.** `ContentBox` already includes the frame position, borders and padding. |
| Placement of a paragraph occurrence | `FitParagraph.X/Y`, exposed as `FragmentGroup.X/Y`, are relative to the receiving content origin. | **Available.** They already include branch composition and owning-box placement. Add the frame content origin once. |
| Container placement | `Geometry()` and `FitContainer.Geometry` supply `Frame`, `Content` and `RelativeTo`. | **Derivable.** Follow parent content origins within this fragment/tree, then add the receiver origin. Do not use one global map keyed by bare BoxID. |
| Fragment-specific edges | `Containers()` carries `First`, `Last`, resolved `Edges` and source `Paint`; `UsedValues()` carries resolved widths/edges. | **Available.** Continuing containers omit top edges; nonlast containers omit bottom edges. Use published widths rather than recomputing from `BoxInfo.Constraints`. This is the current fragmentation policy, not general `box-decoration-break` support. |
| Backgrounds and borders of consumed atomic interiors | `Geometry()` and `UsedValues()` retain consumed atomic subtrees; `Box.Info().Paint` supplies their source paint. | **Available, traversal needed.** `Containers()` is not an exhaustive list of these boxes. Include retained interior blocks as well as inline-block roots, with each box painted once. |
| Paragraph lines and baselines | `typography/layout.Fragment.Lines()` publishes local top, height, baseline, original line index and frozen horizontal line. | **Available.** Use retained baselines and full line heights; do not restack lines or infer baselines from font metrics. Render only the fragment's line range. |
| Inline atomic placement | `GroupInput.Occurrences`, `ParentAtomic`, `Fragment.AtomicPlacements()` and `FrozenKhipu.Atomic(knot)` connect marker IDs to retained boxes and physical placements. | **Derivable.** Atomic interiors are separate paragraph occurrences whose receiver-relative origins are already resolved. Their local atomic placement must not be added a second time. |
| Precision of atomic interior origins | Typography retains local atomic placements in `float64` big points; the pagelayout adapter converts placements to CSS `float32`, and published group origins also use CSS `float32`. | **Contract gap for precision guarantees.** Tytui demonstrates retaining precise atomic border origins while translating interior geometry. Determine the required precision and authoritative transform path for pagelayout output before adopting that technique; do not combine it additively with the already-resolved group origin. |
| Occupied extent versus ink bounds | `PageFrame.Extent()` measures allocated space; the existing proof's bounds also include outline control points. | **Available but distinct.** Neither allocation extent nor conservative proof bounds is a clipping instruction or a tight glyph-ink bound. |
| Ordinary inline background rectangles | Inline source paint is retained; `GroupInput.Paint` supplies only text foreground. Frozen lines have no per-inline-box background fragments. | **Contract gap.** Solid backgrounds can be accepted on ordinary inline elements, but the published output does not directly supply their per-line extents. Decide how that geometry is owned/published, or explicitly bound the first renderer's accepted input. Do not approximate it with glyph ink bounds. |

Sources: [neutral fragments](../../pagelayout/content/fit.go),
[frame composition](../../pagelayout/content/compose.go),
[source boxes and paint](../../pagelayout/content/document.go),
[group input](../../pagelayout/content/extract.go),
[core fragment geometry](../../css-box-layout/layout/fit_geometry.go),
[retained paragraph fragments](../../typography/layout/fragment.go).

### Coordinate contract to carry into planning

For a paragraph occurrence, let `F` be the frame content origin in page CSS
pixels, `G` its `FragmentGroup.X/Y` in receiver-relative CSS pixels, and `q` an
already-positioned glyph origin in paragraph-local big points. Then:

```text
page origin in CSS pixels = F + G + (4/3) × q
page origin in big points = 0.75 × (F + G) + q
```

These equations describe existing unit relationships, not the units of a future
display list. Paragraph big points are already physical 1/72-inch units. The PDF
backend still owns any conversion to its axis convention. Applying `0.75` to an
already-physical glyph offset would scale it twice.

Container rectangles take a different path: start with the container's own
`Geometry.Frame`, sum the `Geometry.Content` origins named by its `RelativeTo`
chain, and add `F`. Paragraph `G` has already accumulated that ownership path.
Neither path adds source margins or frame padding again.

## Paint and ordering

CSS paint traversal is an explicitly encapsulated algorithm; its module owner
is not selected. The tables describe facts and responsibilities, not an assignment
to a particular painter package. The initial accepted print profile may be
narrower than the full set of paint values admitted by upstream layout.

| Painter requirement | Current output | Status and boundary work |
| --- | --- | --- |
| Resolved solid colors | Template/content paint uses non-premultiplied `color.NRGBA`; text foreground uses processed-byte `TextPaintSpan` ranges. | **Available.** Preserve alpha and source styles. Text does not inherit receiving-frame styles. Color alpha is relevant even though CSS element opacity is outside the profile. |
| Solid border geometry | Frame border widths and fragment used edges accompany per-side paint. None/hidden have zero effective width. | **Derivable.** The paint algorithm constructs supported border areas from these facts. Its module remains open; the first fixture need not exercise every border/alpha case. |
| Deterministic paint order | Frames and branch fragments have published order; paragraphs have occurrence order; geometry has coordinate parents. | **Contract gap.** These are not a complete interleaved paint traversal. Establish the supported ordering of block backgrounds, inline text, atomics and their interiors, including overlap from signed margins or glyph overhang. Never use Go map iteration or numeric BoxID sorting as paint order. |
| Atomic placement among surrounding text | Physical frozen knot order identifies each atomic marker; occurrence IDs and `ParentAtomic` connect interiors. | **Derivable with an ordering contract.** The information is richer than a flat list of all paragraph groups. Painting all groups consecutively does not by itself establish the correct atomic/background interleaving. |
| Source ancestry without recascade | Qualified boxes retain source identity/context; the retained `content.Document` exposes `NodeSource`. | **Available source facts; incomplete paint traversal.** Original ancestry is distinct from fragment/receiver placement. Source inspection does not authorize restyling or rebuilding layout. |
| Frame clipping and overflow | Frame rectangles and allocation bounds exist; no resolved clip payload is published. Authored clipping/overflow effects are outside the current profile. | **Outside profile / explicit policy needed.** Flowing into a frame alone does not request clipping. Do not crop glyph overhang to allocation bounds by default. |
| Transforms, stacking levels, group opacity and blending | Current source/template profiles do not publish these effects. | **Outside profile / deferred.** These do not drive the spike's retained result or traversal interface. Broader support would need its own resolved input contract. |
| Images, rich backgrounds, text decoration, outlines, rounded borders and generated content | No corresponding render payloads in the supported bridge. | **Outside profile / deferred.** These are not prerequisites. Do not manufacture image/resource handling from an inline atomic; current atomics are laid-out inline-blocks. |

The current normal-flow profile reduces the ordering problem but does not prove
that “draw all rectangles, then all glyphs” is sufficient. The
[shipout SVG proof](../../pagelayout/shipout_proof_test.go) is useful evidence for
coordinates and retained outlines; it explicitly identifies itself as a test
utility. Its loops do not define a production CSS paint contract.

Sources: [content admission](../../pagelayout/content/profile.go),
[source paint projection](../../pagelayout/content/project.go),
[template admission](../../pagelayout/template/profile.go),
[content profile details](../../pagelayout/doc/CONTENT-LOADING.md),
[template profile details](../../pagelayout/doc/TEMPLATE-LOADING.md).

## Glyph drawing and deferred semantic text

Upstream source facts remain necessary for existing pagination and for temporary
foreground resolution. They do not imply a retained semantic-text contract for
rendering. The spike publishes positioned glyphs with resolved color.

| Painter requirement | Current output | Status and boundary work |
| --- | --- | --- |
| Exact selected glyphs and font instances | `Fragment.Frozen().CopyText(knot)` returns copied glyphs, clusters, shaping context, font instance and replacement facts. | **Available.** GIDs are `uint32`, meaningful only with the retained face. Any backend using narrower codes must validate before conversion. |
| Final glyph origins | Frozen lines supply side skips; knots supply final widths; shaped glyphs supply signed font-unit advances/offsets and retained scale. | **Derivable.** Traverse physical knots, advance through glue/fill/atomics, order detached directional glyph fragments once, then apply offsets and scale. This replays settled placement; it performs no new line layout. |
| Bidi visual order | Frozen knots are physical left-to-right; `FrozenText.Glyphs` retain logical shaping order and direction. `render.Backend.VisualOrder` supplies the existing conversion. | **Available service, integration boundary open.** Preserve construction-time glyph associations; do not reorder lines/knots again or sort by X. Direction and logical source order need not be retained in the drawing result. |
| Foreground selection across clusters | `GroupInput.Paint`, retained anchors and `render.PaintSources` associate glyphs with processed text paint. | **Derivable within current mapping support.** Resolve colors while source/replacement facts are available, then retain colors only. Uniform default paint needs no source mapping. |
| Glyph traversal without outlines | `render.BuildFragmentProof` already computes origins, paint and order; `tytui/tycli` exercises it through documentlayout's frame proofs. It also obtains outlines and returns path-oriented proof glyphs. | **Contract gap at the output boundary.** Share the existing placement algorithm and publish exact fonts, positions, colors and atomic anchors without paths or semantic spans. |
| Source coverage for ordinary text | `FrozenCluster`, `FrozenKnot.Source`, `FragmentGroup.Coverage` and `Input.TextSources` connect shaping, processed bytes and decoded source ranges. | **Available upstream / deferred as rendering semantics.** Keep existing pagination ownership checks. Use byte/scalar facts temporarily where paint requires them; do not export a new semantic cluster model. |
| Original decoded source strings | `Input.Text` retains processed group text. `content.Document.TextSource` exposes decoded original node strings; a page/box has no equivalent public lookup. | **Deferred.** The spike needs no new frozen-page semantic-text capability or extraction policy. |
| Manual replacements and silent material | Frozen records retain generated spelling, local clusters and original source partitions; endpoints track consumption. | **Available for drawing / semantics deferred.** Replay the selected glyphs and final advances. Needed nonuniform paint uses the supported replacement mapping; no retained spelling or consumption model is required in renderer output. |
| Reading order, links and accessibility | Source/group references and placed events exist, but are not a PDF structure tree or annotation list. | **Deferred.** Keep these out of spike acceptance and the small retained result. |

Sources: [frozen lines and text](../../khipu/frozen_result.go),
[shaped glyph contract](../../khipu/contracts/shaping_stream.go),
[fragment proof entry point](../../typography/render/fragment.go),
[retained glyph traversal](../../typography/render/paragraph.go),
[paint source mapping](../../typography/render/paint_sources.go),
[source coverage mapping](../../pagelayout/preparation/typography/fit_sources.go).

Current `pagelayout` output retains enough information to derive final glyph
origins. Shared typography rendering traversal now owns that
derivation; the backend then consumes settled positions. Neither Folio nominal
widths nor a new layout pass may replace the shaper/layout decisions.

The [glyph traversal boundary sketch](glyph_traversal_boundary_sketch.md) develops
reuse inside typography/render, keeping source mapping temporary while resolving
local placement and foreground. Outlines, backend identity and page composition
remain separate. The slim public result is implemented; display-list design is open.

The traversal output is now an immutable result per selected typography
paragraph fragment; the [result ownership sketch](immutable_fragment_result_sketch.md)
details fonts, positions, resolved colors and atomic anchors. A structural
pagelayout fragment can contain several of these paragraph results.

### Glyph traversal experiments in tytui

`tytui` is an experimental integration application. Its `tycli` command exercises
the shared typography traversal in two routes:

| Route | Current implementation | Relevant responsibility |
| --- | --- | --- |
| Ordinary document with `--show-glyphs` or `--hide-boxes` | `tycli.run` → `documentlayout.WriteLayoutSVG` → `BuildProof` → `typography/render.BuildParagraphProof` | The workbench resolves owner-content origins, translates paragraph proofs and attaches source spans. Typography owns local glyph placement, paint lookup and directional ordering. |
| Bounded flow with `--frames` | `tycli.run` → `Document.LayoutFlow` → `WriteFlowSVG` → documentlayout's fragment proof walker → `typography/render.BuildFragmentProof` | Layout selects the fragments first. The proof walker applies `FrameOrigin + Offset`, visits only selected lines and follows their included atomic interiors. Export performs no new layout. |

The document fragment walker validates processed source text against the retained
group, accepts continuation fragments with different frozen owners, and leaves
empty ranges undrawn. Atomic subtrees are reached through retained placements;
they are not visited again through an inline group's structural children. Nested
interior paragraphs use the shared paragraph proof builder. `appendParagraph`
adds the destination once and attaches all source spans covering each glyph's
processed source anchor. These are existing diagnostic associations, not a
requirement to retain semantic text in the spike's new drawing result.

The atomic walker also preserves the precise paragraph border origin and adds
retained content-edge offsets to it. Its negative-advance test deliberately
perturbs an atomic origin by `1/65536` big point to check that CSS summary
narrowing does not lose the placement. Pagelayout's `fitPlacements` currently
converts such origins through `pointsToCSS`; the bridge must explicitly assess
this precision difference. The existing rule against adding an atomic origin
twice still applies.

These experiments narrow the glyph-traversal question to a reusable output
boundary in typography and the page-specific composition around it. They do not
settle CSS paint ordering: the proof aggregates boxes, lines, glyphs and atomics
separately, and ordinary diagnostic boxes are sorted by BoxID. Nor do their
text-fragment rectangles provide ordinary inline-element background geometry.

The bounded flow admits one direct paragraph and fixed-width opaque children;
it is not pagelayout's multi-channel page selection. Its private document tree,
`PositionedFragment` and proof format should be treated as experimental evidence,
not compatibility requirements for a library bridge. Reusable behavior and
regressions belong in the responsible library; tycli can exercise the resulting
integration as a demo.

Sources: [tycli entry point](../../tytui/tycli/main.go),
[CLI profile](../../tytui/tycli/README.md),
[document proof composition](../../tytui/documentlayout/proof.go),
[fragment traversal](../../tytui/documentlayout/proof_fragment.go),
[atomic subtree traversal](../../tytui/documentlayout/proof_fragment_boxes.go),
[source associations](../../tytui/documentlayout/proof_paragraph.go),
[flow export](../../tytui/documentlayout/flow_proof.go),
[pagelayout placement conversion](../../pagelayout/preparation/typography/fit.go).

## Fonts and backend resources

| Painter or backend requirement | Current capability | Status and boundary work |
| --- | --- | --- |
| Sized face and scale | `contracts.FontInstance` exposes `Face`, `EmSize` and retained `Scale`. | **Available.** It binds size only; it does not describe variations or synthetic styling. |
| Exact retained font bytes and identity | Optional `font.ResourceFace` exposes immutable `fontresource.Resource` and metadata; `Resource.Open` reads retained bytes. The current OpenType face implements it. | **Available for compatible faces.** Generic `FontFace` does not promise this capability. Validate it or require an injected resource service; never reopen a source path during replay. |
| Resource identity versus content identity | `Resource.ID()` includes provenance; `SHA256()` identifies bytes. | **Available; cache policy open.** Keep their meanings distinct. A proof's `FaceSHA256` alone is not a complete resource/instance handoff. |
| Embedding descriptors and nominal glyph widths | Generic `FontFace` supplies basic vertical metrics and scale; parsed internals stay private. | **Implemented for the initial static TrueType profile in docrender.** The local Folio model supplies embedding data from retained bytes, with descriptor unit adaptation. No Folio requirements enter upstream contracts or change glyph placement. |
| Separate backend font parse | Retained immutable bytes are available for compatible faces. | **Accepted approach.** Docrender may let Folio parse the same program separately and owns the association with the upstream instance. Avoiding a second parse is not a prerequisite. |
| PDF resource registration and document cache | No PDF resource names or mutable embedding registries occur in page output. | **Implemented locally against Folio v0.11.0.** The adapter registers exact GIDs, shares program resources across pages/sizes and emits absolute origins without source text. Joining actual page output is still bridge work. |

Sources: [font contracts](../../khipu/contracts/font_api.go),
[font instances](../../khipu/contracts/font_instance.go),
[retained identity capability](../../typography/font/identity.go),
[immutable resource](../../typography/fontresource/resource.go),
[glyphing font adapter](../../typography/glyphing/adapter/opentype/font.go),
[outline service](../../typography/render/adapter/opentype/render.go).

## Worked boundary example

Use this hypothetical already-selected page to review the boundary. It is a
coordinate example, not a claim about which line breaks a font will produce.

The page is 600 × 800 CSS pixels. Two main receivers have content rectangles
`(40, 50, 240, 300)` and `(320, 50, 240, 300)`. A notes receiver at
`(40, 400, 520, 100)` is empty but has a visible frame border. A decorated block
and its paragraph continue from the first main receiver into the second. A
consumed inline-block belongs to the first occurrence and has its own text group.

| Observation in selected output | Painter consequence |
| --- | --- |
| The first paragraph occurrence has `G=(12,18)`. A derived local glyph origin is `q=(9,15)` big points. | Its page origin is `(64,88)` CSS pixels, or `(48,66)` big points before any backend axis conversion. |
| The second occurrence refers to the same source group but has a different `FragmentID` and its own local lines/origin. | Render only its published range. Do not restart the full paragraph or share occurrence geometry by source group. |
| The block's first container occurrence is `First=true, Last=false`; the second is `First=false, Last=true`. | Use the first fragment's top edges and the second fragment's bottom edges, with retained side edges on both. Do not restore omitted edges from source constraints. |
| The inline-block has a frozen atomic marker, retained box geometry and an interior paragraph occurrence with `ParentAtomic`. | Associate the interior with that occurrence. When using its published paragraph origin, do not add the local atomic origin again. The bridge chooses one transform path and avoids a second visit through a flat group pass. |
| The notes receiver contains no branch fragments. | Its compiled border still paints. Empty content does not imply an invisible frame. |
| Preparation has closed. | All these inspections remain usable while borrowed font/resource services are valid. No fallback to preparation or shaping is necessary. |

This example deliberately leaves the interleaved painter traversal unspecified.
It supplies the facts that traversal must preserve. Add overlap cases relevant
to the accepted print profile; inline-background variants can wait until that
geometry enters the renderer's scope.

## Evidence and planning sequence

Existing evidence can anchor later bridge checks without treating proof helpers
as export APIs:

| Evidence | What it establishes for this boundary |
| --- | --- |
| [Fragment fitting tests](../../pagelayout/preparation/typography/fit_test.go) | Distinct occurrences, retained paint/coverage, manual continuation and frozen atomic interiors across changed widths. |
| [Real-font document proofs](../../pagelayout/shipout_realfont_test.go) and [sample guide](../../pagelayout/testdata/pagination/README.md) | Selected pages, changing receiver widths, source conservation and retained rendering after execution cleanup. |
| [Delivery tests](../../pagelayout/shipout_test.go) | Selected-only delivery, failure progress, cancellation and empty/decorated material. |
| [Retained fragment tests](../../typography/layout/fragment_test.go) | Local geometry, original frozen line identities and immutable fragment ownership. |
| [Glyph paint mapping tests](../../typography/render/paint_sources_test.go) | Existing processed-source mapping and explicit generated-text limitations. |
| [Immutable fragment-result tests](../../typography/integration/fragment_result_test.go) | Outline-free RTL/ligature/SHY placement, selected anchors, failure/retry and collection of construction inputs. |
| [Tytui fragment proofs](../../tytui/documentlayout/proof_fragment_test.go) | LTR/RTL ligatures and paint/source conservation after translation; nested atomics, negative advances, precise origins, different frozen owners, empty ranges and failure publication. |
| [Tycli frame tests](../../tytui/tycli/frames_test.go) | Equal/changed-width demo flows, glyph/box controls and preservation of existing output on incomplete or invalid flow. |

The next planning steps should resolve the following in order:

1. **Choose the two-page print fixture.** Declare its simple accepted paint.
   Ordinary inline backgrounds and complex effects need not enter this fixture.
2. **Small glyph result implemented.** Shared typography traversal retains final
   positions, fonts, resolved colors and anchors. Paint-source mapping stays
   transient and semantic text deferred; page consumers still need integration.
3. **Resolve occurrence and atomic bindings.** Specify the selected child joins,
   precision and one authoritative transform path.
4. **Encapsulate supported paint traversal.** Trace an interleaved page, including
   relevant overlap cases. Define the algorithm's inputs and invariants while
   leaving module ownership open.
5. **Backend font pairing implemented first.** Docrender connects the same
   immutable font program to its upstream and Folio representations. Glyph-only
   registration, resource lifetime and exact placement are tested against
   v0.11.0. `Page.PaintFragmentGlyphs` now consumes retained glyph ranges with
   explicit translation; selected page composition remains pending.

After those contracts are understood, a small rendering experiment can test
coordinates, occurrence conservation, paint order, resolved colors and resource
lifetime together. It need not freeze the display-list API.

Flat versus nested display lists, operation types, immediate versus retained
replay, graphics-state encoding, resource-handle representation and final package
layout all remain undecided.
