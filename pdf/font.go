package pdf

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"strings"

	"github.com/carlos7ags/folio/core"
	foliofont "github.com/carlos7ags/folio/font"
	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/dimen"
	fontmeta "github.com/npillmayer/typography/font"
)

// Font is an opaque sized font binding belonging to one Document. Copies are
// safe to retain; the zero value is invalid. No upstream face is closed or
// mutated, and no Folio service is exposed.
type Font struct {
	owner   *Document
	program *fontProgram
	size    float64
}

type fontProgram struct {
	sha256   string
	name     string
	embedded *foliofont.EmbeddedFont
	used     bool
}

// buildObjects contains the v0.11.0 font-object adaptations. Glyph-only
// EncodeGIDs usage gives every used GID a placeholder rune 0. Folio's subsetter
// turns these into duplicate cmap segments. Embed the original program instead
// until glyph-only subsetting has a sound backend API; never invent Unicode.
func (f *fontProgram) buildObjects(add func(core.PdfObject) *core.PdfIndirectReference) *core.PdfDictionary {
	raw := f.embedded.Face().RawData()
	subsetName := false
	dict := f.embedded.BuildObjects(func(obj core.PdfObject) *core.PdfIndirectReference {
		if stream, ok := obj.(*core.PdfStream); ok && stream.Dict.Get("Length1") != nil {
			// In the accepted TrueType profile this is the FontFile2 stream,
			// built before the descriptor and CID font by the pinned release.
			subsetName = !bytes.Equal(stream.Data, raw)
			stream.Data = raw
			stream.Dict.Set("Length1", core.NewPdfInteger(len(raw)))
		}
		if desc, ok := obj.(*core.PdfDictionary); ok {
			if kind, ok := desc.Get("Type").(*core.PdfName); ok && kind.Value == "FontDescriptor" {
				// Folio writes design units here; PDF expects 1000-em units.
				normalizeDescriptor(desc, f)
			}
			if subsetName {
				removeSubsetTag(desc, "FontName")
				removeSubsetTag(desc, "BaseFont")
			}
		}
		return add(obj)
	})
	if subsetName {
		removeSubsetTag(dict, "BaseFont")
	}
	// Do not advertise U+0000 placeholders as semantic mappings. BuildObjects
	// has already registered the CMap object; it remains unreferenced.
	dict.Remove("ToUnicode")
	return dict
}

func removeSubsetTag(dict *core.PdfDictionary, key string) {
	if name, ok := dict.Get(key).(*core.PdfName); ok {
		prefix, rest, found := strings.Cut(name.Value, "+")
		if found && len(prefix) == 6 {
			dict.Set(key, core.NewPdfName(rest))
		}
	}
}

// BindFont pairs an upstream sized instance with a private Folio model of the
// same retained binary. The face must implement typography/font.ResourceFace.
// No source path is reopened and no font is rediscovered or substituted.
//
// Programs with identical bytes share document-local embedding state, including
// across sizes and provenance records. The size remains part of each binding.
// BindFont reads the instance's already-bound scale, but never calls Face.Scale
// or Face.Metrics. Painting and writing need no upstream font services afterward.
func (d *Document) BindFont(instance contracts.FontInstance) (Font, error) {
	if d == nil || !instance.Valid() {
		return Font{}, fmt.Errorf("%w: document or font instance", ErrInvalidInput)
	}
	bindings := d.stageFonts()
	font, err := bindings.bind(instance, instance.Scale().ToDU(1)/float64(dimen.BP))
	if err != nil {
		return Font{}, err
	}
	d.fonts = bindings.programs
	return font, nil
}

// fontBindings stages new programs without publishing them to the document.
// Existing programs are only inspected until the entire drawing call succeeds.
type fontBindings struct {
	owner    *Document
	programs []*fontProgram
}

func (d *Document) stageFonts() fontBindings {
	// Limit capacity so the first append cannot change the document's backing
	// storage. Existing program pointers remain shared within this document.
	return fontBindings{owner: d, programs: d.fonts[:len(d.fonts):len(d.fonts)]}
}

func (b *fontBindings) bind(instance contracts.FontInstance, unitsToBP float64) (Font, error) {
	if !instance.Valid() || !finite(unitsToBP) || unitsToBP <= 0 {
		return Font{}, fmt.Errorf("%w: retained font instance or scale", ErrInvalidInput)
	}
	info, err := fontmeta.Identify(instance.Face())
	if err != nil {
		return Font{}, fmt.Errorf("%w: font identity: %w", ErrInvalidInput, err)
	}
	var program *fontProgram
	for _, candidate := range b.programs {
		if candidate.sha256 == info.SHA256 {
			program = candidate
			break
		}
	}
	newProgram := program == nil
	if newProgram {
		r := instance.Face().(fontmeta.ResourceFace).Resource().Open()
		data, err := io.ReadAll(r)
		r.Close()
		if err != nil {
			return Font{}, fmt.Errorf("pdf: read retained font: %w", err)
		}
		if err := checkFontProfile(data); err != nil {
			return Font{}, err
		}
		face, err := foliofont.ParseTTF(data)
		if err != nil {
			return Font{}, fmt.Errorf("pdf: Folio font parse: %w", err)
		}
		if face.UnitsPerEm() <= 0 || face.NumGlyphs() <= 0 {
			return Font{}, fmt.Errorf("%w: font has invalid em or glyph count", ErrInvalidInput)
		}
		program = &fontProgram{
			sha256:   info.SHA256,
			name:     fmt.Sprintf("F%d", len(b.programs)+1),
			embedded: foliofont.NewEmbeddedFont(face),
		}
	}
	// The retained conversion must agree with the exact program's UPEM and
	// physical em size. This guards against a mismatched resource association.
	em := instance.EmSize().Points()
	scaledEm := unitsToBP * float64(program.embedded.Face().UnitsPerEm())
	if !finite(scaledEm) || math.Abs(scaledEm-em) > em*1e-9 {
		return Font{}, fmt.Errorf("%w: retained scale disagrees with font program", ErrInvalidInput)
	}
	if newProgram {
		b.programs = append(b.programs, program)
	}
	return Font{owner: b.owner, program: program, size: em}, nil
}

// Limit this spike to one static SFNT face with monochrome TrueType outlines.
// Checking the directory also prevents unsupported formats from silently taking
// Folio's TrueType embedding fallback. This is not a replacement font parser.
func checkFontProfile(data []byte) error {
	if len(data) < 12 {
		return fmt.Errorf("%w: truncated font header", ErrInvalidInput)
	}
	signature := binary.BigEndian.Uint32(data)
	if signature != 0x00010000 && string(data[:4]) != "true" {
		return fmt.Errorf("%w: expected a single TrueType SFNT face", ErrUnsupported)
	}
	n := int(binary.BigEndian.Uint16(data[4:6]))
	if len(data) < 12+16*n {
		return fmt.Errorf("%w: truncated font directory", ErrInvalidInput)
	}
	var glyf, loca bool
	for i := 0; i < n; i++ {
		entry := data[12+16*i : 12+16*(i+1)]
		offset, length := binary.BigEndian.Uint32(entry[8:]), binary.BigEndian.Uint32(entry[12:])
		if uint64(offset)+uint64(length) > uint64(len(data)) {
			return fmt.Errorf("%w: font table outside snapshot", ErrInvalidInput)
		}
		switch tag := string(entry[:4]); tag {
		case "glyf":
			glyf = true
		case "loca":
			loca = true
		case "fvar", "CFF ", "CFF2", "COLR", "CPAL", "SVG ", "CBDT", "CBLC", "sbix", "EBDT", "EBLC":
			return fmt.Errorf("%w: font table %q outside static outline profile", ErrUnsupported, tag)
		}
	}
	if !glyf || !loca {
		return fmt.Errorf("%w: font has no TrueType outlines", ErrUnsupported)
	}
	return nil
}
