# `docrender`: Rendering pipeline and Folio PDF backend

**Architectural handoff for a Go coding assistant**  
**Status:** Discussion summary, 23 November 2026  
**Scope:** From an already-paginated HTML/CSS fragment tree to PDF output  
**Important:** The **detailed display-list data model is intentionally undecided**. Do not select a flat versus nested representation, commit to particular `Op` Go types, or implement a finalized display-list API based solely on this document.

## 1. Objective and agreed direction

The top-level Go module will be named **`docrender`**. The typesetter is responsible for turning styled HTML into typeset pages. The PDF backend must be a *paint-only* backend: it does not parse HTML, cascade CSS, shape text, perform line breaking, measure text, paginate, or decide where individual glyphs belong.

The intended pipeline is:

```text
HTML / CSS document
    │
    ▼
Style resolution, box construction, layout, shaping, line breaking
    │
    ▼
Pagination into HTML page-template frames
    │
    ▼
Page-specific resolved render/fragment trees
    │
    ▼
CSS-aware painting driver (paint order, stacking contexts)
    │
    ▼
Backend-independent page display list
    │
    ▼
PDF display-list interpreter / resource adapter
    │
    ├── Folio content.Stream     graphics and text operators
    ├── Folio document           pages and PDF resources
    ├── Folio font.EmbeddedFont  glyph tracking, subset, font objects
    └── Folio core               low-level PDF objects
    │
    ▼
PDF file
```

**Agreed architectural principles:**

- Keep **layout**, **painting**, and **PDF serialization** separate.
- Let the CSS painter understand the HTML/CSS fragment tree. Do **not** make the PDF backend understand CSS boxes.
- Use a **display list** between painting and backend output. Its exact representation is still open.
- Reuse **Folio** for PDF content operators, document assembly, and font embedding.
- Reuse the project's **existing OpenType font parser and shaping engine**, adapting it to Folio's existing `font.Face` interface instead of parsing the font again with Folio.
- Prefer **small public API extensions** to Folio's `document.Page` over copying/forking its document-assembly implementation.
- Preserve original Unicode text and glyph-cluster relationships for text extraction.

## 2. Input contract: what rendering receives

At this point the upstream typesetter has already done the expensive typographic work:

- The source has been interpreted as an HTML/CSS render tree.
- Computed/used style values required by painting are resolved.
- Box dimensions, placement, borders, padding, transforms where supported, and geometry are determined.
- Text has been shaped by the project's OpenType shaper. GIDs, glyph origin positions, font instances, and relevant Unicode clusters are known.
- Inline material has been line-broken and placed.
- Page templates contain **fixed-size frames**, i.e. block-like regions into which source document material flows.
- The result for each page is an HTML/CSS **page-specific render tree** (or, more precisely, a *fragment tree*): it contains the template and material actually appearing on that page.

A logical box can be split across pages or frames. Painting must operate on its **page-specific fragments**, not attempt to paginate the original logical box again. Fragment-specific decoration rules (e.g. `box-decoration-break`), clipping and continuation edges should already be known or represented in information available to the painter.

The original source ancestry and CSS inheritance do **not** change merely because document material is rendered inside a page-template frame. The template controls placement/clipping; it should not accidentally restyle the flowed content.

## 3. Tree traversal versus CSS painting order

### 3.1 Generic tree traversal

A simple Go walker is useful independently of rendering. Go interfaces are sufficient; classical inheritance and elaborate double dispatch are unnecessary.

Illustrative, **not a required package API**:

```go
type Node interface {
    Children() []Node
}

type Visitor interface {
    Enter(Node) (descend bool, err error)
    Leave(Node) error
}
```

A recursive driver can call `Enter`, descend into children if requested, and call `Leave` while unwinding. A function callback or Go type switch can be used for simpler inspection visitors. Applications include diagnostics, tree visualization, consistency checks and resource discovery.

**Caution:** Ensure `Leave` is balanced with `Enter` even when a child fails. Return errors with clear context. Avoid building painting rules into a general-purpose tree walker.

### 3.2 CSS painting is not a plain depth-first walk

A naïve depth-first walk paints each subtree as a unit in DOM/box-tree order. CSS stacking and paint order can require a different sequence: background and border of stacking-context roots, negative stacking levels, in-flow boxes, floats, inline content, positioned descendants and positive stacking levels have prescribed ordering. Outlines and other features have additional rules.

Therefore introduce a **CSS-aware painting driver**, conceptually:

```go
// Concept only; this signature is not yet fixed.
func PaintPage(page *PageFragmentTree, sink PaintSink) error
```

The painter is responsible for:

- Following CSS painting order and stacking contexts.
- Resolving which parts of a box are painted in which phase.
- Converting CSS backgrounds, borders, text decoration and related visual features to generic graphics operations.
- Respecting clips, transforms and composition-group boundaries.
- Emitting already-shaped and positioned glyphs without reshaping.
- Emitting the operations in a deterministic order for each page.

The output is a **backend-neutral display list**. It must not contain Folio types, PDF resource names, or PDF operators.

**Do not finalize yet:** Whether the display list is flat or contains nested sublists; whether it uses a Go interface, tagged union, callback events, or another representation; how groups and shared resources are encoded; and whether it is replayed immediately or retained. Those decisions are explicitly postponed.

## 4. Conceptual display-list instruction vocabulary

The following is a **semantic inventory**, not an approved Go type hierarchy or wire format:

| Operation family | Examples | Intended purpose |
|---|---|---|
| Graphics state | Save, Restore | Scoped graphics state |
| Transform | Transform/Concat | Translation, rotation, scale, CSS transforms |
| Clipping | ClipPath, ClipRect | Frame overflow, clipping and rounded corners |
| Geometry | FillPath, StrokePath, FillRect | Backgrounds, borders, rules, decoration |
| Color/style | Fill paint, stroke paint, width, dash | Fully resolved graphical appearance |
| Text | DrawGlyphRun / DrawPositionedGlyphs | Already-shaped glyphs with positions and font reference |
| Images | DrawImage | Raster or supported image representation |
| Text semantics | Original-text association | Extractable text and accessibility |
| Composition | Group / opacity / blend | Isolation and CSS group effects (later) |

For example, a box with a gray background, a stroke, and two text lines might logically paint as:

```text
Save
FillRect(...)
StrokePath(...)
DrawPositionedGlyphs(line1)
DrawPositionedGlyphs(line2)
Restore
```

This illustration is intentionally informal. It makes no decision about display-list nesting or representation. The painter must compute CSS painting semantics **before** PDF replay.

## 5. Folio's package layers

Folio repository: <https://github.com/carlos7ags/folio>

| Package | Role in this project | Should it see our CSS tree? |
|---|---|---|
| `content` | Write operators into a PDF content stream (`q`, `Q`, `cm`, paths, `BT`, `Tm`, `Tj`, `TJ`, `Do`, etc.) | No |
| `document` | Manage pages, page-level resources, object reuse, metadata, PDF file assembly | No |
| `core` | Represent PDF objects such as dictionaries, arrays, indirect references, streams | No |
| `font` | Via `font.Face` + `font.EmbeddedFont`: embed font, subset glyphs, construct font dictionaries and widths, generate baseline `/ToUnicode` | No |
| `layout` | Folio's own layout/text shaping | **Do not use** |

Important boundaries:

- `content.Stream` is essentially a **PDF graphics operator builder**. It does not verify that a name such as `F1` actually denotes an embedded font on the page.
- `document.Page` owns the page's `/Resources` dictionary and assigns resource names (e.g., `F1`, `Im1`).
- `document.Document` and its writer assemble page dictionaries, content streams, font resources and the object graph; `core` provides the low-level objects.
- `font.EmbeddedFont` produces PDF font objects (including a subset) and maintains glyph usage.

Relevant sources:

- [Folio architecture](https://github.com/carlos7ags/folio/blob/main/ARCHITECTURE.md)
- [`content` package documentation](https://pkg.go.dev/github.com/carlos7ags/folio/content)
- [`document/page.go`](https://github.com/carlos7ags/folio/blob/main/document/page.go)
- [`document/document.go`](https://github.com/carlos7ags/folio/blob/main/document/document.go)
- [`font/face.go`](https://github.com/carlos7ags/folio/blob/main/font/face.go)
- [`font/embed.go`](https://github.com/carlos7ags/folio/blob/main/font/embed.go)

## 6. Translation of paint operations into Folio's content operators

This table records **existing Folio `content.Stream` API methods**. Display-list operation names are conceptual.

| Conceptual paint operation | Folio API | PDF operator(s) |
|---|---|---|
| Save graphics state | `SaveState()` | `q` |
| Restore graphics state | `RestoreState()` | `Q` |
| Concatenate affine transform | `ConcatMatrix(a,b,c,d,e,f)` | `cm` |
| Build a rectangle path | `Rectangle(x,y,w,h)` | `re` |
| Build a path | `MoveTo`, `LineTo`, `CurveTo`, `ClosePath` | `m`, `l`, `c`, `h` |
| Fill path | `Fill()` / `FillEvenOdd()` | `f` / `f*` |
| Stroke path | `Stroke()` | `S` |
| Clip path | `ClipNonZero()` or `ClipEvenOdd()` **then `EndPath()`** | `W n` / `W* n` |
| Fill/stroke RGB color | `SetFillColorRGB`, `SetStrokeColorRGB` | `rg`, `RG` |
| Line width/dash | `SetLineWidth`, `SetDashPattern` | `w`, `d` |
| Select extended graphics state | `SetExtGState(name)` | `gs` |
| Begin/end text object | `BeginText()`, `EndText()` | `BT`, `ET` |
| Select font and size | `SetFont(resourceName, size)` | `Tf` |
| Set precise text position | `SetTextMatrix(a,b,c,d,e,f)` | `Tm` |
| Show encoded glyph bytes | `ShowTextHex([]byte)` | `Tj` with hex string |
| Show text with spacing corrections | `ShowTextArrayHex(...)` | `TJ` |
| Draw registered image/XObject | `Do(resourceName)` | `Do` |
| Begin/end original-text span | `BeginMarkedContentActualText(text)`, `EndMarkedContent()` | `BDC` / `EMC` |

Clipping changes the current graphics state and persists until restored. A clip operation must construct a path, call `ClipNonZero()`/`ClipEvenOdd()`, **then terminate the path** with `EndPath()`. Graphics save/restore pairs must be balanced.

Illustrative low-level sequence:

```go
s.SaveState()
s.SetFillColorRGB(0.94, 0.94, 0.94)
s.Rectangle(x, y, w, h)
s.Fill()
s.RestoreState()
```

Folio's `content.Stream` emits PDF operator syntax. The PDF renderer remains responsible for correctly converting geometry/colors, keeping graphics state consistent, and ensuring all resource names exist.

## 7. Page and resource management: what requires an adapter

### 7.1 Simple graphics are largely self-contained

A path, rectangle or line can be emitted using `content.Stream` with no special PDF resource (beyond appropriate graphics state). Images, fonts, external graphics states, patterns, shadings and transparency groups **do** require resources in the page dictionary.

The PDF renderer therefore needs both:

1. An **operation emitter** that interprets painting commands into PDF operators.
2. A **resource manager** that maps backend-neutral font/image/etc. identities to page-specific PDF names and registers appropriate objects with Folio.

These are responsibilities of the PDF backend, not of the CSS painting driver.

### 7.2 Current public API gap in Folio

As of the inspected Folio sources, `document.Page` keeps:

```go
type fontResource struct {
    name     string
    standard *font.Standard
    embedded *font.EmbeddedFont
}
```

Its private method `embeddedFontResource(...)` registers an `*font.EmbeddedFont` and returns a name such as `F1`. The public `AddTextEmbedded(...)` registers the font **as a side effect of drawing text**, which is not suitable when text has already been shaped and positioned externally.

Furthermore:

```go
func (p *Page) ContentStream() *content.Stream
```

currently returns `nil` if the page has no content; the stream initializer (`ensureStream`) is unexported.

**Proposed minimal upstream extension (not existing API):**

```go
// Proposed names/signatures; negotiate with the Folio maintainer.
func (p *Page) RegisterEmbeddedFont(f *font.EmbeddedFont) string
func (p *Page) EnsureContentStream() *content.Stream
```

The first exposes existing internal font registration without drawing text. The second ensures that the content-stream builder is initialized. This retains the existing `document` implementation and the concrete Folio embedded-font type.

A more general `RegisterResource` API is possible, but **is not required for the initial implementation** and should not be designed preemptively. Avoid copying the `document` package just to gain this access.

Images can already be drawn via `Page.AddImage`, which both registers the image and writes graphics operations. When the display-list interpreter requires more control over image transforms or resource reuse, inspect whether a corresponding public resource-registration primitive would be useful.

## 8. Font integration strategy: adapt to Folio's existing `font.Face`

This is a previously agreed decision and remains the preferred starting point.

The project already owns the font binary and has an OpenType parser and shaper. **Do not ask Folio to parse fonts again.** Instead, implement Folio's `font.Face` interface by delegating to the existing font implementation.

Conceptual adapter:

```go
type FolioFace struct {
    font *MyOpenTypeFont // project-specific type; placeholder
}

// Implement all methods required by folio/font.Face, using existing
// metrics and raw bytes. See Folio's actual face.go for signatures.

var _ font.Face = (*FolioFace)(nil)

// After implementing the interface:
embedded := font.NewEmbeddedFont(&FolioFace{font: myFont})
```

`font.Face` exposes font identity, metrics, glyph mapping, raw bytes and additional font-descriptor information. It also includes shaping-oriented facilities (`Kern`, `GSUB`, `GPOS`) not needed for the independent painting path. Methods not consumed on this path can be benign implementations **only after checking Folio's embedding call graph**. Do not mistakenly assume that every method is optional merely because the typesetter shaped the text.

`font.NewEmbeddedFont(face)` wraps the adapter, allowing the existing `document` package to accept the font **without changing `document`'s concrete font type dependencies**. This is a less invasive starting point than adding a generic PDF-font interface.

### Expected additional state and memory use

If the adapter references an existing parsed font and raw `[]byte`, a second OpenType parse and duplicate `cmap`, GSUB and metric structures are unnecessary. `EmbeddedFont` primarily keeps a face reference and a per-document used-glyph map; PDF generation additionally allocates subset/font streams and resource objects. This is considerably lighter than maintaining two parsed font models.

### Embedding caveats

- Use one compatible static font program per PDF font resource. Metrics used during shaping and PDF font widths must agree.
- Verify TrueType `glyf` subsetting/embedding first.
- CFF/CFF2 support and variable-font instances need separate testing; don't assume every raw OpenType binary will work automatically.
- A variable font may require generating a static instance suitable for PDF embedding, with metrics matching the chosen variation coordinates.
- Ensure embedded font use complies with any font embedding rights and license requirements.

## 9. Glyph-level output: no reshaping or line layout

The painter emits a *positioned glyph run* associated with a font and original text/cluster metadata. The PDF backend's only typographic work is encoding GIDs and emitting PDF text operators.

For Folio's Type0 `/Identity-H` model, the encoded character code typically equals the GID and is written as a **big-endian 16-bit value**. The backend must not pass UTF-8 source strings to `ShowTextHex`; those bytes are not glyph IDs.

`font.EmbeddedFont.EncodeGIDs(gids, originalText)` is useful because it both:

1. Produces the 2-byte-per-GID encoding used in `Tj`/`TJ`.
2. Registers the glyph IDs with Folio for font subsetting and PDF widths.

### 9.1 First implementation: position every glyph absolutely

The simplest correct baseline is one `Tm` plus one `Tj` per positioned glyph, all in a text object. This avoids recomputing GPOS adjustments in the PDF layer. It is verbose, but precise and easy to test.

The following **sketch** assumes glyph `X/Y` are final *baseline-origin* coordinates in CSS pixels and ignores additional glyph transforms. It also assumes `fontName` is an already-registered Folio resource name.

```go
// Sketch only; Glyph, sourceText, and coordinate types are project-defined.
const cssPxToPt = 72.0 / 96.0

func emitCluster(
    s *content.Stream,
    embedded *font.EmbeddedFont,
    fontName string,
    fontSizeCSSPx float64,
    pageHeightCSSPx float64,
    glyphs []Glyph, // fields: ID uint16, X float64, Y float64
    sourceText string,
) {
    if len(glyphs) == 0 {
        return
    }

    ids := make([]uint16, len(glyphs))
    for i, g := range glyphs {
        ids[i] = g.ID
    }
    encoded := embedded.EncodeGIDs(ids, sourceText)

    s.BeginMarkedContentActualText(sourceText)
    s.BeginText()
    s.SetFont(fontName, fontSizeCSSPx*cssPxToPt)
    for i, g := range glyphs {
        x := g.X * cssPxToPt
        y := (pageHeightCSSPx - g.Y) * cssPxToPt
        s.SetTextMatrix(1, 0, 0, 1, x, y)
        s.ShowTextHex(encoded[2*i : 2*i+2])
    }
    s.EndText()
    s.EndMarkedContent()
}
```

Real implementation must handle colors, glyph transforms, coordinate conventions, glyph-less text and cluster boundaries. Do not treat this sketch as the final text API. It shows the essential *translation* from positioned glyphs to Folio operators.

### 9.2 Later optimization: `TJ`

For ordinary horizontal text, adjacent glyphs can be packed into `TJ` arrays instead of absolute positioning. With font size `S` in PDF points, PDF nominal advance `aPDF` and required shaped advance `aShape` in PDF points, the horizontal spacing adjustment is conceptually:

```text
TJ adjustment = 1000 × (aPDF − aShape) / S
```

This convention is for ordinary horizontal text (and excludes scaling/transform complications). `TJ` handles progression adjustments, not arbitrary two-dimensional placement. Combining-mark offsets and transformed glyphs may still require `Tm` or another positioning strategy.

**Start with per-glyph absolute positions.** Optimize only when rendering comparisons establish the baseline is correct.

## 10. Unicode semantics: glyph usage, `/ToUnicode`, `/ActualText`

Distinguish two independent concerns:

- **Glyph registration:** records which GIDs must be embedded in a subset. This is required by the current `EmbeddedFont` workflow.
- **Text recovery:** records which Unicode characters the glyphs represent for search, copy/paste, extraction, and accessibility.

A GID is **not** inherently a Unicode character. Ligatures, contextual substitutions, combining marks, and different Unicode sequences mapping to the same GID make `GID → rune` insufficient.

Folio's current `EmbeddedFont` uses a `map[uint16]rune` to track glyph usage and construct a basic `/ToUnicode` CMap. In the initial integration, allow Folio to build this baseline map but **do not rely on it to reconstruct complex text**.

The project's typesetter already knows the original source text and cluster-to-glyph association. Use that information to emit PDF **`/ActualText`** marked content around the appropriate cluster or logical text span:

```go
s.BeginMarkedContentActualText(sourceText)
// BT / Tf / Tm / Tj / ET for the corresponding glyphs
s.EndMarkedContent()
```

Folio takes an ordinary Go UTF-8 `string` and encodes the PDF `/ActualText` as **UTF-16BE with a BOM**. No UTF-16 representation is required in `docrender` itself.

Avoid huge, paragraph-wide `ActualText` spans where finer-grained selection is desired. Retain text-cluster/source-span information through painting; do not infer source text from GIDs afterward.

For a future improved text model, consider separate APIs for (a) glyph registration and (b) a caller-supplied `/ToUnicode` mapping, possibly with independent PDF CIDs. This is **not a prerequisite** for the first backend. PDF viewers and extraction tools do not all use `/ActualText` consistently, so assess extraction interoperability explicitly.

## 11. Coordinates and transformations

The PDF backend should be the sole owner of coordinate conversion. Let the display list keep a conventional **CSS page-space** coordinate system (origin top-left, +Y downward), unless the project later agrees otherwise.

In CSS absolute units:

```text
1 CSS px = 1/96 inch
1 PDF point = 1/72 inch
1 CSS px = 0.75 PDF pt
```

For a point `(x, y)` on a page of CSS-pixel height `H`, under an unrotated standard page coordinate system:

```text
xPDF = 0.75 × x

yPDF = 0.75 × (H − y)
```

For a top-left-origin rectangle `(x, y, w, h)`:

```text
xPDF = 0.75 × x

yPDF = 0.75 × (H − y − h)

wPDF = 0.75 × w

hPDF = 0.75 × h
```

Centralize this in the backend. Path control points and text baselines also need the correct conversion. CSS transform matrices require consistent affine matrix conversion; text orientation needs particular care if the backend uses a global Y-flip. The initial implementation may prefer per-coordinate conversion over one global PDF transform to avoid flipped glyphs.

This assumes the display list uses CSS pixels and default PDF user space. The actual document geometry must be consistent with page sizes and any PDF `/UserUnit` or page rotations.

## 12. Graphics state, clipping and resource scope

PDF's `q`/`Q` graphics state saves/restores clipping, transformation, color and other graphics properties. CSS painting will require nested scopes. A paint operation should not accidentally leak clipping, alpha or transforms to subsequent siblings.

Examples:

```text
Save
  Transform(...)
  ClipPath(...)
  FillPath(...)
  DrawGlyphs(...)
Restore
```

The **nesting semantics** are important; however, whether the display list represents them as flat `Save`/`Restore` operations or nested group objects is deferred.

Page frames may need to clip content to the frame geometry. The painter must determine whether clipping applies according to the resolved CSS/flow semantics. Rendering should not invent clipping simply because text flowed into a frame.

For images, Folio's existing `Page.AddImage` implements registration plus `q`, `cm`, `Do`, `Q`. Using a raw `Do` call requires a registered XObject name. Different resource classes may need additional small adapter APIs.

## 13. Opacity, blending and groups: known future requirement

Individual PDF fill/stroke opacity can be expressed through `/ExtGState` and `gs`; Folio exposes page helpers such as `SetOpacity` / `SetOpacityFillStroke` for registering these graphics states.

But CSS `opacity` on an element applies to the **composited group**, not separately to each child. Painting two overlapping children at 50% alpha is not equivalent to painting the children together and then compositing the resulting group at 50%.

Therefore support for full CSS opacity, `mix-blend-mode`, and some masks/filters will require **isolated groups**, commonly implemented as transparency-group Form XObjects in PDF.

This requirement must inform the eventual display-list design, but **the group's data representation and precise backend implementation are deferred**. For the first milestone, ordinary opaque painting, transforms, and clipping may be implemented before advanced compositing.

## 14. Suggested Go package responsibilities (provisional)

Package names below are **organizational suggestions**, not an approved final layout:

```text
docrender/
├── fragment/              # Page-specific resolved box/fragment trees
│   └── walk.go            # Generic tree inspection/traversal
├── paint/                 # CSS paint-order driver
│   ├── painter.go
│   ├── stacking.go
│   ├── backgrounds.go
│   └── borders.go
├── displaylist/           # Backend-neutral drawing instructions (TBD)
└── render/
    └── pdf/
        ├── renderer.go    # Interpret the display list
        ├── resources.go   # Resource names and shared resources
        ├── fonts.go       # folio/font.Face adapter, embedded font cache
        ├── text.go        # GID encoding, precise positions, ActualText
        ├── graphics.go    # Paths, stroke/fill, clipping, transform
        ├── images.go      # Image registration and output
        └── coords.go      # CSS px → PDF points/coordinate transform
```

A render-to-SVG or raster backend could consume the same display list. Keep Folio imports inside `render/pdf` (or a similarly narrow package). Avoid creating too many packages until interfaces and cycles are understood.

## 15. Overall PDF production procedure

A likely document-level workflow is:

1. **Upstream:** Produce and validate the page fragment trees with used values, resolved page/template/frame geometry and final shaped glyph positions.
2. **Paint each page:** Apply CSS paint order, stacking-context rules, clipping and decorations to produce a backend-independent display list.
3. **Create Folio document and pages:** Instantiate the document, configure page sizes and metadata, create a page for each display list.
4. **Prepare and cache font resources:** Wrap each existing OpenType font in an implementation of `font.Face`, then `font.NewEmbeddedFont(face)`. Manage one embedded instance per document/font instance as appropriate. Prepare images.
5. **Associate PDF page resources:** Obtain Folio font names and an initialized content stream using the proposed minimal API extension (or a temporary evaluated workaround, if the extension is not yet available).
6. **Replay display lists in order:** Convert coordinates; emit drawing operators; register used GIDs through `EncodeGIDs`; emit original-text marked content as needed; place images and graphics.
7. **Finalize:** Folio builds font subsets and their PDF dictionaries, registers page content streams/resources, constructs the page tree/catalog and serializes the PDF.
8. **Validate:** Compare rasterized output against trusted expectations and inspect extraction/accessibility behavior separately.

**Concurrency:** Folio's `font.EmbeddedFont` mutates glyph usage and is not safe for concurrent use. Do not mutate one shared embedded-font instance concurrently when replaying pages. Use serialized replay, explicit locking, or a separate resource-collection phase with a safe merge strategy. A parsed font face may be independently shareable if the project's adapter supports safe concurrency.

## 16. Implementation sequence for a coding assistant

### Milestone A — minimal backend proof of concept

- Verify and pin the Folio version; inspect the actual `font.Face` and `document.Page` APIs.
- Implement the `font.Face` adapter with a compile-time assertion.
- Create one embeddable static TrueType font without reparsing it in Folio.
- Confirm a public, non-hacky way to register a font and obtain an initialized page content stream. If absent, prepare the **minimal upstream patch** described in §7.2.
- Draw one rectangle and a small already-shaped glyph run on one page.
- Position each glyph explicitly with `Tm` + `Tj` and register glyph usage through `EncodeGIDs`.
- Emit `/ActualText` for the source cluster(s).
- Save PDF; open it in more than one PDF reader; confirm positioning and text extraction.

### Milestone B — painter-to-PDF integration

- Define a **temporary/experimental** bridge from painter events to PDF replay, or a test-oriented display-list adapter, without freezing the final display-list representation.
- Add paths, borders, fills, clipping, transforms, images, and multiple pages/frames.
- Verify expected stacking order and that graphics state does not leak between boxes.
- Compare the PDF result with layout coordinates and independently rasterized expected output.

### Milestone C — production fidelity and optimization

- Optimize suitable text into `TJ` sequences after establishing positional equivalence.
- Improve multi-script extraction, Unicode mappings, and accessible semantic output.
- Verify subset behavior and font resource deduplication across pages.
- Evaluate CFF support, font instances, additional color spaces, and PDF/A needs.
- Implement isolated group opacity and advanced CSS paint effects when the display-list design is settled.

## 17. Validation and acceptance criteria

**Basic geometry and painting**

- Page dimensions match page templates, including fixed-size frame placements.
- Backgrounds, borders, images and text appear in the intended order.
- Box fragments split across pages are painted correctly without repeating/missing content.
- Clips, transforms, and graphics-state push/pop are correct and balanced.
- Stacking contexts and `z-index` test cases follow CSS paint order, not naïve depth-first order.

**Text and fonts**

- Glyph IDs and final origins agree with the shaper's output; no shaping or line wrapping is performed by Folio.
- Kerning and GPOS marks are reproduced accurately via absolute positions.
- Ligatures, combining marks, RTL scripts and CJK produce visually correct output for the supported features.
- Source text copy/paste/search works for representative cases using `/ActualText`; assess different PDF readers/extractors.
- Font subsets contain every glyph used, and reused fonts are not unnecessarily duplicated per page.

**Architecture**

- `paint` does not import Folio; `render/pdf` does not walk or interpret CSS boxes.
- Backend-neutral resources do not contain PDF resource-name strings.
- All necessary PDF page resources are registered before serialization.
- The system detects unbalanced paint/graphics-state operations and unsupported features rather than silently producing incorrect output.

## 18. Deliberately deferred decisions

The next design discussion must settle these items; **do not silently decide them in the coding handoff**:

1. **Display-list representation:** flat operations versus nested sublists/groups; interface versus tagged type; builder versus direct append; retained versus immediately replayed.
2. **Display-list graphics-state discipline:** how `Save`/`Restore`, clipping, transforms and error recovery are represented and validated.
3. **Text-run representation:** final glyph origin coordinates versus advances/offsets; font-instance identity; source ranges and cluster granularity.
4. **Group representation:** opacity isolation, blend modes, masks and transparency groups.
5. **Resources:** where font/image identifiers are assigned and how immutable assets are shared across pages or documents.
6. **Potential upstream Folio API design:** exact names, signatures and acceptance of font registration/content-stream initialization methods.

These are known open questions, not blockers to prototyping the Folio font adapter and simple glyph painting path.

## 19. Primary references

**Folio implementation**

- <https://github.com/carlos7ags/folio>
- <https://github.com/carlos7ags/folio/blob/main/ARCHITECTURE.md>
- <https://pkg.go.dev/github.com/carlos7ags/folio/content>
- <https://github.com/carlos7ags/folio/blob/main/document/page.go>
- <https://github.com/carlos7ags/folio/blob/main/document/document.go>
- <https://github.com/carlos7ags/folio/blob/main/font/face.go>
- <https://github.com/carlos7ags/folio/blob/main/font/embed.go>
- <https://github.com/carlos7ags/folio/issues/260> (reason Folio replaced the `sfnt` parser; not a reason to prohibit external `font.Face` implementations)

**CSS painting/fragmentation**

- CSS 2 Appendix E, painting order: <https://drafts.csswg.org/css2/#painting-order>
- CSS Fragmentation Level 3: <https://drafts.csswg.org/css-break-3/>
- CSS Backgrounds and Borders: <https://drafts.csswg.org/css-backgrounds-3/>

**PDF semantics**

- PDF Association resources: <https://pdfa.org/resource/iso-32000-pdf/>
- Consult ISO 32000 for PDF text, resource, transparency, marked-content and CMap semantics where needed.

---

## Summary for the coding assistant

Implement `docrender`'s PDF rendering as a **backend for a CSS-painter-generated display list**, not as a PDF-specific HTML/CSS renderer. The input pages are already laid out, line-wrapped, shaped and paginated into fixed frames. A separate CSS painting driver resolves stacking order and paints the page fragments into backend-neutral drawing instructions. A PDF adapter interprets those instructions using Folio `content.Stream` and `document`; an adapter implements Folio `font.Face` around the existing OpenType library, using `font.EmbeddedFont` for glyph tracking, subsetting and font objects. Start with explicit per-glyph positioning and `/ActualText` from retained source clusters. A small upstream `document.Page` extension may be needed for embedded-font registration and stream initialization. **Do not decide the concrete display-list structure yet.**
