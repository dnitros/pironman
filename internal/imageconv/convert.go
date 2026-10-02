package imageconv

import (
	"fmt"
	"image"
	"image/color"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/image/draw"

	"github.com/dnitros/pironman/internal/hardware"
	"github.com/dnitros/pironman/internal/pbm"
)

var bayer4x4 = [4][4]int{
	{0, 8, 2, 10},
	{12, 4, 14, 6},
	{3, 11, 1, 9},
	{15, 7, 13, 5},
}

func Convert(src image.Image, w, h int, invert bool) *image.Gray {
	sb := src.Bounds()
	sw, sh := sb.Dx(), sb.Dy()
	if sw <= 0 || sh <= 0 {
		return image.NewGray(image.Rect(0, 0, w, h))
	}

	scale := min(float64(w)/float64(sw), float64(h)/float64(sh))
	tw := max(1, int(float64(sw)*scale))
	th := max(1, int(float64(sh)*scale))

	scaled := image.NewRGBA(image.Rect(0, 0, tw, th))
	draw.CatmullRom.Scale(scaled, scaled.Bounds(), src, sb, draw.Over, nil)

	out := image.NewGray(image.Rect(0, 0, w, h))
	ox, oy := (w-tw)/2, (h-th)/2
	for y := 0; y < th; y++ {
		for x := 0; x < tw; x++ {
			if litPixel(scaled.RGBAAt(x, y), bayer4x4[y%4][x%4], invert) {
				out.SetGray(ox+x, oy+y, color.Gray{Y: 255})
			}
		}
	}
	return out
}

func litPixel(c color.RGBA, bayerCell int, invert bool) bool {
	if c.A == 0 {
		return false
	}
	y := uint8((299*uint32(c.R) + 587*uint32(c.G) + 114*uint32(c.B)) / 1000)
	lit := y > uint8(bayerCell*16+8)
	if invert {
		lit = !lit
	}
	return lit
}

func PersistImage(srcPath, destDir, destName string, invert bool) (string, error) {
	img, err := loadAsDisplayImage(srcPath, invert)
	if err != nil {
		return "", err
	}

	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return "", fmt.Errorf("imageconv: create %s: %w", destDir, err)
	}

	destPath := filepath.Join(destDir, destName+".pbm")
	if err := pbm.EncodeFile(destPath, img); err != nil {
		return "", fmt.Errorf("imageconv: write %s: %w", destPath, err)
	}
	return destPath, nil
}

func Invert(img *image.Gray) *image.Gray {
	out := image.NewGray(img.Bounds())
	for i, v := range img.Pix {
		out.Pix[i] = 255 - v
	}
	return out
}

func loadAsDisplayImage(srcPath string, invert bool) (*image.Gray, error) {
	if strings.EqualFold(filepath.Ext(srcPath), ".pbm") {
		img, err := pbm.DecodeFile(srcPath)
		if err != nil {
			return nil, fmt.Errorf("imageconv: %w", err)
		}
		if b := img.Bounds(); b.Dx() != hardware.SSD1306Width || b.Dy() != hardware.SSD1306Height {
			return nil, fmt.Errorf("imageconv: %s is %dx%d, must be exactly %dx%d", srcPath, b.Dx(), b.Dy(), hardware.SSD1306Width, hardware.SSD1306Height)
		}
		if invert {
			img = Invert(img)
		}
		return img, nil
	}

	f, err := os.Open(srcPath)
	if err != nil {
		return nil, fmt.Errorf("imageconv: open %s: %w", srcPath, err)
	}
	defer f.Close()

	src, _, err := image.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("imageconv: decode %s: %w", srcPath, err)
	}
	return Convert(src, hardware.SSD1306Width, hardware.SSD1306Height, invert), nil
}
