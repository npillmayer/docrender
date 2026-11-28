// Package pdf emits settled glyph positions and simple paint through Folio.
// It performs no shaping, layout, bidi reordering or CSS paint traversal.
// Calls are painted in submission order; the caller chooses that order.
//
// All dimensions are big points (1/72 inch), with page origins at the top left
// and Y increasing downwards. Glyph origins include all shaping offsets and
// layout adjustments. Each glyph gets its own absolute PDF text matrix.
//
// This experimental adapter supports static, single-face TrueType outlines and
// opaque RGB paint. It emits no semantic text mapping or accessibility tags.
// Folio types and mutable embedding state stay private to this package.
// PaintFragmentGlyphs consumes selected ranges of typography/render.FragmentResult
// using an explicit caller translation. Fully transparent retained glyphs are
// skipped. Atomic composition and CSS paint-order ownership remain external.
package pdf
