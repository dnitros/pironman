// Package imageconv converts arbitrary images into the SSD1306's native
// 128x64 1-bit format and persists the result as a .pbm file.
//
// Convert and isWhitePixel below are ported byte-for-byte from
// github.com/l-you/pironman5-go's internal/imageconv/convert.go (GPLv2),
// for a direct A/B comparison against this project's own conversion
// approach — only the oled.Width/oled.Height references were renamed to
// this project's hardware.SSD1306Width/SSD1306Height, since that package
// doesn't exist here. PersistImage/loadAsDisplayImage are this project's
// own glue, unchanged in shape from before this port, minus the invert
// parameter (l-you's Convert has no equivalent).
package imageconv

import (
	"fmt"
	"image"
	"image/color"
	_ "image/jpeg"
	_ "image/png"
	"math"
	"os"
	"path/filepath"
	"strings"

	xdraw "golang.org/x/image/draw"

	"github.com/dnitros/pironman/internal/hardware"
	"github.com/dnitros/pironman/internal/pbm"
)

func Convert(src image.Image, width, height int) *image.Gray {
	dst := image.NewGray(image.Rect(0, 0, width, height))
	bounds := src.Bounds()
	if bounds.Dx() < 1 || bounds.Dy() < 1 {
		return dst
	}
	scale := math.Min(float64(width)/float64(bounds.Dx()), float64(height)/float64(bounds.Dy()))
	scaledWidth := max(1, int(math.Round(float64(bounds.Dx())*scale)))
	scaledHeight := max(1, int(math.Round(float64(bounds.Dy())*scale)))
	resized := image.NewRGBA(image.Rect(0, 0, scaledWidth, scaledHeight))
	xdraw.CatmullRom.Scale(resized, resized.Bounds(), src, bounds, xdraw.Over, nil)
	x0 := (width - scaledWidth) / 2
	y0 := (height - scaledHeight) / 2
	for y := 0; y < scaledHeight; y++ {
		for x := 0; x < scaledWidth; x++ {
			if !isWhitePixel(resized.At(x, y), x, y) {
				continue
			}
			dst.SetGray(x0+x, y0+y, color.Gray{Y: 255})
		}
	}
	return dst
}

func isWhitePixel(c color.Color, x, y int) bool {
	r, g, b, a := c.RGBA()
	if a == 0 {
		return false
	}
	luma := float64(299*r+587*g+114*b) / 1000
	luma *= float64(a) / 0xffff
	bayer := [4][4]float64{
		{0, 8, 2, 10},
		{12, 4, 14, 6},
		{3, 11, 1, 9},
		{15, 7, 13, 5},
	}
	threshold := 0xffff * (0.35 + bayer[y%4][x%4]/16*0.3)
	return luma >= threshold
}

// PersistImage converts srcPath (.png/.jpg/.pbm) to a 128x64 1-bit .pbm file
// at destDir/destName+".pbm" and returns the written path. A .pbm source
// must already be exactly 128x64; it is rejected rather than resized.
//
// destName is caller-chosen rather than derived from srcPath's basename so
// that persisting several source paths in one call can't collide on the
// same destination file just because two sources share a filename.
func PersistImage(srcPath, destDir, destName string) (string, error) {
	img, err := loadAsDisplayImage(srcPath)
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

func loadAsDisplayImage(srcPath string) (*image.Gray, error) {
	if strings.EqualFold(filepath.Ext(srcPath), ".pbm") {
		img, err := pbm.DecodeFile(srcPath)
		if err != nil {
			return nil, fmt.Errorf("imageconv: %w", err)
		}
		if b := img.Bounds(); b.Dx() != hardware.SSD1306Width || b.Dy() != hardware.SSD1306Height {
			return nil, fmt.Errorf("imageconv: %s is %dx%d, must be exactly %dx%d", srcPath, b.Dx(), b.Dy(), hardware.SSD1306Width, hardware.SSD1306Height)
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
	return Convert(src, hardware.SSD1306Width, hardware.SSD1306Height), nil
}
