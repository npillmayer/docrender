# Folio PDF backend boundary for the two-page spike

**Status:** Planning brief, revised 28 November 2026 under
[REMARKS.md](REMARKS.md). Folio is a candidate serialization/font backend;
implementation requires a pinned version and verification of its actual APIs.
This brief selects no display-list design.

## Integration direction

Keep all Folio-specific adaptation in `docrender`. The upstream typesetter
supplies selected GIDs, exact sized font references and authoritative placement.
Folio supplies the backend mechanisms needed to emit the PDF. Its internal font
requirements must not reshape typography or pagelayout contracts.

The working font strategy permits two representations of the same immutable
font binary: one used by the chosen typesetting/shaping path, another built
privately for Folio. `docrender` keeps them associated. Reusing a parsed model
could be an optimization if convenient, but a single parse is not a goal that
justifies exposing parser internals or adding Folio-specific methods upstream.

This supersedes the earlier mandatory `font.Face` adapter/no-reparse approach
and semantic-text requirements. No particular Folio method signature is assumed
to be current.

## Ownership

| Data or behavior | Owner |
| --- | --- |
| Selected GIDs and shaped advances/offsets | User-chosen shaper through the existing typesetter. |
| Final line/knot spacing, baselines and glyph/atomic origins | Layout and shared retained rendering traversal. |
| Resolved foreground and supported box paint | Rendering inputs and the separate paint algorithm. |
| CSS paint traversal | Encapsulated algorithm; module ownership open. |
| Pairing a retained font with its exact binary and Folio model | `docrender`. |
| PDF resource names, used-glyph sets, encoding/subsetting state | `docrender` backend integration and Folio. |
| Serialization and output finalization | PDF backend. |

The backend receives painting decisions and settled geometry. It does not walk
CSS trees, choose paint order, infer document styles or perform text layout.

## Pair the two font representations

At setup or first backend resource use, `docrender` must establish that the
Folio model corresponds to the exact font program used for the retained GIDs.
Use the same immutable binary, with any relevant face selection kept consistent.
Use explicit supported static-font fixtures for the initial experiment.

The existing typography resource capability can provide retained bytes for
compatible faces. For a foreign face, an explicit caller-supplied association
may be required. Generic `FontFace` does not guarantee embedding support.
A missing association is a backend error, not permission to rediscover a font
by family name or substitute another program.

Keep provenance/resource identity, font-program identity and sized-instance
identity distinct. Several sizes can share one program while retaining their
own placement scale. Cache design is local to `docrender`; do not compare
arbitrary interface values or treat a bare GID as globally meaningful.

The second parse consumes the retained binary. It does not require reopening
an original font path during replay. Backend private structures may have their
own lifetime; retained upstream font services remain borrowed and are not
closed by a fragment result.

Sources: [font instances](../../khipu/contracts/font_instance.go),
[resource identity capability](../../typography/font/identity.go),
[immutable resource](../../typography/fontresource/resource.go).

## Preserve authoritative positioning

The chosen shaper selects glyphs and supplies signed advances/offsets.
Layout supplies final knot advances, side skips, baselines and atomic positions.
Shared traversal computes final origins from those facts once.

A backend must reproduce those origins. Folio's nominal widths may be necessary
for its internal PDF font structures, but must not replace shaped spacing,
kerning, mark placement or layout justification. Do not ask Folio to shape or
remeasure the text.

The first experiment can explicitly position each glyph. If a later encoding
batches glyphs or adjusts a text pen, it must be demonstrably equivalent to the
stored origins and use authoritative upstream advances where advances are
needed. Adjacent offset origins alone are not a general reconstruction of
shaping advances.

Coordinate conversion and font-size selection occur once at the PDF boundary.
Positions, font size and scale must refer to the same retained instance. Validate
any narrowing of GID or coordinate representations rather than truncating.

## Verify the backend locally before implementation

Against a pinned Folio version, determine:

- How to register/use the exact selected GIDs for embedding and subsetting.
- How to associate a font resource with a page without invoking text layout.
- How to emit explicit glyph positions and initialize/access output streams.
- Which data the private font model requires and whether supported static
  fixtures provide it.
- Whether glyph-only use requires a local wrapper or small backend extension.
- How resource/document finalization and failures affect retries or output files.

These are verification tasks, not assertions of current API limitations.
If a convenience API couples glyph registration to original text, investigate
its lower-level/backend integration path within `docrender`. Do not introduce
semantic text retention across the pipeline solely to accommodate that API,
and do not fabricate Unicode to satisfy it. Backend suitability is a question
to resolve, not a reason to weaken the chosen shaper's authority.

Folio reference for that future audit:
[repository](https://github.com/carlos7ags/folio).

## Semantic text and richer output are deferred

The spike does not require original-text retention, cluster-based
`/ActualText`, complete `/ToUnicode` semantics, extraction/search/copy
guarantees or tagged accessibility output. Incidental backend metadata is not
a claim of semantic correctness.

The rendering payload consists of positioned glyphs and drawing paint, with
fonts/resources connected locally. Original source data remains upstream only
where existing preparation, pagination or paint construction needs it.

Images, rich compositing, advanced font formats and broad PDF conformance
profiles do not drive this first contract. Unsupported cases should be handled
under an explicit accepted profile; the initial test document can stay simple.

## Evidence expected from the spike

Verify that two selected pages contain the expected glyphs, positions and
foreground/box paint; that selected atomic interiors are composed once; that
the embedded program matches the upstream GIDs; and that repeated font use
has a coherent document resource lifetime.

Use RTL marks, a ligature and spacing that differs from nominal advances to
check placement authority. Keep failures in font pairing, resources and output
distinct from completed glyph-result construction. Source text extraction is
not part of this acceptance check.

See the [pipeline handoff](docrender_rendering_pipeline_folio_handoff.md),
[API proposal](immutable_fragment_result_api_proposal.md) and
[page boundary inventory](pagelayout_painter_boundary_inventory.md) for the
other responsibilities. All remain planning documents.
