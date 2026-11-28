# Minimal Go API for selected box inspection

**Status:** Implemented inspection API, 28 November 2026. Refines the
[box-paint context sketch](paint_context_data_sketch.md) under
[REMARKS.md](REMARKS.md). The declarations are implemented in
[pagelayout/content](../../pagelayout/content/selected.go). Paragraph painting keeps
its existing relaxed rules; paint-order implementation, semantic text and
display-list design remain outside this API.

The style-span, glyph-cluster and ordering decisions already made by `khipu` and
`typography` are authoritative. The renderer does not revisit them to recover
strict CSS inline painting; the accepted practical behavior remains unchanged.
The [two-page and inline-interior walkthrough](selected_box_subtree_walkthrough.md)
checks selection, ownership and coordinates for this contract.

The API provides one read-only handle in `pagelayout/content`, obtained from a frozen
`FitFragment`. It combines an existing source Box with that fragment's selected
material and geometry. Existing `Page.Frames()` and `PageFrame.Fragments()` remain
the page-level entry points. No separate public tree object is necessary.

## Declarations

Bodies and private storage are intentionally omitted. All referenced return types
already exist in `content`.

```go
package content

// SelectedBox inspects one box in a particular frozen fragment selection.
// The zero value is invalid. Obtain handles from FitFragment or Children.
type SelectedBox struct { /* private snapshot references */ }

func (f *FitFragment) SelectedRoot() SelectedBox
func (f *FitFragment) Selected(box Box) (SelectedBox, bool)

func (b SelectedBox) Valid() bool
func (b SelectedBox) Box() Box
func (b SelectedBox) Children() []SelectedBox
func (b SelectedBox) Container() (FitContainer, bool)
func (b SelectedBox) Paragraph() (FitParagraph, bool)
```

`Box().Info()` supplies the existing box kind, source paint and formatting facts.
There is no second BoxInfo, new node-kind enum or generic traversal interface.

| Method | Contract |
| --- | --- |
| `SelectedRoot` | Root of this selected inspection view. Nil or material-empty fragments return an invalid handle, even if the existing `Root()` still identifies a source branch. Zero glyphs or zero height alone do not mean empty. |
| `Selected` | Look up a source Box in this fragment's inspection view. Foreign snapshot/tree, excluded boxes and unselected suffixes return false. A source Box present in several fragments resolves to this fragment's occurrence. |
| `Box` | Return the existing qualified source handle. It identifies the source box, not its placement on a particular page. Invalid handles return the zero Box. |
| `Children` | Return an independent slice of immediate inspection children in structural order, under the projection described below. This is not a paint sequence. Invalid handles and leaves return no children. |
| `Container` | Return copied selected decoration/geometry for a block container, including an inline-block. Return false for paragraph groups or invalid handles. |
| `Paragraph` | Return the selected `FitParagraph` for an anonymous inline group; its `Group` equals `Box()`. Copy/clone its payload as existing `FitFragment.Paragraphs()` does. Return false for containers or invalid handles. |

There is no additional error channel: absence is represented by an invalid
handle or false, consistent with existing inspection methods. Inspection performs
no styling, fitting, shaping or backend work.

## The inspection view

Expose selected block containers, anonymous block containers, inline-blocks and
paragraph groups. Ordinary text boxes and inline span wrappers remain inside the
paragraph's completed horizontal output. They do not become inspection nodes.

- A container's children are its selected block/group children in normalized order.
- A paragraph group's children are its selected immediate inline-block roots,
  obtained from the existing group/atomic relationships. Ordinary inline wrappers
  are skipped. Nested atomic interiors remain beneath their own inline-block root.
- Every exposed child belongs to the same fragment selection. No child traversal
  enters an excluded suffix or material merely present in the source snapshot.

If one paragraph group continues across pages, its source Box can occur in both
views while `Paragraph()` returns the distinct existing occurrence for each.
Repeated inspection preserves that ID. Group children are filtered by selected
atomic inclusion, not copied from the complete prepared group's atomic list.

For example, the inspection shape of a paragraph containing a badge is:

```text
paragraph's block container          Container() succeeds
    selected paragraph group        Paragraph() succeeds
        inline-block badge          Container() succeeds
            interior paragraph      Paragraph() succeeds
```

This is a selected projection of existing structure, not a literal copy of every
normalized node. In the current profile each valid inspection node supplies either
a container or a paragraph record. Anonymous block wrappers are preserved where
normalization requires them. Source spans within a paragraph need no nodes or
glyph associations in this view.

Structural child order supports inspection of subtrees. During paragraph painting,
the existing horizontal traversal and atomic anchors determine when to use an
inline child's subtree. Enumerating `Children()` does not introduce a second
logical ordering of glyphs or require painting the interior again.

## Reusing decoration and paragraph records

For containers already listed by `FitFragment.Containers()`, return their retained
`FitContainer` values, including `First`, `Last`, effective edges and geometry.
The new accessor also covers complete consumed atomic interior containers that
are absent from that list. Their geometry/used edges and source paint already
exist; in the current indivisible-atomic profile both `First` and `Last` are true.
This extends inspection coverage without changing the meaning of the existing
container list or allocating a new public decoration type.

For every returned record, `Container().Box` or `Paragraph().Group` matches the
handle's `Box()`. An atomic interior retains its own complete edges even when an
ancestor container continues across pages.

For paragraph nodes, preserve the existing occurrence ID, owner, `ParentAtomic`,
placement and frozen payload. A consumer using `preparation/typography` can call
the existing `InspectFragment(f)`, check its error, and match that ID through
`bundle.Group(id)`. The core inspection API adds no typography dependency.

An atomic's source Box from the existing occurrence mapping can be passed to
`f.Selected(box)`. Its selected parent paragraph remains the authority for the
local atomic ID. This lookup does not make IDs global or expose glyph data.
An empty decorated atomic can yield a valid container with no glyph-bearing child.

## Coordinates and ownership

`Container().Geometry` retains its existing parent-content CSS-pixel convention;
`Paragraph().X/Y` retain receiver-relative CSS pixels. The inspection parent need
not be the coordinate parent: an atomic is an inspection child of its paragraph
group, while `RelativeTo` names the actual coordinate origin. Existing fragment
geometry remains available for resolving that chain. This API adds no transforms.

Use handles from the composed fragments returned by a page frame when inspecting
page placement. A handle obtained before `ComposeFragments` continues to describe
its original immutable placement; it does not follow a later composition.

The handle does not itself retain a receiving frame or page origin. Callers using
page-local coordinates retain the matching frame descriptor alongside it.

The selected handle keeps the frozen fragment and its source snapshot reachable.
It captures that immutable selection when obtained: replacing a caller-visible
fragment wrapper must not retarget or invalidate an existing handle.
No DOM, fitter, continuation or preparation session is required. The existing
private source data and borrowed-font lifetime obligations remain as they are.
Semantic text is neither newly exported nor removed from those snapshots.

Returned child slices and container values are independent copies. Paragraph
inspection preserves the existing payload-cloning contract. No mutable layout
node or internal map escapes. Copying a SelectedBox copies a handle; this API
does not introduce a global or serialized occurrence identity, or an equality
contract based on addresses of copied fragment wrappers.

## Deliberately separate decisions

The selected view supplies structural inputs to box/context painting. It does
not determine stacking contexts or decoration/content phases. No speculative
`ZIndex`, paint-phase or per-glyph scope fields are proposed. Existing box roles
and frame descriptors remain available while the supported context and receiver
policies are planned.

Parent accessors and alternate iterator forms can wait for a demonstrated need;
an inspection walk can carry its ancestor path. The implementation captures a
private fragment wrapper and shares frozen maps, records and the source snapshot.
Children reuse normalized children and existing group/atomic relationships;
inspection creates no second tree or mutable cache.

[Selected-box regressions](../../pagelayout/content/selected_test.go) exercise
continuations, complete and nested atomic interiors, empty decorated material,
anonymous wrappers, excluded boxes, cloned payloads and wrapper replacement.
[Composition checks](../../pagelayout/content/compose_test.go) preserve earlier
placements and paragraph IDs, and the
[typography pairing regression](../../pagelayout/preparation/typography/fit_test.go)
joins selected paragraph IDs to typed output after preparation closes.

Sources: [Box and snapshot ownership](../../pagelayout/content/document.go),
[selected records and freezing](../../pagelayout/content/fit.go),
[immutable composition](../../pagelayout/content/compose.go),
[normalized group relationships](../../css-box-layout/layout/groups.go),
[selected geometry](../../css-box-layout/layout/fit_geometry.go),
[typed paragraph lookup](../../pagelayout/preparation/typography/fit.go) and
[page inspection](../../pagelayout/page.go).
