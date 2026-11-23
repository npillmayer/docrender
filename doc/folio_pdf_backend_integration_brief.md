# Folio PDF Backend Integration: `font.Face` Adapter Approach

**Implementation brief for a coding AI**  
**Prepared:** 23 November 2026  
**Target:** [`github.com/carlos7ags/folio`](https://github.com/carlos7ags/folio)  
**Source basis:** Folio's `main` branch inspected on the date above; **pin a release/commit and recheck signatures before implementation**. The latest release seen during research was v0.11.0 (23 October 2026).

## Executive decision

Build a PDF backend for an existing Go typesetter by **implementing Folio's existing `font.Face` interface as a thin adapter around the typesetter's own OpenType parser**. Pass that adapter to `font.NewEmbeddedFont` and reuse Folio for PDF font embedding/subsetting and PDF document serialization.

The typesetter already owns OpenType parsing, character mapping, shaping (GSUB/GPOS), measuring, line breaking, pagination, and final glyph positions. **Do not use Folio to shape, measure, or lay out the text.** Do not load or parse the font a second time with Folio's `font.ParseTTF` unless testing demonstrates a specific need.

The one important **remaining API obstacle** is in `document.Page`: it currently has no public way to **register an embedded font without drawing text** and obtain the corresponding `/Resources /Font` name. Also, `Page.ContentStream()` returns `nil` until internally initialized. A tiny upstream extension is preferable to copying/forking `document`.

Correct text extraction should be implemented chiefly with **`/ActualText` marked content**, emitted by the typesetter at shaped-cluster or meaningful span boundaries. Folio's existing `/ToUnicode` generation can remain as a *fallback* initially; it is not sufficiently expressive to represent all source-text-to-glyph relationships.

## 1. Architecture and ownership

```text
Existing Go typesetter                           Folio
────────────────────────────────────────────────────────────────────
Font binary ───┐
Font parser ───┼──> adapter implements font.Face ───> font.EmbeddedFont
GSUB/GPOS ────┤                                   • track used GIDs
Measuring ────┤                                   • subset/embed font
Line/page ────┤                                   • /W widths, descriptor
layout ───────┤                                   • automatic /ToUnicode
Positioned ───┼──> encode GIDs ───────────────────> content.Stream
GIDs/clusters │                                   • BT, Tf, Tm, Tj, TJ
Unicode text ─┘──> /ActualText ───────────────────> • BDC/EMC
                                                     │
                                            document.Page / Document
                                            • resource names/references
                                            • pages, page tree, catalog
                                            • sharing fonts across pages
                                                     │
                                            document.Writer + core
                                            • PDF objects, serialization
```

| Responsibility | Owner | Notes |
| --- | --- | --- |
| Original Unicode and source ranges | Typesetter | Retain mapping to shaped clusters. |
| Font binary + OpenType tables | Typesetter | Use one canonical in-memory representation. |
| Unicode→GID, GSUB, GPOS, visual ordering | Typesetter | Folio's shaping code is bypassed. |
| Glyph advances, line breaks, page layout, x/y positions | Typesetter | Export positions in PDF points. |
| `font.Face` methods | Thin adapter | Delegate mostly to existing parsed font. |
| Track used GIDs, subset static TTF, embed font | Folio `font.EmbeddedFont` | Initially reuse without modifying. |
| PDF font encoding, width array and fallback `/ToUnicode` | Folio `font` | Caveats for complex extraction below. |
| Accurate original text of shaped glyph clusters | Typesetter + Folio `content.Stream` | `/ActualText`. |
| Content operators | Folio `content` | No font parsing or shaping. |
| Fonts/images/page resources, page tree, catalog | Folio `document` | Needs a small API extension for manual glyph output. |
| Object types / low-level serialization | Folio `core`; document writer | Reuse unchanged. |

Folio's package architecture is documented in [`ARCHITECTURE.md`](https://github.com/carlos7ags/folio/blob/main/ARCHITECTURE.md). `document` is *document assembly*, not the typesetting engine; direct `AddPage()` can be used without `layout.Element`.

## 2. The crucial existing interface: `font.Face`

See [`font/face.go`](https://github.com/carlos7ags/folio/blob/main/font/face.go). The **current interface has 17 methods**, transcribed below. Keep in mind that future versions may change it.

```go
type Face interface {
    PostScriptName() string
    UnitsPerEm() int
    GlyphIndex(r rune) uint16
    GlyphAdvance(glyphID uint16) int
    Ascent() int
    Descent() int
    BBox() [4]int
    ItalicAngle() float64
    CapHeight() int
    StemV() int
    Kern(left, right uint16) int
    Flags() uint32
    RawData() []byte
    NumGlyphs() int
    GSUB() *GSUBSubstitutions
    GIDToUnicode() map[uint16]rune
    GPOS() *GPOSAdjustments
}
```

**Adapter design** (illustrative, not drop-in code: substitute actual local types and accessors):

```go
package folioadapter

import foliofont "github.com/carlos7ags/folio/font"

type Face struct {
    OT *OurParsedOpenTypeFont // placeholder: own immutable parsed-font type
}

// Implement all 17 foliofont.Face methods.
// Delegate PostScriptName, UnitsPerEm, GlyphIndex, GlyphAdvance,
// Ascent, Descent, BBox, ItalicAngle, CapHeight, StemV,
// Flags, RawData and NumGlyphs to OT.
// These methods must use units/semantics specified by font.Face.

func (f *Face) Kern(left, right uint16) int { return 0 }
func (f *Face) GSUB() *foliofont.GSUBSubstitutions { return nil }
func (f *Face) GPOS() *foliofont.GPOSAdjustments { return nil }
func (f *Face) GIDToUnicode() map[uint16]rune { return nil }

var _ foliofont.Face = (*Face)(nil)
```

Returning zero/nil for the four *shaping/reverse-mapping* methods is acceptable **for the scoped path using `EmbeddedFont.EncodeGIDs` and `BuildObjects` rather than Folio's layout/shaping**, subject to an integration test. The PDF-specific descriptor methods still need valid values; use real font metrics and document justified fallbacks rather than arbitrary guesses. `RawData()` must supply the appropriate complete embeddable font program (not merely a `glyf` or `CFF ` table), and keep its bytes alive through document emission.

Construct once per font instance per document:

```go
face := &folioadapter.Face{OT: parsedFont}
embedded := foliofont.NewEmbeddedFont(face)
```

**Memory implications:** `font.Face` is an interface around your existing data; `EmbeddedFont` stores the face and a `map[uint16]rune` of used glyphs. No second parsed `cmap`, metrics array, or duplicated original font binary is required. Subsetting, compression, and PDF writing **will** allocate temporary/new byte arrays. This is not a claim of zero allocation. Folio's `EmbeddedFont` is *not concurrency-safe* because encoding methods mutate its glyph registry. The parsed font may be shared; use separate `EmbeddedFont` instances for concurrent documents, and avoid mutating one instance concurrently.

## 3. Why `font.EmbeddedFont` remains valuable

[`font/embed.go`](https://github.com/carlos7ags/folio/blob/main/font/embed.go) declares, essentially:

```go
type EmbeddedFont struct {
    face       Face
    usedGlyphs map[uint16]rune
}
```

This object provides functionality the typesetter does not need to duplicate initially:

1. **Glyph registration:** `EncodeGIDs(gids, originalText)` encodes each already-shaped GID as a two-byte big-endian value and *records the GID* for subsetting. It does **not** re-shape.
2. **Embedding:** `BuildObjects(addObject)` constructs PDF font dictionaries/streams and their indirect references, including font descriptor, CIDFont, `/W`, embedded font program, and `/ToUnicode` CMap.
3. **TrueType subsetting:** `font.Subset` uses the registered glyph set, includes `.notdef` and composite dependencies, and preserves original GIDs by zeroing unused slots. Consequently an Identity CID-to-GID map still works.
4. **Fallback:** On TrueType subset error, `BuildObjects` falls back to embedding the whole font. This **does not** mean every font container/outline format can be embedded validly via that path; see limitations.

For the current TrueType path, the PDF mapping is:

```text
2-byte text code → Identity-H CID → same GID → embedded glyph outline
```

Font-side operators use e.g. `/F1 12 Tf` and `<002A> Tj` where `0x002A` selects GID 42. This is *not* UTF-16 text: although the codes are also two bytes long, they identify PDF font characters/glyphs rather than Unicode code points.

### Do not skip glyph registration

This is essential when using current `EmbeddedFont`: `EncodeGIDs()` populates `usedGlyphs`; `BuildObjects()` uses that set to build the TrueType subset and `/W` widths. Writing raw `ShowTextHex` bytes **without** calling `EncodeGIDs` (or another supported registration mechanism) risks excluding displayed outlines from the embedded subset. Register all glyphs before `Document.WriteTo` builds PDF objects. The original text string passed to `EncodeGIDs` is a best-effort Unicode fallback, **not** a shaping input.

Use the **same `*font.EmbeddedFont` for all pages sharing that font within one document**. The document writer caches the resulting font object by pointer, avoiding redundant copies across pages. Do not reset glyph usage partway through document generation.

## 4. How Folio currently gets from text to PDF glyph operators

Source paths: [`document/page.go`](https://github.com/carlos7ags/folio/blob/main/document/page.go), [`layout/draw.go`](https://github.com/carlos7ags/folio/blob/main/layout/draw.go), [`content/stream.go`](https://github.com/carlos7ags/folio/blob/main/content/stream.go).

### Ordinary Unicode text (not our intended path)

`Page.AddTextEmbedded(text, embeddedFont, size, x, y)` roughly does:

```text
register embedded font on page
→ embeddedFont.EncodeString(text)
    → Face.GlyphIndex for each rune, register used GIDs
→ BeginText, SetFont, MoveText, ShowTextHex, EndText
```

This assumes a simple character-to-glyph relation. It is **not appropriate for already-shaped complex runs**.

### Folio's own layout path (also not our intended path)

`layout.drawWordEmbedded` can consume `Word.GIDs`, calls `EmbeddedFont.EncodeGIDs`, and outputs `ShowTextHex`. Other branches implement kerning via `ShowTextArrayHex` (`TJ`) or special cursive/mark positioning. **Do not adopt `layout.Word`, nor try to map our shaping results into Folio's own shaping structures**: the external typesetter already knows every glyph's precise placement.

### Desired independent rendering path

For a first correctness-oriented version, use `content.Stream` directly:

- `BeginText` / `EndText` → `BT` / `ET`.
- `SetFont` → `Tf`.
- `SetTextMatrix(1, 0, 0, 1, x, y)` → `Tm`.
- `ShowTextHex(encodedGIDBytes)` → hex `Tj`.
- Later: `ShowTextArrayHex` → `TJ` for compact advance adjustments.
- `BeginMarkedContentActualText(sourceText)` / `EndMarkedContent()` → `BDC` / `EMC` for semantic text.

`content.Stream` **does not know which actual font** `/F1` denotes and does not register resources. The `document.Page` layer supplies `/Resources /Font` entries. Coordinate systems must be converted to PDF points, normally with page origin bottom-left.

## 5. The real obstacle: `document.Page` needs two small public operations

The `font.Face` adapter solves **font type compatibility** but **does not by itself expose a complete low-level drawing API through `document.Page`**.

Current internal code in `document/page.go`:

```go
type fontResource struct {
    name     string
    standard *font.Standard
    embedded *font.EmbeddedFont
}
```

The unexported `p.embeddedFontResource(embedded)` allocates/reuses `/F1`, `/F2`, ... names and registers them with the page. Existing public `AddTextEmbedded` uses this helper, but also draws text and runs `EncodeString` on the input. It does not reveal the resource name.

`Page.ContentStream()` is public but returns `nil` until a drawing call initializes the stream with the unexported `ensureStream()`.

**Preferred minimal upstream extension:** retain Folio's concrete `*font.EmbeddedFont`; do **not** introduce a second font interface yet. Add public operations roughly like:

```go
// Proposed API — NOT present in Folio at time of research.
func (p *Page) RegisterEmbeddedFont(ef *font.EmbeddedFont) string {
    return p.embeddedFontResource(ef)
}

// Proposed API — NOT present in Folio at time of research.
func (p *Page) EnsureContentStream() *content.Stream {
    p.ensureStream()
    return p.stream
}
```

These changes would be localized to `document/page.go`. Existing `document.Document`/`document.Writer` logic **already accepts `*font.EmbeddedFont`** and caches it across pages, so no change to `document/document.go` is inherently required for this adapter-first plan. Exact names and validation behavior are subject to upstream review.

This means **no copying of the `document` package**, no separate PDF object writer, and no dependence on Folio's layout. Alternatives such as allocating fonts indirectly with `AddTextEmbedded("", ...)` or assuming a font will be named `F1` are possible experimental hacks but should not become production interfaces. Another no-fork workaround is `Page.ImportPage` with a supplied resource dictionary; it wraps content in a Form XObject and is architecturally less direct.

If an upstream PR is not accepted, the options are a tiny maintained fork or a local overlay for these two methods, rather than rewriting `document` wholesale. The more general alternative of generic PDF font resources is a **later, optional** design, *not a requirement of this plan*.

## 6. Illustrative emission algorithm (after the two proposed Page APIs)

The following code is **pseudocode / a design sketch, not a compiling patch**. Adapt local run and glyph data types. It deliberately gives each glyph an explicit PDF position so Folio never measures or positions text.

```go
// Hypothetical local types:
// cluster.SourceText: original Unicode string (Go UTF-8)
// cluster.Glyphs: []{ GID uint16; X, Y float64 } // absolute PDF-point origins

resourceName := page.RegisterEmbeddedFont(embedded) // PROPOSED API
stream := page.EnsureContentStream()               // PROPOSED API

for _, cluster := range shapedClusters {
    // Preserve original source text independently from Folio's fallback map.
    stream.BeginMarkedContentActualText(cluster.SourceText)
    stream.BeginText()
    stream.SetFont(resourceName, fontSize)

    for _, glyph := range cluster.Glyphs {
        encoded := embedded.EncodeGIDs(
            []uint16{glyph.GID}, cluster.SourceText,
        ) // registers the GID + emits two-byte PDF character code

        stream.SetTextMatrix(1, 0, 0, 1, glyph.X, glyph.Y)
        stream.ShowTextHex(encoded)
    }

    stream.EndText()
    stream.EndMarkedContent()
}

// After emitting all pages:
// doc.WriteTo(output)
```

This is intentionally verbose but handles independent x/y positions (including mark offsets) and is easy to validate. For practical output, avoid thousands of tiny text objects/allocations: batch whole glyph runs where feasible, register GIDs in batches, and pack simple horizontal advances into `TJ` arrays.

For horizontal writing with no special text scaling, a `TJ` adjustment is approximately:

```text
TJ_number = 1000 × (nominal_PDF_advance - desired_shaped_advance) / font_size
```

The sign is reversed relative to a positive extra advance; **negative `TJ` values move following glyphs forward** in the usual horizontal text direction. `TJ` does *not* express arbitrary independent y offsets, so retain `Tm`/other positioning operators where required. For true RTL/vertical/mixed-direction cases, choose the correct PDF text matrix and emission order, not just a sign-flipped advance.

## 7. Unicode recovery: glyph registration ≠ text representation

Two separate concepts are involved:

### (A) Glyph registration — needed by Folio's subsetter

`EmbeddedFont.usedGlyphs` is currently:

```go
map[uint16]rune // GID → one Unicode rune
```

In the current implementation this *single map* provides both:

- the set of glyphs required for subsets/`/W` widths;
- a one-rune-per-GID fallback for the automatically emitted `/ToUnicode` CMap.

Registration is mandatory **with the current embedding implementation**. Unicode associations are incidental to registering the used GIDs.

### (B) Text representation — optional for appearance, essential for good extraction

PDF viewers do not need the original text to draw glyph outlines. But copy/paste, searching, extraction, indexing, and accessibility do.

PDF offers two relevant mechanisms:

| Mechanism | Keyed by | Purpose and limitation |
| --- | --- | --- |
| `/ToUnicode` CMap | PDF character code, font-wide | Maps codes to Unicode sequences. Current Folio maps one rune per GID, insufficient for some ligatures and contextual ambiguities. |
| `/ActualText` | Marked content span in a page | Declares the exact original Unicode string corresponding to a group of displayed glyphs, independently of GIDs. |

For example, source text `office` might shape into glyphs `[o, ffi-ligature, c, e]`. A one-rune-per-GID map cannot express the `ffi` sequence. The same glyph may also correspond to different source strings in different contexts. **Use the original source text retained by the typesetter and emit `/ActualText` around each meaningful shaped cluster or span.** This is a production-quality improvement over relying solely on the automatic CMap.

`content.Stream.BeginMarkedContentActualText(string)` is **already public** in Folio. It accepts a normal Go UTF-8 string and internally encodes a PDF text string as **UTF-16BE with BOM `FE FF`**. `EndMarkedContent()` emits `EMC`. There is no reason to change the typesetter's internal UTF-8 representation. PDF 1.7 commonly uses UTF-16BE; PDF 2.0 additionally permits BOM-marked UTF-8 for PDF text strings, but Folio's current helper uses UTF-16BE.

**Pragmatic first version:** keep Folio's automatic `/ToUnicode` as a fallback, but emit accurate `/ActualText` for shaped text. Extraction tools vary in how they honor `/ActualText`; test multiple consumers. A later upstream improvement could split `RegisterGIDs([]uint16)` from `SetToUnicode(...)`/custom CMap generation. Do **not** block the initial implementation on this change.

A `/ToUnicode` mapping is keyed by *PDF character code*, not universally by GID. In the present Identity-H encoding those coincide. A future more advanced scheme can allocate distinct CIDs that map to the same GID if different occurrences need different font-wide Unicode mappings, but it requires a CID-to-GID map and changes to the encoding/subsetting model. Prefer `/ActualText` first.

**Note:** `/ActualText` assists textual recovery; it is **not a substitute for the structure tree/reading order required for true PDF/UA accessibility**. Reordered RTL spans, spaces, ligatures, grapheme clusters, and line-break behavior all require extraction tests.

## 8. Font-format and technical constraints

1. **Phase 1: static TrueType (`glyf`/`loca`) fonts.** Folio's TrueType path uses a Type0/CIDFontType2 hierarchy with `/FontFile2`, `/Encoding /Identity-H`, `/CIDToGIDMap /Identity`, `/W` widths, and `/ToUnicode`.
2. **Subset retains GIDs** and includes composite dependencies. Its subset writer reconstructs selected TrueType tables and omits variation tables. Review correctness of all used glyphs, not only file size.
3. **CFF requires a different PDF embedding path.** The current source uses a package-private structural interface named `cffFace` with *exported* methods `IsCFF()`, `IsCFF2()`, and `CFFData()`, and only routes **CID-keyed CFF v1** to its special PDF builder. A foreign adapter could supply those methods, but **ordinary name-keyed CFF v1 and CFF2 must not be assumed to work**. Confirm explicitly before enabling them.
4. **Variable fonts:** PDF emission should use a properly **instantiated static font program** matching the variation coordinates and metrics used by the shaper; passing the uninstantiated variable font unchanged is not a valid substitute. Plan a separate implementation effort.
5. **TTC/WOFF/WOFF2:** `RawData()` must contain a PDF-embeddable font program in a form Folio's embedding/subsetting path understands. If the source is a collection or webfont container, extraction/conversion may be required.
6. **Metrics:** Folio asks for nominal font design-unit advances/descriptor data; use matching PDF font instance and correct UPEM/scale. Per-glyph actual advance/offset remains under typesetter control. Validate PDF `/W` against those nominal metrics.
7. **Licensing:** check embedding restrictions for the chosen font, especially if broad font ingestion is planned.
8. **Concurrency:** one immutable parsed font may serve many documents; one mutable `EmbeddedFont` is scoped to one document and should not be shared for concurrent emission.
9. **Validation/conformance:** optional PDF/A and PDF/UA requirements are more demanding than merely generating a visually correct PDF; do not claim conformance from glyph rendering alone.

## 9. Why an upstream change would be compatible with Folio's philosophy

Folio stopped using `golang.org/x/image/font/sfnt` for font metrics because limits such as `maxCmapSegments = 20000` prevented parsing some large CJK fonts. It moved parsing of necessary tables in-tree to control correctness and error handling: [issue #260](https://github.com/carlos7ags/folio/issues/260), [PR #262](https://github.com/carlos7ags/folio/pull/262), and [related issue #248](https://github.com/carlos7ags/folio/issues/248).

**The proposed small `document.Page` extension does not reintroduce an external parser dependency into Folio.** The external application implements the already existing `font.Face` interface and supplies a PDF-ready `*font.EmbeddedFont`. This preserves Folio's own parser and layout for existing callers, while making its low-level content API usable for an independent shaper.

A good upstream issue/PR framing:

> Expose registration of an existing `*font.EmbeddedFont` on `document.Page`, returning the page font-resource name, and a guaranteed initialized `content.Stream`. This allows external typesetters to emit pre-shaped, positioned glyph IDs without invoking `AddTextEmbedded` or Folio layout. No changes to Folio's font parser, encoding rules, document writer, or existing APIs are required.

Do not propose generic external font parsers or a wholesale rewrite of `document` in the first PR. A more general `PDFFont`/generic-resource interface is a possible later contribution if the adapter proves insufficient.

## 10. Proposed execution plan for a coding AI

### Milestone 0 — Confirm compatibility

- [ ] Pin Folio to an exact released version/commit and inspect the actual `font.Face`, `EmbeddedFont`, `document.Page`, and `content.Stream` signatures.
- [ ] Audit `BuildObjects`/`Subset` for supported outline formats and fallback behavior.
- [ ] Determine the typesetter's internal units and coordinates; define conversion to PDF points.

### Milestone 1 — Build the adapter

- [ ] Implement the 17 `font.Face` methods around the existing parsed OpenType font; add compile-time interface assertion.
- [ ] Use real font bytes and consistent font descriptor metrics; choose a static TTF test font first.
- [ ] Return nil/zero for `GSUB`, `GPOS`, `GIDToUnicode`, `Kern` **only when those methods are not used by the chosen output path**; confirm with tests.
- [ ] Create a `*font.EmbeddedFont` per font instance per document. Never call Folio `ParseTTF` in the intended production path.

### Milestone 2 — Open the smallest page API seam

- [ ] Make a small upstream patch to `document.Page` exposing `RegisterEmbeddedFont` and a non-nil stream accessor (names provisional).
- [ ] Add tests ensuring resource-name stability on a page, sharing across pages, initialization on an empty page, and unchanged standard APIs.
- [ ] Keep `document.Writer`, `core`, and Folio font internals unchanged.

### Milestone 3 — Emit accurate glyphs

- [ ] Use externally computed GIDs and absolute positions; register GIDs via `EncodeGIDs` before writing.
- [ ] Use `SetFont`, `SetTextMatrix`, `ShowTextHex`, and text object boundaries.
- [ ] Apply `/ActualText` to appropriate clusters/spans using the typesetter's original source text.
- [ ] Test actual x/y positions, component glyphs, ligatures, combining marks, RTL runs, and font switching.

### Milestone 4 — Validate output

- [ ] Inspect the resulting PDF structure: page `/Resources/Font`, Type0, CIDFont, font descriptor, `/W`, `/CIDToGIDMap`, embedded font data, and `/ToUnicode`.
- [ ] Confirm **each visible glyph** survives subsetting and uses the expected GID.
- [ ] Test copy/paste, search and text extraction with multiple viewers/tools, especially ligatures, combining marks, CJK and non-BMP Unicode.
- [ ] Check multi-page sharing: embedded font program appears once for a shared `*EmbeddedFont`.
- [ ] Check that consecutive builds are deterministic and that independent documents do not leak glyph registries.

### Milestone 5 — Optimize and expand

- [ ] Coalesce run output and use `TJ` where feasible, preserving per-glyph y offsets.
- [ ] Consider a better interface for independent GID registration and `/ToUnicode` mapping only if experiments justify it.
- [ ] Add CFF, TTC/WOFF conversion, variable-font instancing, advanced tagging/PDF/A only in separate phases.

## 11. Acceptance criteria and non-goals

**Minimum successful proof of concept**

- A one-page PDF using a static TTF, with the font parsed **only by the typesetter** and adapted through `font.Face`.
- The page uses a single real Folio `*font.EmbeddedFont`, registered via a small page API extension.
- A pre-shaped run rendered via `content.Stream` contains the exact GIDs at the exact expected positions; Folio does not reshape or remeasure it.
- The font is correctly embedded/subset; text is visually correct.
- `/ActualText` recovers original source text for at least a ligature case.
- Two pages using the same embedded-font pointer share the PDF font program.

**Explicit non-goals of phase 1:** implementing a new PDF serializer, rewriting Folio's document layer, integrating Folio's `layout` package, building Folio-specific GSUB/GPOS representations, replacing Folio's automatic `/ToUnicode`, complete PDF/UA conformance, or full CFF/variable-font support.

## 12. Primary source links (checked on 2026-11-23)

- [Folio repository and README](https://github.com/carlos7ags/folio)
- [Architecture and package contracts](https://github.com/carlos7ags/folio/blob/main/ARCHITECTURE.md)
- [`font/face.go` — Face interface + CFF detection](https://github.com/carlos7ags/folio/blob/main/font/face.go)
- [`font/embed.go` — EmbeddedFont, EncodeGIDs, PDF font objects, ToUnicode](https://github.com/carlos7ags/folio/blob/main/font/embed.go)
- [`font/subset.go` — GID-preserving TrueType subsetting](https://github.com/carlos7ags/folio/blob/main/font/subset.go)
- [`document/page.go` — font registration, AddTextEmbedded, ContentStream](https://github.com/carlos7ags/folio/blob/main/document/page.go)
- [`document/document.go` — page-resource serialization, font cache, writer](https://github.com/carlos7ags/folio/blob/main/document/document.go)
- [`content/stream.go` — PDF text/positioning and ActualText operators](https://github.com/carlos7ags/folio/blob/main/content/stream.go)
- [`layout/draw.go` — Folio's own shaped-GID rendering paths (reference only)](https://github.com/carlos7ags/folio/blob/main/layout/draw.go)
- [Issue #248: large-CJK cmap limit](https://github.com/carlos7ags/folio/issues/248)
- [Issue #260: replace sfnt dependency](https://github.com/carlos7ags/folio/issues/260)
- [PR #262: internal OpenType table parsers](https://github.com/carlos7ags/folio/pull/262)

---

**Instructions to the coding AI:** Implement the **adapter-first** plan, not a replacement PDF backend. Distinguish verified existing Folio APIs from *proposed methods*; never assume `RegisterEmbeddedFont` or `EnsureContentStream` already exists. Start with static TrueType and a minimal golden PDF test, and report any constraints discovered in the pinned Folio version before broadening scope. Source-level checks are documented here; no end-to-end integration program was compiled or run as part of this brief.
