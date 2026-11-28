// folio-proof exercises the bottom PDF boundary with fixed demonstration rows.
// It is deliberately not a pagination or CSS traversal implementation.
package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"image/color"
	"os"
	"path/filepath"
	"strings"

	"github.com/npillmayer/docrender/pdf"
	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/dimen"
	glyphot "github.com/npillmayer/typography/glyphing/adapter/opentype"
	renderot "github.com/npillmayer/typography/render/adapter/opentype"
	"golang.org/x/text/language"
	"golang.org/x/text/unicode/bidi"
)

func main() {
	out := flag.String("o", "output/pdf/folio-proof.pdf", "output PDF path")
	latin := flag.String("latin", "../opentype/testdata/fonts/GentiumPlus-R.ttf", "static TrueType Latin font fixture")
	hebrew := flag.String("hebrew", "../opentype/testdata/fonts/NotoSansHebrew-Regular.ttf", "static TrueType Hebrew font fixture")
	flag.Parse()
	if err := run(*out, *latin, *hebrew); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println(*out)
}

type proofRow struct {
	text           string
	size           int
	x, y, tracking float64
	paint          color.NRGBA
}

func run(path, latinPath, hebrewPath string) error {
	latin, err := glyphot.LoadFile(latinPath)
	if err != nil {
		return err
	}
	hebrew, err := glyphot.LoadFile(hebrewPath)
	if err != nil {
		return err
	}
	var d pdf.Document
	ink := color.NRGBA{R: 32, G: 45, B: 61, A: 255}
	muted := color.NRGBA{R: 95, G: 110, B: 124, A: 255}
	blue := color.NRGBA{R: 36, G: 80, B: 121, A: 255}
	white := color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	for index := range 2 {
		p, err := d.AddPage(500, 680)
		if err != nil {
			return err
		}
		if err = p.FillRect(48, 48, 36, 4, blue); err != nil {
			return err
		}
		// Fixed proof rows live only in this command; they are not a proposed
		// rendering payload, display list, or general text-to-PDF API.
		rows := []proofRow{
			{fmt.Sprintf("Folio adapter / %02d", index+1), 26, 48, 92, 0, ink},
			{"Positioned glyphs from the OpenType shaper", 12, 48, 117, 0, muted},
			{"Backend proof: fixed rows, no CSS traversal or pagination.", 10, 48, 625, 0, muted},
		}
		if index == 0 {
			rows = append(rows,
				proofRow{"Office affinity: ffi, fi, fl.", 24, 48, 184, 0, ink},
				proofRow{"AVATAR and To: shaped spacing.", 20, 48, 228, 0, ink},
				proofRow{"One font program, several sizes, two pages.", 12, 48, 263, 0, muted},
				proofRow{"Deliberately expanded glyph origins", 12, 48, 336, 0, muted},
				proofRow{"AVATAR", 30, 48, 382, 6, blue},
				proofRow{"Opaque foreground and rectangle paint", 16, 64, 493, 0, white},
			)
			if err = p.FillRect(48, 460, 404, 56, blue); err != nil {
				return err
			}
		} else {
			rows = append(rows,
				proofRow{"Hebrew marks and right-to-left glyph order", 12, 48, 164, 0, muted},
				proofRow{"One glyph, one absolute origin", 22, 48, 338, 0, ink},
				proofRow{"The caller supplies paint order and settled geometry.", 13, 48, 376, 0, ink},
				proofRow{"Nominal font widths do not place the next glyph.", 13, 48, 404, 0, ink},
				proofRow{"Semantic text remains deferred.", 12, 48, 468, 0, muted},
			)
			if err = line(&d, p, hebrew, "שָׁלוֹם עוֹלָם", 32, 48, 232, 0, blue, bidi.RightToLeft); err != nil {
				return err
			}
		}
		for _, row := range rows {
			if err = line(&d, p, latin, row.text, row.size, row.x, row.y, row.tracking, row.paint, bidi.LeftToRight); err != nil {
				return err
			}
		}
	}
	// Finish serialization before opening a destination. File I/O remains owned
	// by this caller; the pdf adapter only writes to an io.Writer.
	var data bytes.Buffer
	if _, err = d.WriteTo(&data); err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, data.Bytes(), 0644)
}

func line(d *pdf.Document, p *pdf.Page, face *glyphot.Face, text string, size int, x, y, tracking float64, paint color.NRGBA, direction bidi.Direction) error {
	instance, err := contracts.NewFontInstance(face, dimen.DU(size)*dimen.BP)
	if err != nil {
		return err
	}
	bound, err := d.BindFont(instance)
	if err != nil {
		return err
	}
	script, lang := language.MustParseScript("Latn"), language.English
	if direction == bidi.RightToLeft {
		script, lang = language.MustParseScript("Hebr"), language.Hebrew
	}
	stream, err := glyphot.NewStream(16)
	if err != nil {
		return err
	}
	done := make(chan error, 1)
	go func() {
		err := stream.Shape(context.Background(), contracts.ShapeParams{
			Font: instance, Direction: direction, Script: script, Language: lang,
			Features: []contracts.ShapeFeature{{Tag: [4]byte{'l', 'i', 'g', 'a'}, Enabled: true}},
		}, strings.NewReader(text), contracts.ShapeBufferOptions{})
		if finishErr := stream.Finish(); err == nil {
			err = finishErr
		}
		done <- err
	}()
	var logical []contracts.ShapedGlyph
	for g := range stream.Glyphs() {
		logical = append(logical, g)
	}
	if err = <-done; err != nil {
		return err
	}
	if err = stream.Err(); err != nil {
		return err
	}
	// Reuse existing direction handling without calling Outline. This temporary
	// proof-only loop is not the pending per-fragment traversal implementation.
	visual, err := renderot.New().VisualOrder(logical, direction)
	if err != nil {
		return err
	}
	scale := instance.Scale().ToDU(1) / float64(dimen.BP)
	glyphs := make([]pdf.Glyph, 0, len(visual))
	for _, g := range visual {
		glyphs = append(glyphs, pdf.Glyph{Font: bound, ID: g.GlyphID, X: x + float64(g.XOffset)*scale, Y: y - float64(g.YOffset)*scale, Color: paint})
		x += float64(g.XAdvance)*scale + tracking
		y -= float64(g.YAdvance) * scale
	}
	return p.PaintGlyphs(glyphs)
}
