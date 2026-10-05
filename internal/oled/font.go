package oled

import (
	_ "embed"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
)

//go:embed assets/Minecraftia-Regular.ttf
var minecraftiaTTF []byte

const textFaceSize = 8

var textFace = mustLoadTextFace()

// ponytail: sfnt/opentype don't cache rasterized glyphs, so Tick()-driven
// redraws re-rasterize every glyph every frame. No glyph cache here — add
// one if profiling on real hardware shows it matters.
func mustLoadTextFace() font.Face {
	parsed, err := opentype.Parse(minecraftiaTTF)
	if err != nil {
		panic("oled: parse embedded Minecraftia font: " + err.Error())
	}
	face, err := opentype.NewFace(parsed, &opentype.FaceOptions{
		Size:    textFaceSize,
		DPI:     72,
		Hinting: font.HintingNone,
	})
	if err != nil {
		panic("oled: build Minecraftia face: " + err.Error())
	}
	return face
}
