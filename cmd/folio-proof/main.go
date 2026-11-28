// folio-proof connects immutable fragment results to PDF with fixed demo rows.
// It is deliberately not a pagination or CSS traversal implementation.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"image/color"
	"os"
	"path/filepath"

	"github.com/npillmayer/cords/styled"
	"github.com/npillmayer/docrender/pdf"
	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/khipu/helpers"
	glyphot "github.com/npillmayer/typography/glyphing/adapter/opentype"
	"github.com/npillmayer/typography/glyphing/paragraph"
	"github.com/npillmayer/typography/render"
	renderot "github.com/npillmayer/typography/render/adapter/opentype"
	"github.com/npillmayer/uax/bidi"
	"golang.org/x/text/language"
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
	text  string
	size  int
	x, y  float64
	paint color.NRGBA
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
			{fmt.Sprintf("Fragment result / %02d", index+1), 26, 48, 92, ink},
			{"Retained typography through the Folio adapter", 12, 48, 117, muted},
			{"Fragment proof: fixed rows, no CSS traversal or pagination.", 10, 48, 625, muted},
		}
		if index == 0 {
			rows = append(rows,
				proofRow{"Office affinity: ffi, fi, fl.", 24, 48, 184, ink},
				proofRow{"AVATAR and To: shaped spacing.", 20, 48, 228, ink},
				proofRow{"One font program, several sizes, two pages.", 12, 48, 263, muted},
				proofRow{"Captured glyph origins and solid color", 12, 48, 336, muted},
				proofRow{"AVATAR", 30, 48, 382, blue},
				proofRow{"Opaque foreground and rectangle paint", 16, 64, 493, white},
			)
			if err = p.FillRect(48, 460, 404, 56, blue); err != nil {
				return err
			}
		} else {
			rows = append(rows,
				proofRow{"Hebrew marks and right-to-left glyph order", 12, 48, 164, muted},
				proofRow{"One glyph, one absolute origin", 22, 48, 338, ink},
				proofRow{"The caller supplies paint order and settled geometry.", 13, 48, 376, ink},
				proofRow{"Nominal font widths do not place the next glyph.", 13, 48, 404, ink},
				proofRow{"Semantic text remains deferred.", 12, 48, 468, muted},
			)
			if err = line(p, hebrew, "שָׁלוֹם עוֹלָם", 32, 48, 232, blue, bidi.RightToLeft); err != nil {
				return err
			}
		}
		for _, row := range rows {
			if err = line(p, latin, row.text, row.size, row.x, row.y, row.paint, bidi.LeftToRight); err != nil {
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

func line(p *pdf.Page, face *glyphot.Face, text string, size int, x, y float64, paint color.NRGBA, direction bidi.Direction) error {
	script, lang := language.MustParseScript("Latn"), language.English
	if direction == bidi.RightToLeft {
		script, lang = language.MustParseScript("Hebr"), language.Hebrew
	}
	prepared, err := paragraph.Prepare(paragraph.Input{Text: styled.TextFromString(text), Direction: direction}, khipu.PrepareOptions{}, khipu.Config{
		Streaming: &khipu.StreamingConfig{Resolver: helpers.DefaultStyleResolver{},
			Defaults: contracts.ResolvedRunStyle{Font: face, EmSize: dimen.DU(size) * dimen.BP, Script: script, Language: lang,
				Features: []contracts.ShapeFeature{{Tag: [4]byte{'l', 'i', 'g', 'a'}, Enabled: true}}},
			NewShaper: func() (contracts.StreamingShaper, error) { return glyphot.NewStream(16) },
		},
	})
	if err != nil {
		return err
	}
	laidOut, err := prepared.Layout(paragraph.LayoutRequest{
		Params: paragraph.Params{Constraints: paragraph.Constraints{Width: 404 * dimen.BP}, Breaker: paragraph.FirstFit,
			Assembly: paragraph.Options{Policy: paragraph.CSSLineHeight}, LineLayout: &paragraph.LineLayout{Default: paragraph.LineHeight{Kind: paragraph.NormalHeight}}},
		Horizontal: &paragraph.HorizontalLayout{},
	})
	if err != nil {
		return err
	}
	fragment, err := paragraph.Fragment(laidOut.VList)
	if err != nil {
		return err
	}
	result, err := render.BuildFragmentResult(fragment, render.TextPaint{Default: paint}, renderot.New())
	if err != nil {
		return err
	}
	// This command places fixed rows by baseline. The adapter accepts only the
	// resulting explicit translation and adds no layout offsets of its own.
	return p.PaintFragmentGlyphs(result, render.IndexRange{End: result.GlyphCount()}, x, y-result.Box().Baseline)
}
