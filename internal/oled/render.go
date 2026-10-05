package oled

import (
	_ "embed"
	"fmt"
	"image"
	"image/color"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"

	"github.com/dnitros/pironman/internal/hardware"
)

const textLineHeight = 13

//go:embed fonts/minecraftia/Minecraftia-Regular.ttf
var minecraftiaTTF []byte

var (
	textFace = mustTextFace(8)
	bigFace  = mustTextFace(16)
)

func mustTextFace(size float64) font.Face {
	f, err := opentype.Parse(minecraftiaTTF)
	if err != nil {
		panic(fmt.Sprintf("oled: parse embedded Minecraftia font: %v", err))
	}
	face, err := opentype.NewFace(f, &opentype.FaceOptions{Size: size, DPI: 72, Hinting: font.HintingFull})
	if err != nil {
		panic(fmt.Sprintf("oled: build Minecraftia face: %v", err))
	}
	return face
}

func newFrame() *image.Gray {
	return image.NewGray(image.Rect(0, 0, hardware.SSD1306Width, hardware.SSD1306Height))
}

func paginate(total, size, idx int) (start, end int) {
	groups := (total + size - 1) / size
	start = (idx % groups) * size
	end = min(start+size, total)
	return start, end
}

func renderLines(lines []string) *image.Gray {
	img := newFrame()
	d := &font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(color.Gray{Y: 255}),
		Face: textFace,
	}
	for i, line := range lines {
		d.Dot = fixed.Point26_6{X: fixed.I(0), Y: fixed.I((i + 1) * textLineHeight)}
		d.DrawString(fitLine(textFace, line, hardware.SSD1306Width))
	}
	return img
}

func fitLine(face font.Face, s string, width int) string {
	maxWidth := fixed.I(width)
	for len(s) > 0 && font.MeasureString(face, s) > maxWidth {
		s = s[:len(s)-1]
	}
	return s
}
