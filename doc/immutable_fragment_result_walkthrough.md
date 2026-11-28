# Walking the spike API through four cases

**Status:** Boundary walkthrough, 28 November 2026. This follows
[REMARKS.md](REMARKS.md) and the
[slim API](immutable_fragment_result_api_proposal.md).
The numeric examples remain illustrative. The boundary now has implementation
coverage in [fragment-result tests](../../typography/integration/fragment_result_test.go),
including real fonts, source-dependent paint, atomic anchors and ownership.
It does not select a display-list design.

The result retains final glyph positions, fonts/colors, logical lines and atomic
anchors. Source strings, replacement spellings, direction, logical ordinals and
cluster mappings are temporary construction data. Semantic PDF output is deferred.

## 1. RTL text with a combining mark

Use logical text `אְב`: ALEF, SHEVA, BET. Assume a retained RTL text piece
with logical glyphs `[alef, mark, bet]`, scalar anchors `[0,0,2]`, and an
ordering permutation `[2,1,0]`. The three source scalars occupy UTF-8 bytes
`[0,2)`, `[2,4)`, `[4,6)`.

For illustrative geometry, use initial X=10, baseline Y=20, scale=0.01,
logical advances `[600,0,500]` font units and mark offsets `(200,200)`.
The retained glyph table is:

| Physical index | Glyph label | Final X/Y | Retained association |
| --- | --- | --- | --- |
| 0 | bet | `(10,20)` | Its sized font and resolved color. |
| 1 | mark | `(17,18)` | Its sized font and resolved color. |
| 2 | alef | `(15,20)` | Its sized font and resolved color. |

These labels stand for actual GIDs from the chosen shaper. No ordering is done
again at replay, and the X values are not sorted. The next knot begins at X=21
when its retained preceding knot advance is 11 big points.

If colors vary, temporary mapping converts bet's scalar anchor 2 to processed
byte 4; the repeated anchor-zero glyphs map to byte 0. Colors travel with their
glyphs through ordering. If all glyphs use the default color, no source mapping
is needed.

After construction, direction, anchors, the permutation and source text can be
released. The result has no `LogicalIndex`, `ScalarAnchor` or cluster accessor.
This retains the placement and paint behavior needed for SVG/PDF without a
semantic bidi model.

Evidence: [ordering semantics](../../opentype/otshape/visual_order.go),
[repeated-anchor check](../../typography/integration/render_test.go),
[paint mapping](../../typography/render/paint_sources_test.go).

## 2. A ligature in office

Assume the retained shaping is `[o, ffi, c, e]`. During construction the
ligature corresponds to source bytes/scalars `[1,4)` and one logical glyph.
That association is useful for paint selection, not a reason to retain the
string `ffi` or a source-span object in the drawing result.

The result contains one GID, exact font, final origin and one foreground color
for the ligature. A color boundary inside the ligature follows the existing
anchor-based one-color policy; it does not create independently paintable
characters or trigger reshaping.

The temporary ordering association must preserve that glyph once, even if
other glyphs share its GID or anchor. No public cluster membership graph, caret
positions or per-character geometry is needed. Text extraction is not a spike
acceptance criterion.

Evidence: [frozen snapshots](../../khipu/frozen_result.go),
[tytui RTL/ligature experiment](../../tytui/documentlayout/proof_fragment_test.go).

## 3. A selected soft-hyphen replacement

Use `co\u00ADoperate`, broken into `co-` and `operate`. The original
paragraph occupies 11 UTF-8 bytes. Construction sees these temporary domains:

| Selected spelling | Original paragraph partition | Local generated domain |
| --- | --- | --- |
| `co-` | bytes `[0,4)` | bytes/scalars `[0,3)` |
| `operate` | bytes `[4,11)` | bytes/scalars `[0,7)` |

For nonuniform paint, the shared soft-hyphen policy maps the generated hyphen
to original byte 2. The continuation's first generated scalar is zero, but
its processed paint byte is 4. The helper must not confuse replacement-local
anchors with paragraph byte offsets.

Each result keeps the selected glyphs, positions, fonts and resolved colors.
Neither retains the original paragraph, the generated spelling, a replacement
flag or a paint-source map. The visible hyphen appears only in the selected
pre-break result; retained layout decides that, not the presence of U+00AD in
a string.

A silent replacement has no output glyph/color entry. Its frozen advance still
affects later positions. Uniform-color rendering can also handle an otherwise
unknown replacement spelling without needing a semantic mapping. If varying
paint requires an unsupported source mapping, construction fails explicitly.

This differs from the earlier retained-map proposal: validating every glyphless
replacement's spelling is not required merely to publish drawing output.
Original source and consumed-prefix information remain available upstream where
pagination needs them; reconstructing semantic words across results is deferred.

Evidence: [manual-prefix render fixture](../../typography/integration/prefix_manual_render_test.go),
[current mapping policy](../../typography/render/paint_sources.go).
Tytui's document profile currently has narrower hyphenation support than this
independent typography fixture.

## 4. An atomic inline box with negative advance

Use processed input `x\uFFFCy` with an attached atomic ID 1. The source marker
is a construction fact; the result retains the occurrence ID, not its text range.

Suppose the atom's knot pen starts at X=30 on baseline Y=20. A pre-resolved
10-by-8 border box with margins left=-15, right=2, bottom=1 has advance=-3
and retained local border origin `(15,11)`. The next knot pen is X=27.

Assuming `x` produced one glyph, the anchor has `BeforeGlyph=1`. That
boundary records its position between the surrounding glyphs; it is not the
atomic ID or an X-coordinate sort key. Consecutive atoms may share the boundary
and preserve their order in the atomic table. A glyphless line can still carry
anchors at its empty glyph boundary.

Construction copies the authoritative border origin from
`Fragment.AtomicPlacements()`; it does not recompute margins. An atom on an
excluded line contributes no anchor. An unattached authored U+FFFC stays ordinary
text rather than becoming a box.

The bridge qualifies ID 1 by paragraph occurrence, joins matching selected child
geometry, and composes the interior once. Its border/content offsets and choice
of precise local-anchor versus already-composed page origin stay outside the
result. The encapsulated paint-order algorithm decides how this occurrence
interleaves with supported box/text painting; its module ownership is open.

Evidence: [atomic layout](../../typography/glyphing/paragraph/atomic.go),
[precise composition](../../tytui/documentlayout/proof_fragment_boxes.go),
[negative-advance checks](../../tytui/documentlayout/proof_fragment_test.go).

## Remaining planning boundaries

| Boundary | Remaining work for the spike |
| --- | --- |
| Resolved foreground input | Adapt page caller paint spans to the implemented simple color input; keep source mapping transient and shared. |
| Page/atomic geometry | Bind exact selected occurrences and choose one transform/precision policy. |
| CSS paint traversal | Define the supported ordering algorithm and its required inputs before choosing its module. |
| PDF fonts | Retained glyph ranges now connect to `docrender/pdf` font pairing and explicit placement. Extend page-level evidence after occurrence composition is defined. |
| Deferred semantics | No new public cluster/source retention or extraction requirement. Existing upstream source ownership remains intact. |

Typography regressions compare GIDs, positions, colors, selected anchors and
lifetime after construction inputs are released. A future bridge check must
verify the composed page occurrences. Folio must preserve final origins even
where its nominal widths differ from shaped advances or layout spacing.
