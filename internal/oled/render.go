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

var (
	pixelOn  = color.Gray{Y: 255}
	pixelOff = color.Gray{Y: 0}
)

const (
	rowsPerPage    = 3
	emptyMessageY  = 18
	displayCenterX = hardware.SSD1306Width / 2
)

type pageRow struct {
	label, value string
	percent      float64
}

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

func formatUsedTotal(used, total uint64, decimals int) string {
	units := [...]string{"B", "KB", "MB", "GB", "TB"}
	u, t := float64(used), float64(total)
	unit := 0
	for t >= 1024 && unit < len(units)-1 {
		u /= 1024
		t /= 1024
		unit++
	}
	return fmt.Sprintf("%.*f/%.*f %s", decimals, u, decimals, t, units[unit])
}

func drawText(img *image.Gray, face font.Face, s string, x, y int, c color.Gray) {
	d := &font.Drawer{Dst: img, Src: image.NewUniform(c), Face: face}
	d.Dot.X = fixed.I(x)
	d.Dot.Y = fixed.I(y) + face.Metrics().Ascent
	d.DrawString(s)
}

func centeredX(face font.Face, s string, cx int) int {
	return cx - font.MeasureString(face, s).Round()/2
}

func drawBar(img *image.Gray, x, y, w, h int, percent float64) {
	x1, y1 := x+w, y+h
	fillRect(img, x, y, x1, y1, pixelOn)
	fillRect(img, x+1, y+1, x1-1, y1-1, pixelOff)
	fillRect(img, x, y, x+int(float64(w)*min(max(percent, 0), 100)/100), y1, pixelOn)
}

func fillRect(img *image.Gray, x0, y0, x1, y1 int, c color.Gray) {
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			img.SetGray(x, y, c)
		}
	}
}

func renderEmptyMessage(msg string) *image.Gray {
	img := newFrame()
	drawText(img, bigFace, msg, centeredX(bigFace, msg, displayCenterX), emptyMessageY, pixelOn)
	return img
}
