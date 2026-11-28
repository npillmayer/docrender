# Box subtrees and stacking contexts in painting

**Status:** Planning walkthrough, 28 November 2026. Follows
[REMARKS.md](REMARKS.md) and the
[minimal box-paint context](paint_context_data_sketch.md). Paragraphs preserve their
agreed relaxed horizontal painting rules. Selected-box inspection is implemented;
paint-order implementation and display-list design remain open, and semantic text
remains deferred.

The examples concern where box decoration, contained paragraph output and nested
box subtrees participate in painting. They do not prescribe new glyph metadata or
source-element-to-glyph associations.

## 1. A decorated block containing paragraphs

Consider a shaded content box containing two ordinary paragraphs. The selected
box subtree provides the outer decoration, paragraph-owning boxes and the placed
paragraph occurrences. Each paragraph supplies its settled horizontal output.

CSS distinguishes box-decoration and inline-content phases; an ordinary block is
not automatically an indivisible paint context. Thus a traversal needs more than
an instruction to finish each entire subtree before visiting the next. For this
example the practical checks are that decoration appears behind its contents,
selected paragraph output is included once, and fragment edges come from the
selected layout. [CSS painting order](https://www.w3.org/TR/CSS22/zindex.html)

The [selected-box API](selected_box_subtree_api_proposal.md) exposes normalized
structural order, resolved paint, selected geometry/edges and paragraph ownership.
The remaining boundary work concerns painting policy. It does not require inspecting
the glyphs to reconstruct logical text order.

## 2. Nested stacking contexts

For illustration, assume these boxes establish stacking contexts:

```text
root context
    A, z-index 1
        C, z-index 999
    B, z-index 2
```

C remains behind B because A occupies a lower position in the root context.
C's level is interpreted within A. Sorting all boxes globally by z-index would
lose that relationship. A stacking context occupies one position in its parent;
ordinary formatting or coordinate containment does not establish that grouping.
[CSS layered presentation](https://www.w3.org/TR/CSS22/visuren.html#z-index)

This illustrates why context membership and nesting matter. General authored
z-index is outside the current content profile, so implementing this extension
is not a prerequisite for the two-page spike. When admitted, the relevant resolved
facts must survive source projection and selected fragmentation. Current page/frame
context policy remains a smaller, separate planning question.

## 3. An inline box with its own subtree

```html
<p>Before <span style="display:inline-block; background:lightgray">badge</span> after.</p>
```

The badge occupies one inline position in the paragraph. Its interior is a box
subtree with a background and its own text; a larger badge could contain several
blocks. Inline-blocks have context-like painting rules, with exceptions for
positioned descendants and actual descendant stacking contexts.
[CSS painting order](https://www.w3.org/TR/CSS22/zindex.html)

For the current normal-flow profile, the practical requirement is to reach that
selected subtree and paint its decoration and contained output once. Keep the
existing horizontal traversal and atomic placement in the surrounding paragraph.
There is no new logical reorder of surrounding glyphs.

The parent paragraph occurrence plus local atomic ID identifies the selected box.
Its interior paragraphs have their own occurrences; interior box decoration must
also remain reachable. A flat list of all paragraph groups must not cause a second
visit to the same interior. An empty badge can still have background or borders.

Placement uses one authoritative path: either the chosen composition from its
local anchor or the already-composed child origins, with the precision contract
made explicit. Adding both would place the interior twice. These are subtree
selection and geometry concerns, not glyph-ordering requirements.

## Remaining planning questions

| Question | Relevant facts |
| --- | --- |
| Which selected boxes belong to this subtree, and in what structural order? | Immutable normalized tree/provenance plus selected fitted membership. |
| Which boxes participate in or establish a paint context? | Box roles and supported CSS/profile rules; receiver attachment policy. |
| Which decoration belongs to this occurrence? | Selected paint, used geometry and effective fragment edges. |
| Which paragraph output and inline child subtrees are contained here? | Paragraph ownership/occurrence IDs, existing atomic references and selected interiors. |
| How are separate frames and routed roots attached? | Published frame/root order, composed placement and a shared receiver paint policy. |

RTL, combining marks, cross-element ligatures and selected replacements continue
to use existing typography behavior. The planning target adds no per-glyph CSS
ownership, paint-piece partition, source coverage requirement or new association
between glyph indexes and source elements.
