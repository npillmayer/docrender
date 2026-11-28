package pdf

import (
	"bytes"
	"errors"
	"testing"

	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/dimen"
	fontmeta "github.com/npillmayer/typography/font"
	"github.com/npillmayer/typography/fontresource"
	otfont "github.com/npillmayer/typography/glyphing/adapter/opentype"
)

type opaqueFace struct{ contracts.FontFace }
type wrongIdentity struct{ fontmeta.ResourceFace }

func (f wrongIdentity) Info() fontmeta.Info {
	i := f.ResourceFace.Info()
	i.SHA256 = "wrong program"
	return i
}

type wrongScale struct{ fontmeta.ResourceFace }

func (f wrongScale) Scale(size dimen.DU) (contracts.FontScale, error) {
	return f.ResourceFace.Scale(2 * size)
}

func TestRejectMissingResourceIdentityAndScaleMismatch(t *testing.T) {
	face, _ := fixture(t)
	for name, face := range map[string]contracts.FontFace{
		"missing capability": opaqueFace{face},
		"wrong identity":     wrongIdentity{face},
		"wrong scale":        wrongScale{face},
	} {
		t.Run(name, func(t *testing.T) {
			var d Document
			i, err := contracts.NewFontInstance(face, 12*dimen.BP)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := d.BindFont(i); !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("accepted invalid font association: %v", err)
			}
			if len(d.fonts) != 0 {
				t.Fatal("failed binding retained a program")
			}
		})
	}
}

func TestSameProgramDifferentProvenance(t *testing.T) {
	first, data := fixture(t)
	r, err := fontresource.Read(bytes.NewReader(data), fontresource.Origin{Location: "another/absent.ttf"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := otfont.LoadResource(r)
	if err != nil {
		t.Fatal(err)
	}
	if first.Info().ResourceID == second.Info().ResourceID {
		t.Fatal("fixture provenance did not differ")
	}
	var d Document
	f1, f2 := bind(t, &d, first, 12*dimen.BP), bind(t, &d, second, 18*dimen.BP)
	if f1.program != f2.program || f1.size == f2.size {
		t.Fatal("program identity confused with provenance or sized identity")
	}
	var other Document
	f3 := bind(t, &other, first, 12*dimen.BP)
	if f1.program.embedded == f3.program.embedded {
		t.Fatal("documents share mutable embedding state")
	}
}
