# Selected box inspection across pages and inline interiors

**Status:** Boundary walkthrough, 28 November 2026. Uses the implemented
[SelectedBox API](selected_box_subtree_api_proposal.md). The selected layouts and
coordinates below are illustrative, not generated fixtures. Semantic text and
display-list design remain deferred.

The style-span, glyph-cluster, ordering and related horizontal decisions already
belong to the interplay of `khipu` and `typography`. Their existing behavior is
authoritative for rendering. Differences from strict CSS inline painting are
accepted for the current practical scope; the renderer does not reopen them.

## 1. One container continued onto a second page

Let C be a source branch-root container with one long paragraph group G. Its
selected output spans two pages. Let `f1` and `f2` be the already-composed frozen
fragments obtained from their respective `PageFrame.Fragments()` calls. Assume
the second fragment completes C and contains an inline-block A that falls beyond
the first page's selected paragraph lines.

The lookups are:

```go
c1, ok1 := f1.Selected(C)
c2, ok2 := f2.Selected(C)
d1, _ := c1.Container()
d2, _ := c2.Container()
```

Both lookups succeed. Because C is the branch root, each also describes that
fragment's `SelectedRoot()`. For a nested C, lookup is the same but the selected
root would instead describe its enclosing branch.

| Observation | Page 1 selection | Page 2 selection |
| --- | --- | --- |
| `Box()` | C | The same qualified source Box C |
| `Container().First / Last` | `true / false` | `false / true` |
| Effective top/bottom edges | Top retained, bottom omitted | Top omitted, bottom retained |
| `Children()` | Selected group G | The same source group G, with a different selected paragraph occurrence |
| `G.Paragraph().ID` | Existing occurrence P1 | Existing occurrence P2, distinct from P1 |
| Paragraph output | Selected prefix lines | Selected remaining lines |
| `G.Children()` in this example | Empty | Selected inline-block A |
| Lookup of A and its interior boxes | False | Succeeds |

Both selections reuse C's source paint. They must use their own effective edges
and geometry rather than reconstructing a complete source box on each page.
Sides remain present under the current fragmentation policy. Repeated inspection
returns the existing paragraph IDs; inspection does not mint new occurrences.

The same G can therefore appear in both views without exposing all its material
on both pages: `Paragraph()` supplies the selected frozen output, and `Children()`
filters its atomic roots by actual selected inclusion. The original prepared
group's atomic list alone is not that selection.

### Coordinates

Assume C has no margins and six CSS pixels of combined border/padding on each
side. The selected geometry has already applied fragment-edge omission and frame
composition. All entries below are origins in CSS pixels:

| Fact | Page 1 | Page 2 |
| --- | --- | --- |
| Receiving frame's page-local content origin F | `(40,50)` | `(60,70)` |
| C's `RelativeTo` | `0` (receiver) | `0` (receiver) |
| C's `Geometry.Frame` origin | `(0,20)` | `(0,0)` |
| C's `Geometry.Content` origin | `(6,26)` | `(6,0)` |
| Outer paragraph's `X/Y` | `(6,26)` | `(6,0)` |
| C's page-local border origin | `(40,70)` | `(60,70)` |
| Outer paragraph's page-local origin | `(46,76)` | `(66,70)` |

Page 1's vertical offset represents earlier material in that receiver. Page 2
uses its own frame origin and selected geometry; it does not add page 1's height
or reuse its opening edges. The paragraph's `X/Y` already includes C's content
offset, so add F once. A nested container instead follows the published
`RelativeTo` chain through ancestor content origins within the same fragment.

### Ownership

C and G share their immutable source snapshot across the views. Each SelectedBox
captures its own frozen fragment selection. Keeping `c1` retains page 1's selected
records even after the caller drops `f1`; obtaining or composing page 2 does not
change those records. Replacing a caller-visible fragment wrapper must likewise
not retarget an existing handle.

The receiving frame descriptor is separate. Keeping `c1` does not implicitly
retain F or identify a page. A caller needing page-local coordinates also keeps
the relevant `FrameInfo` or page/frame context. Preparation and fitter lifetimes
are unnecessary for inspection; existing borrowed-font obligations still apply
when rendering paragraph output.

## 2. An inline-block containing its own paragraph

Within C, consider this structure, with enough preceding text to put the badge
on page 2 in the example above:

```html
<div id="c">Before <div id="badge" style="display:inline-block"><p id="inner">Inside</p></div> after.</div>
```

The inline-block is A. The explicit inner `<p>` produces a block container B,
and its text belongs to paragraph group H. The page 2 inspection view is:

```text
C: outer block container             Container() succeeds
    G: outer paragraph group         Paragraph() succeeds, ID P2
        A: inline-block container    Container() succeeds
            B: inner <p> container   Container() succeeds
                H: paragraph group  Paragraph() succeeds, ID PI
```

For the inner paragraph, `Group == H`, `Owner == B`, and `ParentAtomic == A`.
P2 and PI are separate paragraph occurrences. Bare text directly inside the
inline-block would omit B; the explicit `<p>` must not be collapsed into its
paragraph group by this API.

`f2.Selected(A)` can be reached either by following G's children or by using the
source Box from P2's existing atomic occurrence mapping. Its selected interior
belongs to this same frozen fragment. `f1.Selected(A)` and `f1.Selected(H)` fail,
even though the common source snapshot knows those boxes.

A and B are complete consumed atomic-interior containers in the current profile:
both have `First == true` and `Last == true`. They do not inherit C's omitted top
edge. Their geometry, used edges and source paint supply the `Container()`
result even when they are absent from `f2.Containers()`. PI supplies the complete
selected inner paragraph. An empty interior still leaves A's decoration inspectable.

### Coordinate check using page 2

Continue with F = `(60,70)`. Take these already-resolved origins, all in CSS pixels:

| Box | `RelativeTo` | `Frame` origin | `Content` origin |
| --- | --- | --- | --- |
| C | Receiver | `(0,0)` | `(6,0)` |
| A | C | `(40,12)` | `(44,16)` |
| B | A | `(0,5)` | `(2,7)` |

`Frame` and `Content` are both expressed in the parent's content coordinates.
They are not two offsets to add together. Following the coordinate chain gives:

- A's border on the page: F + C.Content + A.Frame = `(106,82)`.
- B's border on the page: F + C.Content + A.Content + B.Frame = `(110,91)`.
- The inner paragraph's published receiver-relative `X/Y`:
  C.Content + A.Content + B.Content = `(52,23)`.
- The inner paragraph's page-local origin: F + `(52,23)` = `(112,93)`.

The inspection parent of A is G, but A's coordinate parent is C. Likewise, the
inner paragraph's `X/Y` already includes the outer container, atomic placement
and inner block's content offset. Do not add P2's origin, A's anchor or those
container offsets again when using that published paragraph origin.

Converting `(112,93)` CSS pixels gives `(84,69.75)` big points for paragraph
placement. Paragraph-local typography output is already in big points and keeps
its existing offsets. This numerical example does not choose between the existing
CSS-summary and precise local-anchor composition paths; their precision contract
remains open, and either path must apply each offset once.

## Contract checks and limits

The seven methods cover these cases without additional glyph data or
a new parent accessor. The contract provides these guarantees:

1. **Selection:** group children include selected atomics only; no suffix interiors.
   Container and paragraph records belong to the same frozen selection as the handle.
2. **Stable records:** `Container().Box` and `Paragraph().Group`, when present,
   match `Box()`. Paragraph IDs survive repeated inspection and copied wrappers.
   Composition preserves those IDs while returning new placement data.
3. **Complete atomic interiors:** synthesize inspection records from retained
   geometry/used values where the crossing-container list has no entry. Preserve
   the interior's own edges and completeness.
4. **Coordinate scope:** resolve raw `RelativeTo` IDs only within this fragment's
   geometry/tree. A fragment-local index using each Box's existing `Info().ID` is
   sufficient; no global BoxID lookup or inspection-parent arithmetic is implied.
5. **Lifetime:** selected handles retain their immutable fragment backing, while
   the caller retains frame context and required font resources separately.

These are inspection guarantees. Reaching a subtree through both `Children()` and
an atomic lookup does not authorize painting it twice. Existing paragraph/atomic
traversal remains authoritative; this API neither changes that order nor performs
painting itself.

The walkthrough checks the contract against the existing
[fragment records/freezing](../../pagelayout/content/fit.go),
[fragment-edge geometry](../../css-box-layout/layout/fit_geometry.go),
[atomic placement](../../css-box-layout/layout/atomic.go),
[composition and identity regression](../../pagelayout/content/compose_test.go),
[selected atomic-interior regression](../../pagelayout/preparation/typography/fit_test.go)
and [frozen lifetime checks](../../pagelayout/preparation/typography/fit_lifetime_test.go).
The [selected-box tests](../../pagelayout/content/selected_test.go) exercise these
selection and ownership contracts with executable layout fixtures; their numerical
fixtures are separate from the illustrative coordinates above.
