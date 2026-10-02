// Package imageconv converts arbitrary images into the SSD1306's native
// 128x64 1-bit format and persists the result as a .pbm file.
package imageconv

import (
	"fmt"
	"image"
	"image/color"
	stddraw "image/draw"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/image/draw"

	"github.com/dnitros/pironman/internal/hardware"
	"github.com/dnitros/pironman/internal/pbm"
)

// bayer4x4 is a 4x4 ordered-dither threshold matrix, scaled from 0-15 to 0-255.
var bayer4x4 = [4][4]int{
	{0, 8, 2, 10},
	{12, 4, 14, 6},
	{3, 11, 1, 9},
	{15, 7, 13, 5},
}

// Convert scales src to fit within w x h preserving aspect ratio, centers it
// on a w x h canvas, and ordered-dithers it to 1-bit (every pixel is Y=0 or
// Y=255). The canvas outside the scaled image is always left unlit — invert
// flips which side of the dithering threshold counts as lit for the scaled
// image itself, without touching that outer margin, so a non-matching aspect
// ratio doesn't grow a lit border around an otherwise dark result.
func Convert(src image.Image, w, h int, invert bool) *image.Gray {
	sb := src.Bounds()
	sw, sh := sb.Dx(), sb.Dy()
	if sw <= 0 || sh <= 0 {
		return image.NewGray(image.Rect(0, 0, w, h))
	}
	src = flattenOnWhite(src)

	scale := min(float64(w)/float64(sw), float64(h)/float64(sh))
	tw := max(1, int(float64(sw)*scale))
	th := max(1, int(float64(sh)*scale))

	scaled := image.NewGray(image.Rect(0, 0, tw, th))
	draw.CatmullRom.Scale(scaled, scaled.Bounds(), src, sb, draw.Src, nil)

	out := image.NewGray(image.Rect(0, 0, w, h))
	ox, oy := (w-tw)/2, (h-th)/2
	for y := 0; y < th; y++ {
		for x := 0; x < tw; x++ {
			threshold := uint8(bayer4x4[y%4][x%4] * 17)
			lit := scaled.GrayAt(x, y).Y > threshold
			if invert {
				lit = !lit
			}
			if lit {
				out.SetGray(ox+x, oy+y, color.Gray{Y: 255})
			}
		}
	}
	return out
}

// flattenOnWhite composites src onto an opaque white background. A pixel's
// color.Color.RGBA() always reports (0,0,0,0) once its alpha is 0, no matter
// what color is actually stored there — so a transparent PNG (the common
// case for icon assets: a colored glyph on a transparent background) would
// otherwise convert to solid black, indistinguishable from a black glyph and
// producing a blank display. Flattening first restores proper contrast; it's
// a no-op for an already fully-opaque image.
func flattenOnWhite(src image.Image) image.Image {
	b := src.Bounds()
	dst := image.NewRGBA(b)
	stddraw.Draw(dst, b, image.White, image.Point{}, stddraw.Src)
	stddraw.Draw(dst, b, src, b.Min, stddraw.Over)
	return dst
}

// PersistImage converts srcPath (.png/.jpg/.pbm) to a 128x64 1-bit .pbm file
// at destDir/destName+".pbm" and returns the written path. A .pbm source
// must already be exactly 128x64; it is rejected rather than resized.
//
// destName is caller-chosen rather than derived from srcPath's basename so
// that persisting several source paths in one call can't collide on the
// same destination file just because two sources share a filename.
//
// invert flips which pixels light up — useful for a dark-glyph-on-light
// source (the common case after flattenOnWhite) when a light-glyph-on-dark
// result is wanted instead. It's applied once here and baked into the
// persisted .pbm, not reapplied at render time.
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

// Invert flips every pixel of a 1-bit image (lit becomes unlit and vice
// versa).
func Invert(img *image.Gray) *image.Gray {
	out := image.NewGray(img.Bounds())
	for i, v := range img.Pix {
		out.Pix[i] = 255 - v
	}
	return out
}

// loadAsDisplayImage decodes srcPath into a ready-to-persist 128x64 1-bit
// image. A .pbm source has no letterbox margin to preserve (it must already
// be exactly 128x64), so invert there is a plain full-image flip via Invert;
// a .png/.jpg source goes through Convert, which applies invert only to the
// scaled image itself.
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
